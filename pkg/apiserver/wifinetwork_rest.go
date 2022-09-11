package apiserver

import (
	"context"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/utils"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

type wifiNetworkREST struct {
	*REST
	wifi *wifi.Wifi
}

func NewWifiNetworkREST(wifi *wifi.Wifi) *wifiNetworkREST {
	return &wifiNetworkREST{
		REST: NewREST(&deviceapi.WifiNetwork{}, storage.InMemory()),
		wifi: wifi,
	}
}

func (r *wifiNetworkREST) Delete(ctx context.Context, key string, deleteValidation registryrest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("cannot delete wifi network scan result")
}

func (r *wifiNetworkREST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("cannot create wifi network scan result")
}

func (r *wifiNetworkREST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("cannot update wifi network scan result")
}

func (r *wifiNetworkREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	err := updateWifiNetworkList(r.wifi, r.Store)
	if err != nil {
		err = fmt.Errorf("scan for wifi networks: %w", err)
		logrus.Error(err)
		return nil, errors.NewInternalError(err)
	}
	return r.REST.List(ctx, options)
}

func updateWifiNetworkList(wifi *wifi.Wifi, wifiNetworks storage.Interface) error {
	foundNetworks := map[string]struct{}{}
	networks, err := wifi.Scan()
	if err != nil {
		return err
	}
	for _, network := range networks {
		n := &deviceapi.WifiNetwork{}
		n.Data.SSID = network.SSID
		n.Name = utils.TruncateName(n.Data.SSID, utils.MaxResourceNameLength)
		foundNetworks[n.Name] = struct{}{}
		err := wifiNetworks.Create(n.Name, n)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			return err
		}
		logrus.WithField("ssid", n.Data.SSID).Info("discovered new wifi network")
	}

	// Remove old networks
	l := &deviceapi.WifiNetworkList{}
	err = wifiNetworks.List(l)
	if err != nil {
		return err
	}
	for _, n := range l.Items {
		if _, found := foundNetworks[n.Name]; !found {
			logrus.WithField("ssid", n.Data.SSID).Info("wifi network disappeared")
			if e := wifiNetworks.Delete(n.Name, &n, func() error { return nil }); e != nil && err == nil && !errors.IsNotFound(err) {
				err = e
			}
		}
	}
	return err
}
