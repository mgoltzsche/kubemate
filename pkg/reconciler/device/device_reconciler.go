package device

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/clientconf"
	"github.com/mgoltzsche/kubemate/pkg/controller"
	"github.com/mgoltzsche/kubemate/pkg/discovery"
	"github.com/mgoltzsche/kubemate/pkg/ingress"
	"github.com/mgoltzsche/kubemate/pkg/reconciler/app"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DeviceReconciler reconciles a Device object.
type DeviceReconciler struct {
	DeviceName            string
	DeviceAddress         string
	DataDir               string
	ManifestDir           string
	ExternalPort          int
	K3sProxyEnabled       *bool
	Docker                bool
	KubeletArgs           []string
	Devices               storage.Interface
	DeviceTokens          storage.Interface
	NetworkInterfaces     storage.Interface
	NetworkInterfaceNames []string
	DeviceDiscovery       *discovery.DeviceDiscovery
	IngressController     *ingress.IngressController
	Shutdown              func() error
	Logger                *logrus.Entry
	client.Client
	scheme         *runtime.Scheme
	k3s            *runner.Runner
	controllers    *controller.ControllerManager
	nodeController *controller.ControllerManager
	dnsServer      *deviceDnsServerReconciler
}

func (r *DeviceReconciler) AddToScheme(s *runtime.Scheme) error {
	err := deviceapi.AddToScheme(s)
	if err != nil {
		return err
	}
	return nil
}

func (r *DeviceReconciler) nodeClientConfig() (*rest.Config, error) {
	return clientconf.New(r.DataDir, deviceapi.DeviceModeAgent)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	nodeReconciler := &NodeReconciler{
		DeviceName:  r.DeviceName,
		DeviceStore: r.Devices,
		K3sDir:      r.DataDir,
		Shutdown:    r.Shutdown,
	}
	dnsDir := filepath.Join(r.DataDir, "dns")
	r.dnsServer = newDeviceDnsServerReconciler(dnsDir, r.DeviceName, r.Devices, r.NetworkInterfaces, r.Logger)
	// TODO: use mgr.GetLogger() logr.Logger that controller-runtime is providing to the Reconcile method as well
	r.controllers = controller.NewControllerManager(ctrl.GetConfig, logrus.WithField("comp", "controller-manager"))
	r.controllers.RegisterReconciler(nodeReconciler)
	r.controllers.RegisterReconciler(&app.AppReconciler{})
	r.controllers.RegisterReconciler(&app.MDNSReconciler{
		DeviceName:        r.DeviceName,
		NetworkInterfaces: r.NetworkInterfaceNames,
	})
	r.nodeController = controller.NewControllerManager(r.nodeClientConfig, logrus.WithField("comp", "node-controller-manager"))
	r.nodeController.RegisterReconciler(nodeReconciler)
	r.k3s = runner.New(r.Logger.WithField("proc", "k3s"))
	r.k3s.TerminationSignal = syscall.SIGQUIT
	r.k3s.Reporter = func(cmd runner.Command) {
		// Update device resource's status
		if cmd.Status.State == runner.ProcessStateFailed {
			r.Logger.WithField("pid", cmd.Status.Pid).Warnf("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
		} else {
			r.Logger.WithField("pid", cmd.Status.Pid).Infof("k3s %s", cmd.Status.State)
		}
		d := &deviceapi.Device{}
		err := r.Devices.Update(r.DeviceName, d, func() error {
			d.Status.Generation = d.Generation
			if d.Status.State != deviceapi.DeviceStateTerminating {
				// TODO: map properly
				d.Status.State = deviceapi.DeviceState(cmd.Status.State)
			}
			d.Status.Message = cmd.Status.Message
			d.Status.Address = fmt.Sprintf("https://%s", r.DeviceAddress)
			d.Status.Current = true
			return nil
		})
		if err != nil {
			r.Logger.WithError(err).Error("failed to update device status")
		}
	}
	// Add CRDs to k3s' manifest directory
	err := copyManifests(r.ManifestDir, filepath.Join(r.DataDir, "server", "manifests"))
	if err != nil {
		return fmt.Errorf("copy default manifests into data dir: %w", err)
	}

	r.scheme = mgr.GetScheme()
	r.Client = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		For(&deviceapi.Device{}).
		Watches(&deviceapi.NetworkInterface{}, handler.EnqueueRequestsFromMapFunc(r.deviceReconcileRequest)).
		Watches(&deviceapi.DeviceToken{}, handler.EnqueueRequestsFromMapFunc(r.deviceReconcileRequest)).
		Watches(&deviceapi.WifiPassword{}, handler.EnqueueRequestsFromMapFunc(r.deviceReconcileRequest)).
		Complete(r)
}

func (r *DeviceReconciler) deviceReconcileRequest(_ context.Context, o client.Object) []ctrl.Request {
	return []ctrl.Request{{NamespacedName: types.NamespacedName{Name: r.DeviceName}}}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to move the current state of the cluster closer to the desired state.
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	logger := log.FromContext(ctx)
	// Fetch Device
	d := deviceapi.Device{}
	err = r.Client.Get(ctx, req.NamespacedName, &d)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return requeue(err)
	}
	logger.V(1).Info("reconcile device")

	if m := d.Spec.Mode; m != deviceapi.DeviceModeServer && m != deviceapi.DeviceModeAgent {
		e := fmt.Errorf("unsupported device mode %q specified", d.Spec.Mode)
		logger.Error(e, "unsupported device mode specified")
		return ctrl.Result{}, nil
	}

	nodeIP, err := r.ipAddress()
	if err != nil {
		logger.Error(err, "no ip address available")
		return ctrl.Result{}, nil
	}
	// TODO: apply network configuration here instead of managing each networkinterface within a separate resource?!
	err = r.dnsServer.Reconcile(ctx, &d)
	if err != nil {
		return requeue(err)
	}
	var args []string
	fn := func() error {
		*r.K3sProxyEnabled = d.Spec.Mode == deviceapi.DeviceModeServer
		switch d.Spec.Mode {
		case deviceapi.DeviceModeServer:
			args = buildK3sServerArgs(&d, nodeIP, r.DataDir, r.Docker, r.KubeletArgs, r.DeviceTokens)
		case deviceapi.DeviceModeAgent:
			if d.Spec.ServerAddress == "" {
				return fmt.Errorf("no server specified to join")
			}
			if d.Spec.ServerAddress == d.Status.Address {
				return fmt.Errorf("cannot join itself")
			}
			if d.Spec.JoinTokenName == "" {
				return fmt.Errorf("cannot join server since no join token name specified")
			}
			joinAddr, err := joinAddress(&d)
			if err != nil {
				return fmt.Errorf("join cluster: %w", err)
			}
			// TODO: provide token as env var
			args = buildK3sAgentArgs(joinAddr, d.Spec.JoinTokenName, nodeIP, r.DataDir, r.Docker, r.KubeletArgs, r.DeviceTokens)
		}
		return nil
	}
	var statusMessage string
	if err = fn(); err != nil {
		logger.Error(err, "failed to reconcile device")
		statusMessage = err.Error()
		defer func() {
			res = ctrl.Result{RequeueAfter: 10 * time.Second}
		}()
	}
	addr := fmt.Sprintf("https://%s", r.DeviceName)
	if r.ExternalPort != 443 {
		addr = fmt.Sprintf("%s:%d", addr, r.ExternalPort)
	}
	if d.Status.Message != statusMessage || d.Status.Address != addr {
		// Update device status
		err = r.Devices.Update(d.Name, &d, func() error {
			d.Status.Message = statusMessage
			d.Status.Address = addr
			d.Status.Current = true
			return nil
		})
		if err != nil {
			return requeue(err)
		}
	}
	if d.Generation == d.Status.Generation {
		// TODO: advertize only when status changed
		err = r.DeviceDiscovery.Advertise(&deviceapi.DeviceDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name: d.Name,
			},
			Spec: deviceapi.DeviceDiscoverySpec{
				Address: d.Status.Address,
				Mode:    d.Spec.Mode,
				Server:  d.Spec.ServerAddress,
				Current: true,
			},
		}, nodeIP)
		if err != nil {
			return requeue(err)
		}
	}
	if len(args) > 0 {
		if d.Status.State == deviceapi.DeviceStateTerminating {
			r.k3s.Stop()
		} else {
			r.k3s.Start(runner.Cmd("/proc/self/exe", args...))
			if d.Spec.Mode == deviceapi.DeviceModeServer {
				err := r.reconcileServerToken()
				if err != nil {
					logger.Error(err, "reconcile server")
					return ctrl.Result{RequeueAfter: time.Second}, nil
				}
			}
		}
		if d.Spec.Mode == deviceapi.DeviceModeServer && d.Status.State != deviceapi.DeviceStateTerminating {
			r.nodeController.Stop()
			r.controllers.Start()
			r.IngressController.Start()
		} else {
			r.controllers.Stop()
			r.IngressController.Stop()
			if d.Status.State == deviceapi.DeviceStateTerminating {
				r.nodeController.Stop()
			} else {
				r.nodeController.Start()
			}
		}
	}
	logger.V(1).Info("device reconciliation complete")
	return ctrl.Result{}, nil
}

func (r *DeviceReconciler) ipAddress() (net.IP, error) {
	l := &deviceapi.NetworkInterfaceList{}
	err := r.NetworkInterfaces.List(l)
	if err != nil {
		return nil, fmt.Errorf("detect ip address: %w", err)
	}
	idx := 9999999
	ip := ""
	for _, iface := range l.Items {
		if iface.Status.Link.Up && iface.Status.Link.IP4 != "" {
			if i := iface.Status.Link.Index; i < idx {
				idx = i
				ip = iface.Status.Link.IP4
			}
		}
	}
	if ip == "" {
		return nil, fmt.Errorf("all network links appear to be down")
	}
	return net.ParseIP(ip), nil
}

func (r *DeviceReconciler) reconcileServerToken() error {
	t := &deviceapi.DeviceToken{}
	err := r.DeviceTokens.Get(r.DeviceName, t)
	if err != nil {
		return err
	}
	if t.Status.JoinToken != "" {
		return nil // already set
	}
	return r.DeviceTokens.Update(r.DeviceName, t, func() error {
		serverTokenFile := filepath.Join(r.DataDir, "server", "token")
		b, err := os.ReadFile(serverTokenFile)
		if err != nil {
			return err
		}
		t.Status.JoinToken = strings.TrimSuffix(string(b), "\n")
		return nil
	})
}

func requeue(err error) (r ctrl.Result, e error) {
	r.RequeueAfter = time.Second
	var cooldown *runner.CooldownError
	if errors.As(err, &cooldown) {
		r.RequeueAfter = cooldown.Duration + time.Millisecond
		err = nil
	}
	return r, err
}

func joinAddress(d *deviceapi.Device) (string, error) {
	u, err := url.Parse(d.Spec.ServerAddress)
	if err != nil {
		return "", fmt.Errorf("invalid server address %q specified: %w", d.Spec.ServerAddress, err)
	}
	return fmt.Sprintf("https://%s:6443", u.Hostname()), nil
}

func buildK3sServerArgs(d *deviceapi.Device, nodeIP net.IP, dataDir string, docker bool, kubeletArgs []string, clusterTokens storage.Interface) []string {
	args := []string{
		"server",
		// TODO: specify path to k3s config here and configure everything there
		fmt.Sprintf("--node-external-ip=%s", nodeIP.String()),
		"--disable-cloud-controller",
		"--disable-helm-controller",
		"--disable=servicelb,traefik",
		fmt.Sprintf("--kube-apiserver-arg=--token-auth-file=%s", "/etc/kubemate/tokens"),
		fmt.Sprintf("--data-dir=%s", dataDir),
	}
	token := &deviceapi.DeviceToken{}
	err := clusterTokens.Get(d.Name, token)
	if err != nil {
		logrus.Error(err)
	} else {
		args = append(args, fmt.Sprintf("--token=%s", token.Data.Token))
	}
	if docker {
		args = append(args, "--docker")
	}
	for _, a := range kubeletArgs {
		args = append(args, fmt.Sprintf("--kubelet-arg=%s", a))
	}
	return args
}

func buildK3sAgentArgs(joinAddress, tokenName string, nodeIP net.IP, dataDir string, docker bool, kubeletArgs []string, clusterTokens storage.Interface) []string {
	args := []string{
		"agent",
		fmt.Sprintf("--node-external-ip=%s", nodeIP.String()),
		fmt.Sprintf("--data-dir=%s", dataDir),
	}
	token := &deviceapi.DeviceToken{}
	err := clusterTokens.Get(tokenName, token)
	if err != nil {
		logrus.Error(fmt.Errorf("join server %s: %w", joinAddress, err))
		return nil
	}
	args = append(args,
		fmt.Sprintf("--server=%s", joinAddress),
		fmt.Sprintf("--token=%s", token.Data.Token),
	)
	if docker {
		args = append(args, "--docker")
	}
	for _, a := range kubeletArgs {
		args = append(args, fmt.Sprintf("--kubelet-arg=%s", a))
	}
	return args
}
