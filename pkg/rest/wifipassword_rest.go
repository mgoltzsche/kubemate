package rest

import (
	"context"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/passwordgen"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

type wifiPasswordREST struct {
	*REST
}

func NewWifiPasswordREST(dir string, scheme *runtime.Scheme) (*wifiPasswordREST, error) {
	store, err := storage.FileStore(dir, &deviceapi.WifiPassword{}, scheme)
	if err != nil {
		return nil, err
	}
	r := &wifiPasswordREST{
		REST: NewREST(&deviceapi.WifiPassword{}, store),
	}
	// Generate access point password if not defined
	pw := &deviceapi.WifiPassword{}
	err = store.Get(deviceapi.AccessPointPasswordKey, pw)
	if err != nil || pw.Data.Password == "" {
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		err = r.regenerateWifiPassword()
		if err != nil {
			return nil, fmt.Errorf("generate wifi password: %w", err)
		}
	}
	return r, nil
}

func (r *wifiPasswordREST) Delete(ctx context.Context, key string, deleteValidation registryrest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	if key == deviceapi.AccessPointPasswordKey {
		pw := &deviceapi.WifiPassword{}
		err := r.Store().Get(deviceapi.AccessPointPasswordKey, pw)
		if err != nil {
			return nil, false, err
		}
		err = r.regenerateWifiPassword()
		if err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	return r.REST.Delete(ctx, key, deleteValidation, options)
}

func (r *wifiPasswordREST) regenerateWifiPassword() error {
	password, err := passwordgen.GenerateMemorablePassword()
	if err != nil {
		return err
	}
	pw := &deviceapi.WifiPassword{}
	err = r.Store().Get(deviceapi.AccessPointPasswordKey, pw)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		pw.Data.Password = password
		err = r.Store().Create(deviceapi.AccessPointPasswordKey, pw)
		if err != nil {
			return err
		}
		return nil
	}
	err = r.Store().Update(deviceapi.AccessPointPasswordKey, pw, func() error {
		pw.Data.Password = password
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
