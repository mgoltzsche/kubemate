package apiserver

import (
	"context"
	"fmt"

	"github.com/hashicorp/mdns"
	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

// TODO: advertise as mdns service, see https://github.com/holoplot/go-avahi#publishing

var (
	_ registryrest.Lister  = &DeviceREST{}
	_ registryrest.Getter  = &DeviceREST{}
	_ registryrest.Updater = &DeviceREST{}
)

type DeviceREST struct {
	rest       *REST
	runner     *runner.Runner
	deviceName string
	registryrest.TableConvertor
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
	r := NewREST(&deviceapi.Device{}, storage.InMemory())
	err := r.Store.Create(deviceName, device)
	if err != nil {
		panic(err)
	}
	r.TableConvertor = &deviceTableConvertor{}
	devices := &DeviceREST{
		rest:           r,
		deviceName:     device.Name,
		TableConvertor: r,
	}
	go devices.populate()
	return devices
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
	go r.populate()
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

func (r *DeviceREST) populate() {
	err := populateDevicesFromMDNS(r.deviceName, r.rest.Store)
	if err != nil {
		logrus.WithError(err).Error("failed to find devices via mdns")
	}
}

func populateDevicesFromMDNS(deviceName string, devices storage.Interface) error {
	foundDevices := map[string]struct{}{
		deviceName: struct{}{},
	}
	ch := make(chan *mdns.ServiceEntry, 4)
	go func() {
		for entry := range ch {
			fmt.Printf("## Found mdns entry: %v\n", entry)
			d := &deviceapi.Device{}
			d.Name = entry.Host
			foundDevices[d.Name] = struct{}{}
			if d.Name == deviceName {
				continue
			}
			err := devices.Get(d.Name, d)
			if errors.IsNotFound(err) {
				err = devices.Create(d.Name, d)
			}
			if err != nil {
				logrus.WithError(err).Error("failed to register device")
			}
		}
	}()

	mdns.Lookup("_tcp", ch)
	close(ch)

	// Remove old devices
	l := &deviceapi.DeviceList{}
	err := devices.List(l)
	if err != nil {
		return fmt.Errorf("scan for devices: %w", err)
	}
	for _, d := range l.Items {
		if _, ok := foundDevices[d.Name]; !ok {
			if e := devices.Delete(d.Name); e != nil && err == nil {
				err = e
			}
		}
	}
	return err
}
