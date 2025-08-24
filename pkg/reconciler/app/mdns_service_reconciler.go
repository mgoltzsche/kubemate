package app

// TODO: implement mDNS advertizing reconciler to expose librespot and snapcast
// Maybe do it based on an annotation on Service resources.

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/brutella/dnssd"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	//deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
)

const (
	annotationMDNSService = "kubemate.mgoltzsche.github.com/mdns-service"
	annotationMDNSName    = "kubemate.mgoltzsche.github.com/mdns-name"
)

var csvRegex = regexp.MustCompile(", *")

// MDNSReconciler watches Service objects and announce them via MDNS.
type MDNSReconciler struct {
	DeviceName        string
	NetworkInterfaces []string
	client            client.Client
	scheme            *runtime.Scheme
	mdnsServers       map[string]mdnsServer
}

type mdnsServer struct {
	cancel context.CancelFunc
}

func (r *MDNSReconciler) AddToScheme(s *runtime.Scheme) error {
	err := corev1.AddToScheme(s)
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MDNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
	r.client = mgr.GetClient()
	r.mdnsServers = map[string]mdnsServer{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			hasMDNSServiceAnnotation,
		))).
		Complete(r)
}

func (r *MDNSReconciler) Close() error {
	// TODO: make sure this is called on shutdown
	for key := range r.mdnsServers {
		r.shutdownMDNSServer(key)
	}

	return nil
}

func hasMDNSServiceAnnotation(o client.Object) bool {
	a := o.GetAnnotations()
	return a != nil && a[annotationMDNSService] != ""
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to move the current state of the cluster closer to the desired state.
func (r *MDNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	svc := &corev1.Service{}
	key := req.String()

	err := r.client.Get(ctx, req.NamespacedName, svc)
	if err != nil {
		if errors.IsNotFound(err) { // Service was deleted
			r.shutdownMDNSServer(key)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !hasMDNSServiceAnnotation(svc) || svc.Spec.Type != corev1.ServiceTypeNodePort || len(svc.Spec.Ports) == 0 {
		r.shutdownMDNSServer(key)

		return ctrl.Result{}, err
	}

	logger.V(1).Info("reconcile MDNS service")

	_, announced := r.mdnsServers[key]
	if announced {
		// TODO: re-announce when service changed
		return ctrl.Result{}, nil
	}

	svcs, err := r.toMDNSEntries(ctx, svc)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Advertize MDNS service

	err = r.advertizeService(key, svcs)

	return ctrl.Result{}, err
}

func (r *MDNSReconciler) advertizeService(key string, svcs []dnssd.Config) error {
	// TODO: use single responder for all services
	rp, err := dnssd.NewResponder()
	if err != nil {
		return fmt.Errorf("new mdns responder: %w", err)
	}

	for _, svc := range svcs {
		sv, err := dnssd.NewService(svc)
		if err != nil {
			return fmt.Errorf("new mdns service: %w", err)
		}

		hdl, err := rp.Add(sv)
		if err != nil {
			return fmt.Errorf("add mdns service to responder: %w", err)
		}

		hdl.UpdateText(map[string]string{
			"CPath":   "/",
			"VERSION": "1.0",
		}, rp)
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger := logrus.WithField("resource", key)
	wg := &sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()

		err := rp.Respond(ctx)
		if err != nil {
			if err == context.Canceled {
				logger.Infof("unpublished mdns service(s) %s", serviceNames(svcs))

				return
			}

			logrus.WithError(err).Errorf("failed to advertize mdns service(s) %s", serviceNames(svcs))
		}

		// TODO: retry in case it terminates - fetching the current IP again
	}()

	r.mdnsServers[key] = mdnsServer{
		cancel: func() {
			cancel()
			wg.Wait()
		},
	}

	return nil
}

func (r *MDNSReconciler) shutdownMDNSServer(key string) {
	srv, ok := r.mdnsServers[key]
	if !ok {
		return
	}

	srv.cancel()

	delete(r.mdnsServers, key)
}

func (r *MDNSReconciler) toMDNSEntries(ctx context.Context, svc *corev1.Service) ([]dnssd.Config, error) {
	logger := log.FromContext(ctx)

	a := svc.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}

	instanceName := fmt.Sprintf("%s-%s", r.DeviceName, svc.Name)
	if name := a[annotationMDNSName]; name != "" {
		instanceName = fmt.Sprintf("%s-%s", r.DeviceName, name)
	}

	port := int(svc.Spec.Ports[0].NodePort)

	ips, err := ipsForNetworkInterfaces(r.NetworkInterfaces)
	if err != nil {
		return nil, fmt.Errorf("failed to get IPs to advertize for mdns service: %w", err)
	}

	serviceTypes := csvRegex.Split(a[annotationMDNSService], -1)
	mdnsServices := make([]dnssd.Config, 0, len(serviceTypes))

	for _, serviceType := range serviceTypes {
		if serviceType != "" {
			logger.Info(fmt.Sprintf("Advertizing MDNS service %s.%s on %s %+v", instanceName, serviceType, r.DeviceName, r.NetworkInterfaces))

			mdnsServices = append(mdnsServices, dnssd.Config{
				Name:   instanceName,
				Type:   serviceType,
				Port:   port,
				Ifaces: r.NetworkInterfaces,
				IPs:    ips,
			})
		}
	}

	return mdnsServices, nil
}

func serviceNames(svcs []dnssd.Config) string {
	r := make([]string, len(svcs))

	for i, svc := range svcs {
		r[i] = fmt.Sprintf("%s.%s", svc.Name, svc.Type)
	}

	return strings.Join(r, ", ")
}

func ipsForNetworkInterfaces(ifaceNames []string) ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ifaceIPMap := make(map[string]net.IP, len(ifaces))
	for _, iface := range ifaces {
		addrs, e := iface.Addrs()
		if e != nil {
			if err == nil {
				err = e
			}
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipnet.IP.To4()
			if v4 == nil || v4.IsLoopback() || v4.IsUnspecified() || v4.IsMulticast() || v4.IsLinkLocalMulticast() || v4.IsInterfaceLocalMulticast() || v4.IsLinkLocalUnicast() {
				continue
			}
			brd := toBroadcastIP(ipnet)
			if brd.String() == v4.String() {
				continue
			}
			ifaceIPMap[iface.Name] = v4
		}
	}

	ips := make([]net.IP, 0, len(ifaceNames))
	if len(ifaceNames) > 0 {
		for _, ifaceName := range ifaceNames {
			ip, ok := ifaceIPMap[ifaceName]
			if ok {
				ips = append(ips, ip)
			}
		}
	} else {
		for _, iface := range ifaces {
			if ip, ok := ifaceIPMap[iface.Name]; ok {
				ips = append(ips, ip)
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP available")
	}

	return ips, nil
}

func toBroadcastIP(ip *net.IPNet) net.IP {
	brd := make(net.IP, len(ip.IP.To4()))
	binary.BigEndian.PutUint32(brd, binary.BigEndian.Uint32(ip.IP.To4())|^binary.BigEndian.Uint32(net.IP(ip.Mask).To4()))
	return brd
}
