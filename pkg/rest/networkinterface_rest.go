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

type networkInterfaceREST struct {
	*REST
}

func NewNetworkInterfaceREST(store storage.Interface) *networkInterfaceREST {
	return &networkInterfaceREST{
		REST: NewREST(&deviceapi.NetworkInterface{}, store),
	}
}

func (r *networkInterfaceREST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("cannot create network interface")
}
