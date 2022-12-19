package rest

import (
	"context"
	"fmt"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

type deviceDiscoveryREST struct {
	*REST
}

func NewDeviceDiscoveryREST(store storage.Interface, deviceDiscovery func() error) *deviceDiscoveryREST {
	store = storage.RefreshPeriodically(store, 10*time.Second, func(store storage.Interface) {
		logrus.Debug("scanning for devices within the local network")
		err := deviceDiscovery()
		if err != nil {
			logrus.WithError(err).Error("failed to discover devices via mdns")
		}
	})
	return &deviceDiscoveryREST{
		REST: NewREST(&deviceapi.DeviceDiscovery{}, store),
	}
}

func (r *deviceDiscoveryREST) Delete(ctx context.Context, key string, deleteValidation registryrest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("cannot delete device discovery result")
}

func (r *deviceDiscoveryREST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("cannot create device discovery result")
}

func (r *deviceDiscoveryREST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("cannot update device discovery result")
}
