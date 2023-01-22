package rest

import (
	"context"
	"encoding/base64"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

type certificateREST struct {
	*REST
}

func NewCertificateREST(scheme *runtime.Scheme, caCert []byte) *certificateREST {
	store := storage.InMemory(scheme)
	c := &deviceapi.Certificate{}
	c.Name = "self"
	c.Spec.CACert = base64.StdEncoding.EncodeToString(caCert)
	err := store.Create("self", c)
	if err != nil {
		panic(err)
	}
	return &certificateREST{
		REST: NewREST(&deviceapi.Certificate{}, store),
	}
}

func (r *certificateREST) Delete(ctx context.Context, key string, deleteValidation registryrest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("cannot delete certificate")
}

func (r *certificateREST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("cannot create certificate")
}

func (r *certificateREST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("cannot update certificate")
}
