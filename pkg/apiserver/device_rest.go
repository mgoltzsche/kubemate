package apiserver

import (
	"context"

	deviceapi "github.com/mgoltzsche/k3spi/pkg/apis/devices/v1"
	"github.com/mgoltzsche/k3spi/pkg/runner"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

var (
	_ registryrest.Lister  = &DeviceREST{}
	_ registryrest.Getter  = &DeviceREST{}
	_ registryrest.Updater = &DeviceREST{}
)

type DeviceREST struct {
	runner        *runner.Runner
	groupResource schema.GroupResource
	registryrest.TableConvertor
	device *deviceapi.Device
}

func NewDeviceREST(deviceName string) *DeviceREST {
	device := &deviceapi.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name: deviceName,
		},
		Spec: deviceapi.DeviceSpec{
			Mode: deviceapi.DeviceModeServer,
		},
		Status: deviceapi.DeviceStatus{
			State: deviceapi.DeviceStateUnknown,
		},
	}
	gr := deviceapi.GroupVersion.WithResource("devices").GroupResource()
	return &DeviceREST{
		device:        device,
		groupResource: gr,
		//TableConvertor: registryrest.NewDefaultTableConvertor(gr),
		TableConvertor: &deviceTableConvertor{},
	}
}

func (f *DeviceREST) New() runtime.Object {
	return &deviceapi.Device{}
}

func (f *DeviceREST) NewList() runtime.Object {
	return &deviceapi.DeviceList{}
}

func (f *DeviceREST) NamespaceScoped() bool {
	return false
}

func (f *DeviceREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	l := &deviceapi.DeviceList{}
	l.Items = []deviceapi.Device{
		*f.device,
	}
	// TODO: add devices found via mdns
	return l, nil
}

func (f *DeviceREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	if name != f.device.Name {
		return nil, errors.NewNotFound(f.groupResource, name)
	}
	return f.device, nil
}

func (f *DeviceREST) Update(ctx context.Context, name string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	if name != f.device.Name {
		return nil, false, errors.NewNotFound(f.groupResource, name)
	}
	// TODO: call runner if updated entity represents this device
	return nil, false, nil
}
