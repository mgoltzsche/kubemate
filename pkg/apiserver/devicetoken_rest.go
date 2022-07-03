package apiserver

import (
	"context"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

type deviceTokenREST struct {
	*REST
	deviceName string
}

func NewDeviceTokenREST(dir string, scheme *runtime.Scheme, deviceName string) (*deviceTokenREST, error) {
	store, err := storage.FileStore(dir, &deviceapi.DeviceToken{}, scheme)
	if err != nil {
		return nil, err
	}
	r := &deviceTokenREST{
		REST:       NewREST(&deviceapi.DeviceToken{}, store),
		deviceName: deviceName,
	}
	// Generate new cluster join token for this device if not exist
	token := &deviceapi.DeviceToken{}
	err = store.Get(deviceName, token)
	if err != nil || token.Data.Token == "" {
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		_, err = r.regenerateClusterJoinToken()
		if err != nil {
			return nil, fmt.Errorf("generate cluster join token: %w", err)
		}
	}
	return r, nil
}

func (r *deviceTokenREST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	if key == r.deviceName {
		return nil, false, fmt.Errorf("refusing to update cluster join token on the server manually. please delete it to force regeneration")
	}
	return r.REST.Update(ctx, key, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

func (r *deviceTokenREST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	if m.GetName() == r.deviceName {
		return nil, fmt.Errorf("refusing to create cluster join token on the server manually. please delete it to force regeneration")
	}
	return r.REST.Create(ctx, obj, createValidation, options)
}

func (r *deviceTokenREST) Delete(ctx context.Context, key string, deleteValidation registryrest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	if key == r.deviceName {
		t := &deviceapi.DeviceToken{}
		err := r.Store.Get(r.deviceName, t)
		if err != nil {
			return nil, false, err
		}
		r.regenerateClusterJoinToken()
		return nil, false, nil
	}
	return r.REST.Delete(ctx, key, deleteValidation, options)
}

func (r *deviceTokenREST) regenerateClusterJoinToken() (*deviceapi.DeviceToken, error) {
	token, err := generateRandomString(32)
	if err != nil {
		return nil, err
	}
	t := &deviceapi.DeviceToken{}
	err = r.Store.Get(r.deviceName, t)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		t.Data.Token = token
		err = r.Store.Create(r.deviceName, t)
		if err != nil {
			return nil, err
		}
		return t, nil
	}
	err = r.Store.Update(r.deviceName, t, func() (resource.Resource, error) {
		t.Data.Token = token
		return t, nil
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}
