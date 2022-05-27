package v1

import (
	"fmt"

	"github.com/mgoltzsche/k3spi/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	//"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// DeviceState specifies the state of a device.
// +enum
type DeviceState string

// DeviceMode specifies the operating mode of a device.
// +enum
type DeviceMode string

const (
	DeviceStateUnknown  DeviceState = "unknown"
	DeviceStateStarting DeviceState = "starting"
	DeviceStateRunning  DeviceState = "running"
	DeviceStateError    DeviceState = "error"
	DeviceStateExited   DeviceState = "exited"
	DeviceModeServer    DeviceMode  = "server"
	DeviceModeAgent     DeviceMode  = "agent"
)

// +k8s:openapi-gen=true
// DeviceSpec defines the desired state of Cache
type DeviceSpec struct {
	Mode DeviceMode `json:"role"`
}

// +k8s:openapi-gen=true
// DeviceStatus defines the observed state of Cache
type DeviceStatus struct {
	State   DeviceState `json:"state,omitempty"`
	Message string      `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Device is the Schema for the devices API
// +k8s:openapi-gen=true
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DeviceSpec   `json:"spec"`
	Status DeviceStatus `json:"status"`
}

func (in *Device) New() resource.Resource {
	return &Device{}
}

func (in *Device) NewList() runtime.Object {
	return &DeviceList{}
}

func (in *Device) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("devices")
}

func (in *Device) GetStatus() resource.SubResource {
	return &in.Status
}

func (in *Device) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*Device)
	if !ok {
		return fmt.Errorf("expected resource of type Device but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// DeviceList contains a list of Cache
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}
