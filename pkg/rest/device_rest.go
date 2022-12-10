package rest

import (
	"context"
	"sync"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
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
	rest            *REST
	runner          *runner.Runner
	deviceName      string
	deviceDiscovery func(store storage.Interface) error
	store           storage.Interface
	registryrest.TableConvertor
}

func NewDeviceREST(deviceName, storageDir string, scheme *runtime.Scheme, deviceDiscovery func(store storage.Interface) error) (*DeviceREST, error) {
	store, err := newDeviceStore(deviceName, storageDir, scheme)
	if err != nil {
		return nil, err
	}
	refreshingStore := storage.RefreshPeriodically(store, 10*time.Second, func(store storage.Interface) {
		logrus.Debug("scanning for devices within the local network")
		err := deviceDiscovery(store)
		if err != nil {
			logrus.WithError(err).Error("failed to discover devices via mdns")
		}
	})
	r := NewREST(&deviceapi.Device{}, refreshingStore)
	r.TableConvertor = &deviceTableConvertor{}
	devices := &DeviceREST{
		rest:            r,
		deviceName:      deviceName,
		deviceDiscovery: deviceDiscovery,
		TableConvertor:  r,
		store:           store,
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
	persistent storage.Interface
	deviceName string
	mutex      *sync.Mutex
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
				Wifi: deviceapi.WifiConfig{
					CountryCode: "", // auto-detected by device controller
					Mode:        deviceapi.WifiModeDisabled,
					AccessPoint: deviceapi.WifiAccessPointConf{
						SSID: deviceName,
					},
				},
			},
		}
		err = s.Create(deviceName, d)
		if err != nil {
			return nil, err
		}
	}
	mem := storage.InMemory()
	d.Status.Current = true
	d.Status.State = deviceapi.DeviceStateUnknown
	d.CreationTimestamp = metav1.Now()
	err = mem.Create(deviceName, d)
	if err != nil {
		return nil, err
	}
	return &deviceStore{
		Interface:  mem,
		persistent: s,
		deviceName: deviceName,
		mutex:      &sync.Mutex{},
	}, nil
}

func (s *deviceStore) Update(key string, res resource.Resource, modify func() (resource.Resource, error)) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if key == s.deviceName {
		// TODO: fix this - currently this rejects all updates with status 409
		/*m, err := meta.Accessor(res)
		if err != nil {
			return err
		}
		rv := res.GetResourceVersion()
		uid := m.GetUID()
		err = s.persistent.Update(key, res, modify)
		if err != nil {
			return err
		}
		res.SetResourceVersion(rv)
		m.SetUID(uid)*/
	}
	return s.Interface.Update(key, res, modify)
}

func (s *deviceStore) Delete(key string, res resource.Resource, validate func() error) error {
	return errors.NewBadRequest("refusing to delete device")
}
