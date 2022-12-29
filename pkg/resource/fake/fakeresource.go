package fake

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:openapi-gen=true
// FakeResourceSpec defines the desired state of the Device.
type FakeResourceSpec struct {
	ValueA string `json:"valueA"`
	ValueB string `json:"valueB"`
}

// FakeResource is the Schema for the device discovery API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type FakeResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec FakeResourceSpec `json:"spec"`
}

func (in *FakeResource) New() resource.Resource {
	return &FakeResource{}
}

func (in *FakeResource) NewList() runtime.Object {
	return &FakeResourceList{}
}

func (in *FakeResource) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("fakeresource")
}

func (in *FakeResource) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*FakeResource)
	if !ok {
		return fmt.Errorf("expected resource of type Device but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// FakeResourceList contains a list of Cache
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type FakeResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FakeResource `json:"items"`
}
