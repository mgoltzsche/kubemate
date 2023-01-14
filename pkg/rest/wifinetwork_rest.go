package rest

import (
	"context"
	"fmt"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/utils"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

type wifiNetworkREST struct {
	*REST
}

func NewWifiNetworkREST(wifi *wifi.Wifi, scheme *runtime.Scheme) *wifiNetworkREST {
	store := storage.RefreshPeriodically(storage.InMemory(scheme), 10*time.Second, func(store storage.Interface) {
		err := updateWifiNetworkList(wifi, store)
		if err != nil {
			logrus.Warn(err)
		}
	})
	return &wifiNetworkREST{
		REST: NewREST(&deviceapi.WifiNetwork{}, store),
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

func updateWifiNetworkList(w *wifi.Wifi, wifiNetworks storage.Interface) error {
	foundNetworks := map[string]struct{}{}
	networks, err := w.Scan()
	if err != nil {
		return err
	}
	for _, network := range networks {
		n := &deviceapi.WifiNetwork{}
		n.Name = fmt.Sprintf("ssid-%s", network.SSID)
		n.Name = utils.TruncateName(n.Name, utils.MaxResourceNameLength)
		n.Data.SSID = network.SSID
		foundNetworks[n.Name] = struct{}{}
		logrus.WithField("ssid", n.Data.SSID).Info("discovered new wifi network")
		err := wifiNetworks.Create(n.Name, n)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			return err
		}
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
