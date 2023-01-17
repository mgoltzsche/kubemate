package device

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/networkifaces"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/utils"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NetworkInterfaceReconciler reconciles a Device object.
type NetworkInterfaceReconciler struct {
	DeviceName        string
	NetworkInterfaces []string
	Store             storage.Interface
	WifiPasswords     storage.Interface
	WifiNetworks      storage.Interface
	Wifi              *wifi.Wifi
	client.Client
	scheme   *runtime.Scheme
	linkSync *networkifaces.NetworkIfaceSync
}

func (r *NetworkInterfaceReconciler) AddToScheme(s *runtime.Scheme) error {
	err := deviceapi.AddToScheme(s)
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkInterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
	r.Client = mgr.GetClient()
	r.linkSync = &networkifaces.NetworkIfaceSync{
		Interfaces:    r.NetworkInterfaces,
		DefaultAPSSID: r.DeviceName,
		Store:         r.Store,
	}
	err := r.linkSync.Start()
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&deviceapi.NetworkInterface{}).
		// TODO: Watches(&source.Kind{Type: &deviceapi.WifiPassword{}}, handler.EnqueueRequestsFromMapFunc(r.deviceReconcileRequest)).
		Complete(r)
}

func (r *NetworkInterfaceReconciler) Close() error {
	return r.linkSync.Stop()
}

func (r *NetworkInterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	logger := log.FromContext(ctx)
	// Fetch Device
	iface := deviceapi.NetworkInterface{}
	err = r.Client.Get(ctx, req.NamespacedName, &iface)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return requeue(err)
	}
	logger.V(1).Info("reconcile network interface")

	switch iface.Status.Link.Type {
	case deviceapi.NetworkInterfaceTypeWifi:
		err = r.reconcileWifiNetworkInterface(&iface, logger)
	}
	if err == nil {
		err = r.ensureIPAddress(&iface)
	}
	var errMsg string
	if err != nil {
		errMsg = err.Error()
		if linkMsg := iface.Status.Link.Error; linkMsg != "" {
			errMsg = fmt.Sprintf("%s. %s", linkMsg, err)
		}
	}
	if iface.Status.Error != errMsg {
		e := r.Store.Update(iface.Name, &iface, func() error {
			iface.Status.Error = errMsg
			return nil
		})
		if e != nil {
			return requeue(e)
		}
	}
	if err != nil {
		return requeue(err)
	}

	logger.V(1).Info("network interface reconciliation complete")

	return ctrl.Result{}, nil
}

// ensureIPAddress copies the IP address from the interface into the resource status or returns an error.
// This is because the IP may be set after the link up status event was received in which case a reconciliation can be scheduled here.
func (r *NetworkInterfaceReconciler) ensureIPAddress(iface *deviceapi.NetworkInterface) error {
	if iface.Status.Link.Up && iface.Status.Link.IP4 == "" {
		ip, err := networkifaces.IPv4Address(iface.Name)
		if err != nil || ip == nil {
			if err == nil {
				err = fmt.Errorf("no ip address assigned to network interface")
			}
			return err
		}
		err = r.Store.Update(iface.Name, iface, func() error {
			iface.Status.Link.IP4 = ip.String()
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *NetworkInterfaceReconciler) reconcileWifiNetworkInterface(iface *deviceapi.NetworkInterface, logger logr.Logger) error {
	switch iface.Spec.Wifi.Mode {
	case deviceapi.WifiModeAccessPoint:
		err := r.Wifi.StartWifiInterface()
		if err != nil {
			return err
		}
		err = updateWifiNetworkList(r.Wifi, r.WifiNetworks, logger)
		if err != nil {
			return err
		}
		err = setWifiIfaceCountry(iface, r.Store, r.Wifi, logger)
		if err != nil {
			return err
		}
		wifiPassword := deviceapi.WifiPassword{}
		err = r.WifiPasswords.Get(deviceapi.AccessPointPasswordKey, &wifiPassword)
		if err != nil {
			return err
		}
		err = r.Wifi.StartAccessPoint(r.DeviceName, wifiPassword.Data.Password)
		if err != nil {
			return err
		}
	case deviceapi.WifiModeStation:
		r.Wifi.StopAccessPoint()
		err := r.Wifi.StartWifiInterface()
		if err != nil {
			return err
		}
		err = updateWifiNetworkList(r.Wifi, r.WifiNetworks, logger)
		if err != nil {
			return err
		}
		err = setWifiIfaceCountry(iface, r.Store, r.Wifi, logger)
		if err != nil {
			return err
		}
		var pw deviceapi.WifiPassword
		ssid := iface.Spec.Wifi.Station.SSID
		if ssid == "" {
			return fmt.Errorf("no wifi network configured to connect to")
		}
		err = r.WifiPasswords.Get(ssidToResourceName(ssid), &pw)
		if err != nil {
			return fmt.Errorf("no password configured for wifi network %q", ssid)
		}
		err = r.Wifi.StartStation(ssid, pw.Data.Password)
		if err != nil {
			return err
		}
	default:
		r.Wifi.StopStation()
		r.Wifi.StopAccessPoint()
		err := r.Wifi.StopWifiInterface()
		if err != nil {
			return err
		}
	}
	return nil
}

func ssidToResourceName(ssid string) string {
	ssid = fmt.Sprintf("ssid-%s", ssid)
	return utils.TruncateName(ssid, utils.MaxResourceNameLength)
}

// setWifiIfaceCountry detects the wifi country and stores it with the provided NetworkInterface resource.
func setWifiIfaceCountry(iface *deviceapi.NetworkInterface, ifaces storage.Interface, w *wifi.Wifi, logger logr.Logger) error {
	w.CountryCode = iface.Spec.Wifi.CountryCode
	if w.CountryCode == "" {
		err := w.DetectCountry()
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("detected wifi country %s", w.CountryCode))
		return ifaces.Update(iface.Name, iface, func() error {
			iface.Spec.Wifi.CountryCode = w.CountryCode
			return nil
		})
	}
	return nil
}

func updateWifiNetworkList(w *wifi.Wifi, wifiNetworks storage.Interface, logger logr.Logger) error {
	foundNetworks := map[string]struct{}{}
	for _, network := range w.Networks() {
		n := &deviceapi.WifiNetwork{}
		n.Name = fmt.Sprintf("ssid-%s", network.SSID)
		n.Name = utils.TruncateName(n.Name, utils.MaxResourceNameLength)
		n.Data.SSID = network.SSID
		foundNetworks[n.Name] = struct{}{}
		err := wifiNetworks.Create(n.Name, n)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			return err
		}
		logger.Info("discovered new wifi network", "ssid", n.Data.SSID)
	}

	// Remove old networks
	l := &deviceapi.WifiNetworkList{}
	err := wifiNetworks.List(l)
	if err != nil {
		return err
	}
	for _, n := range l.Items {
		if _, found := foundNetworks[n.Name]; !found {
			logger.Info("wifi network disappeared", "ssid", n.Data.SSID)
			if e := wifiNetworks.Delete(n.Name, &n, func() error { return nil }); e != nil && err == nil && !errors.IsNotFound(err) {
				err = e
			}
		}
	}
	return err
}
