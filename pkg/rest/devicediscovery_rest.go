package rest

import (
	"context"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

type deviceDiscoveryREST struct {
	*REST
}

func NewDeviceDiscoveryREST(store storage.Interface) *deviceDiscoveryREST {
	return &deviceDiscoveryREST{
		REST: NewREST(&deviceapi.DeviceDiscovery{}, store),
	}
}

func (r *deviceDiscoveryREST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("cannot update device discovery result")
}
