package device

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/controller"
	"github.com/mgoltzsche/kubemate/pkg/discovery"
	"github.com/mgoltzsche/kubemate/pkg/ingress"
	"github.com/mgoltzsche/kubemate/pkg/reconciler/app"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/utils"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// DeviceReconciler reconciles a Device object.
type DeviceReconciler struct {
	DeviceName        string
	DeviceAddress     string
	DataDir           string
	ManifestDir       string
	ExternalPort      int
	K3sProxyEnabled   *bool
	Docker            bool
	KubeletArgs       []string
	Devices           storage.Interface
	DeviceTokens      storage.Interface
	WifiPasswords     storage.Interface
	Wifi              *wifi.Wifi
	DeviceDiscovery   *discovery.DeviceDiscovery
	IngressController *ingress.IngressController
	Logger            *logrus.Entry
	client.Client
	scheme      *runtime.Scheme
	k3s         *runner.Runner
	controllers *controller.ControllerManager
}

func (r *DeviceReconciler) AddToScheme(s *runtime.Scheme) error {
	err := deviceapi.AddToScheme(s)
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: use mgr.GetLogger() logr.Logger that controller-runtime is providing to the Reconcile method as well
	r.controllers = controller.NewControllerManager(ctrl.GetConfig, logrus.WithField("comp", "controller-manager"))
	r.controllers.RegisterReconciler(&app.AppReconciler{})
	r.k3s = runner.New(r.Logger.WithField("proc", "k3s"))
	r.k3s.Reporter = func(cmd runner.Command) {
		// Update device resource's status
		if cmd.Status.State == runner.ProcessStateFailed {
			logrus.Warnf("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
		} else {
			logrus.Infof("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
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
			logrus.WithError(err).Error("failed to update device status")
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
		Watches(&source.Kind{Type: &deviceapi.NetworkInterface{}}, handler.EnqueueRequestsFromMapFunc(r.deviceReconcileRequest)).
		Watches(&source.Kind{Type: &deviceapi.DeviceToken{}}, handler.EnqueueRequestsFromMapFunc(r.deviceReconcileRequest)).
		Watches(&source.Kind{Type: &deviceapi.WifiPassword{}}, handler.EnqueueRequestsFromMapFunc(r.deviceReconcileRequest)).
		Complete(r)
}

func (r *DeviceReconciler) deviceReconcileRequest(o client.Object) []ctrl.Request {
	return []ctrl.Request{{NamespacedName: types.NamespacedName{Name: r.DeviceName}}}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to move the current state of the cluster closer to the desired state.
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	logger := log.FromContext(ctx)
	// Fetch Device
	d := deviceapi.Device{}
	err = r.Client.Get(ctx, req.NamespacedName, &d)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return requeue(err)
	}
	logger.V(1).Info("reconcile device")

	// Reconcile wifi
	switch d.Spec.Wifi.Mode {
	case deviceapi.WifiModeAccessPoint:
		err = setWifiCountry(&d, r.Devices, r.Wifi, r.Logger)
		if err != nil {
			return requeue(err)
		}
		wifiPassword := deviceapi.WifiPassword{}
		err = r.WifiPasswords.Get(deviceapi.AccessPointPasswordKey, &wifiPassword)
		if err != nil {
			return requeue(err)
		}
		err = r.Wifi.StartAccessPoint(r.DeviceName, wifiPassword.Data.Password)
		if err != nil {
			return requeue(err)
		}
	case deviceapi.WifiModeStation:
		r.Wifi.StopAccessPoint()
		err = setWifiCountry(&d, r.Devices, r.Wifi, r.Logger)
		if err != nil {
			return requeue(err)
		}
		err = r.Wifi.StartWifiInterface()
		if err != nil {
			return requeue(err)
		}
		var pw deviceapi.WifiPassword
		ssid := d.Spec.Wifi.Station.SSID
		if ssid == "" {
			e := fmt.Errorf("missing ssid")
			logger.Error(e, "no ssid configured to connect to")
		} else {
			err = r.WifiPasswords.Get(ssidToResourceName(ssid), &pw)
			if err != nil {
				logger.Error(err, "no password configured for wifi network", "ssid", ssid)
			}
		}
		err = r.Wifi.StartStation(ssid, pw.Data.Password)
		if err != nil {
			return requeue(err)
		}
	default:
		r.Wifi.StopStation()
		r.Wifi.StopAccessPoint()
		err = r.Wifi.StopWifiInterface()
		if err != nil {
			return requeue(err)
		}
	}

	// Reconcile k3s
	if m := d.Spec.Mode; m != deviceapi.DeviceModeServer && m != deviceapi.DeviceModeAgent {
		e := fmt.Errorf("unsupported device mode %q specified", d.Spec.Mode)
		logger.Error(e, "unsupported device mode specified")
		return ctrl.Result{}, nil
	}

	ips, err := r.DeviceDiscovery.ExternalIPs()
	if err != nil {
		return requeue(err)
	}
	nodeIP := ips[0]
	var args []string
	fn := func() error {
		*r.K3sProxyEnabled = d.Spec.Mode == deviceapi.DeviceModeServer
		switch d.Spec.Mode {
		case deviceapi.DeviceModeServer:
			args = buildK3sServerArgs(&d, nodeIP, r.DataDir, r.Docker, r.KubeletArgs, r.DeviceTokens)
		case deviceapi.DeviceModeAgent:
			if d.Spec.Server == "" {
				return fmt.Errorf("no server specified to join")
			}
			if d.Spec.Server == d.Name {
				return fmt.Errorf("cannot join itself")
			}
			var server deviceapi.DeviceDiscovery
			err := r.DeviceDiscovery.Store().Get(d.Spec.Server, &server)
			if err != nil {
				return fmt.Errorf("join cluster: %w", err)
			}
			if server.Spec.Mode != deviceapi.DeviceModeServer {
				return fmt.Errorf("cannot join device %q since it doesn't run in %s mode but in mode %q", d.Spec.Server, deviceapi.DeviceModeServer, d.Spec.Mode)
			}
			joinAddr, err := joinAddress(&server)
			if err != nil {
				return fmt.Errorf("join cluster: %w", err)
			}
			// TODO: provide token as env var
			args = buildK3sAgentArgs(&server, joinAddr, nodeIP, r.DataDir, r.Docker, r.KubeletArgs, r.DeviceTokens)
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
		// TODO: update discovery resource
		err = r.DeviceDiscovery.Advertise(&deviceapi.DeviceDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name: d.Name,
			},
			Spec: deviceapi.DeviceDiscoverySpec{
				Address: d.Status.Address,
				Mode:    d.Spec.Mode,
				Server:  d.Spec.Server,
				Current: true,
			},
		}, ips)
		if err != nil {
			return requeue(err)
		}
	}
	if len(args) > 0 {
		if d.Status.State == deviceapi.DeviceStateTerminating {
			r.k3s.Stop()
		} else {
			r.k3s.Start(runner.Cmd("/proc/self/exe", args...))
			err := r.reconcileServerToken()
			if err != nil {
				logger.Error(err, "reconcile server")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
		}
		if d.Spec.Mode == deviceapi.DeviceModeServer && d.Status.State != deviceapi.DeviceStateTerminating {
			r.controllers.Start()
			r.IngressController.Start()
		} else {
			r.controllers.Stop()
			r.IngressController.Stop()
		}
	}

	return ctrl.Result{}, nil
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
		b, err := os.ReadFile(filepath.Join(r.DataDir, "server", "token"))
		if err != nil {
			return err
		}
		t.Status.JoinToken = strings.TrimSuffix(string(b), "\n")
		return nil
	})
}

func requeue(err error) (r ctrl.Result, e error) {
	r.RequeueAfter = time.Second
	return r, err
}

func ssidToResourceName(ssid string) string {
	ssid = fmt.Sprintf("ssid-%s", ssid)
	return utils.TruncateName(ssid, utils.MaxResourceNameLength)
}

// setWifiCountry detects the wifi country and stores it with the Device resource.
func setWifiCountry(d *deviceapi.Device, devices storage.Interface, w *wifi.Wifi, logger *logrus.Entry) error {
	w.CountryCode = d.Spec.Wifi.CountryCode
	if w.CountryCode == "" {
		err := w.StartWifiInterface()
		if err != nil {
			return err
		}
		err = w.DetectCountry()
		if err != nil {
			return err
		}
		err = devices.Update(d.Name, d, func() error {
			d.Spec.Wifi.CountryCode = w.CountryCode
			return nil
		})
		if err != nil {
			return err
		}
		logger.Infof("detected wifi country %s", w.CountryCode)
	}
	return nil
}

func joinAddress(d *deviceapi.DeviceDiscovery) (string, error) {
	a := ""
	if d.Spec.Mode == deviceapi.DeviceModeServer {
		u, err := url.Parse(d.Spec.Address)
		if err != nil {
			return "", fmt.Errorf("status.address %q of device %q is not a valid address", d.Spec.Address, d.Name)
		}
		a = fmt.Sprintf("https://%s:6443", u.Hostname())
	}
	return a, nil
}

func buildK3sServerArgs(d *deviceapi.Device, nodeIP net.IP, dataDir string, docker bool, kubeletArgs []string, clusterTokens storage.Interface) []string {
	args := []string{
		"server",
		fmt.Sprintf("--node-external-ip=%s", nodeIP.String()),
		"--disable-cloud-controller",
		"--disable-helm-controller",
		"--disable=servicelb,traefik,metrics-server",
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

func buildK3sAgentArgs(server *deviceapi.DeviceDiscovery, joinAddress string, nodeIP net.IP, dataDir string, docker bool, kubeletArgs []string, clusterTokens storage.Interface) []string {
	args := []string{
		"agent",
		fmt.Sprintf("--node-external-ip=%s", nodeIP.String()),
		fmt.Sprintf("--data-dir=%s", dataDir),
	}
	token := &deviceapi.DeviceToken{}
	err := clusterTokens.Get(server.Name, token)
	if err != nil {
		logrus.Error(fmt.Errorf("join server %s: %w", server.Name, err))
		return nil
	}
	args = append(args,
		fmt.Sprintf("--server=%s", joinAddress),
		fmt.Sprintf("--token=%s", token.Data.Token),
		"--with-node-id",
	)
	if docker {
		args = append(args, "--docker")
	}
	for _, a := range kubeletArgs {
		args = append(args, fmt.Sprintf("--kubelet-arg=%s", a))
	}
	return args
}
