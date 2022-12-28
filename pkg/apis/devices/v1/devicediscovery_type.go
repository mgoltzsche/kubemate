package v1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:openapi-gen=true
// DeviceDiscoverySpec defines the desired state of the Device.
type DeviceDiscoverySpec struct {
	Mode    DeviceMode `json:"mode"`
	Server  string     `json:"server,omitempty"`
	Address string     `json:"address"`
	Current bool       `json:"current,omitempty"`
}

// DeviceDiscovery is the Schema for the device discovery API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DeviceDiscovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec DeviceDiscoverySpec `json:"spec"`
}

func (in *DeviceDiscovery) New() resource.Resource {
	return &DeviceDiscovery{}
}

func (in *DeviceDiscovery) NewList() runtime.Object {
	return &DeviceDiscoveryList{}
}

func (in *DeviceDiscovery) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("devicediscovery")
}

func (in *DeviceDiscovery) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*DeviceDiscovery)
	if !ok {
		return fmt.Errorf("expected resource of type Device but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// DeviceDiscoveryList contains a list of Cache
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DeviceDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceDiscovery `json:"items"`
}
