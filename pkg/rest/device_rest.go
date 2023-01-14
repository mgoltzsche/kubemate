package rest

import (
	"context"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	//"k8s.io/apimachinery/pkg/api/meta"
)

var (
	_ registryrest.Lister  = &DeviceREST{}
	_ registryrest.Getter  = &DeviceREST{}
	_ registryrest.Updater = &DeviceREST{}
)

type DeviceREST struct {
	rest       *REST
	runner     *runner.Runner
	deviceName string
	store      storage.Interface
	registryrest.TableConvertor
}

func NewDeviceREST(deviceName, storageDir string, scheme *runtime.Scheme) (*DeviceREST, error) {
	store, err := newDeviceStore(deviceName, storageDir, scheme)
	if err != nil {
		return nil, fmt.Errorf("load device config store: %w", err)
	}
	r := NewREST(&deviceapi.Device{}, store)
	r.TableConvertor = &deviceTableConvertor{}
	devices := &DeviceREST{
		rest:           r,
		deviceName:     deviceName,
		TableConvertor: r,
		store:          store,
	}
	return devices, nil
}

func (r *DeviceREST) Destroy() {}

func (r *DeviceREST) Store() storage.Interface {
	return r.store
}

func (r *DeviceREST) New() runtime.Object {
	return r.rest.New()
}

func (r *DeviceREST) NewList() runtime.Object {
	return r.rest.NewList()
}

func (r *DeviceREST) NamespaceScoped() bool {
	return false
}

func (r *DeviceREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return r.rest.List(ctx, options)
}

func (r *DeviceREST) Update(ctx context.Context, name string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	if name != r.deviceName {
		return nil, false, errors.NewNotFound(r.rest.resource.GetGroupVersionResource().GroupResource(), name)
	}
	return r.rest.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

func (r *DeviceREST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (w watch.Interface, err error) {
	return r.rest.Watch(ctx, options)
}

func (r *DeviceREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.rest.Get(ctx, name, options)
}

type deviceStore struct {
	storage.Interface
}

func newDeviceStore(deviceName, dir string, scheme *runtime.Scheme) (storage.Interface, error) {
	s, err := storage.FileStore(dir, &deviceapi.Device{}, scheme)
	if err != nil {
		return nil, err
	}
	d := &deviceapi.Device{}
	err = s.Get(deviceName, d)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		d = &deviceapi.Device{
			ObjectMeta: metav1.ObjectMeta{
				Name: deviceName,
			},
			Spec: deviceapi.DeviceSpec{
				Mode: deviceapi.DeviceModeServer,
			},
			Status: deviceapi.DeviceStatus{
				Current: true,
				State:   deviceapi.DeviceStateUnknown,
			},
		}
		err = s.Create(deviceName, d)
		if err != nil {
			return nil, fmt.Errorf("create default device config: %w", err)
		}
	}
	return &deviceStore{Interface: s}, nil
}

func (s *deviceStore) Delete(key string, res resource.Resource, validate func() error) error {
	return errors.NewBadRequest("refusing to delete device")
}
