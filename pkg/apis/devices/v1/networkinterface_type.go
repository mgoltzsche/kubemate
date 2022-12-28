package v1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:openapi-gen=true
// NetworkInterfaceStatus defines the observed state of the network interface.
type NetworkInterfaceStatus struct {
	Up bool `json:"up"`
}

// NetworkInterface is the Schema for the network interface API.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Status NetworkInterfaceStatus `json:"status"`
}

func (in *NetworkInterface) New() resource.Resource {
	return &NetworkInterface{}
}

func (in *NetworkInterface) NewList() runtime.Object {
	return &NetworkInterfaceList{}
}

func (in *NetworkInterface) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("networkinterfaces")
}

func (in *NetworkInterface) DeepCopyIntoResource(res resource.Resource) error {
	r, ok := res.(*NetworkInterface)
	if !ok {
		return fmt.Errorf("expected resource of type NetworkInterface but received %T", res)
	}
	in.DeepCopyInto(r)
	return nil
}

// NetworkInterfaceList contains a list of network interfaces.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkInterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkInterface `json:"items"`
}
