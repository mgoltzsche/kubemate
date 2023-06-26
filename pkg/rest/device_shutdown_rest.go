package rest

import (
	"context"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/clientconf"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
)

var (
	_ registryrest.Creater = &DeviceShutdownREST{}
)

type DeviceShutdownREST struct {
	deviceName  string
	deviceStore storage.Interface
	k3sDir      string
}

func NewDeviceShutdownREST(deviceName string, deviceStore storage.Interface, k3sDir string) *DeviceShutdownREST {
	return &DeviceShutdownREST{
		deviceName:  deviceName,
		deviceStore: deviceStore,
		k3sDir:      k3sDir,
	}
}

// Create initiates an orderly shutdown of this node.
func (r *DeviceShutdownREST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	// Fetch device resource
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	if name := m.GetName(); name == r.deviceName {
		return nil, errors.NewNotFound(deviceapi.GroupVersion.WithResource("devices/shutdown").GroupResource(), name)
	}
	var d deviceapi.Device
	err = r.deviceStore.Get(r.deviceName, &d)
	if err != nil {
		return nil, err
	}

	// Create cluster client
	config, err := clientconf.New(r.k3sDir, d.Spec.Mode)
	if err != nil {
		return nil, err
	}
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Set Node annotation to make the node controller on the master drain the node with higher cluster privileges.
	p := fmt.Sprintf(`{"metadata":{"annotations":{%q:"true"}}}`, deviceapi.NodeDrainAnnotation)
	_, err = c.CoreV1().Nodes().Patch(ctx, r.deviceName, types.StrategicMergePatchType, []byte(p), metav1.PatchOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (r *DeviceShutdownREST) Destroy() {}

func (r *DeviceShutdownREST) New() runtime.Object {
	return &deviceapi.Device{}
}
