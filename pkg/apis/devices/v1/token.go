package v1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:openapi-gen=true
// DeviceTokenData defines the desired state of Cache
type DeviceTokenData struct {
	Token string `json:"token"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeviceToken is the Schema for the devices API
// +k8s:openapi-gen=true
type DeviceToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Data DeviceTokenData `json:"data"`
}

func (in *DeviceToken) New() resource.Resource {
	return &DeviceToken{}
}

func (in *DeviceToken) NewList() runtime.Object {
	return &DeviceTokenList{}
}

func (in *DeviceToken) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("devices")
}

func (in *DeviceToken) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*DeviceToken)
	if !ok {
		return fmt.Errorf("expected resource of type Device but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// DeviceTokenList contains a list of Cache
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DeviceTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceToken `json:"items"`
}
