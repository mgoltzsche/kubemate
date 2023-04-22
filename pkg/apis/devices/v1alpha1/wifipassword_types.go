package v1alpha1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	AccessPointPasswordKey = "accesspoint"
	SSIDAnnotation         = "ssid"
)

// +k8s:openapi-gen=true
// WifiPasswordData defines the wifi password data.
type WifiPasswordData struct {
	Password string `json:"password"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WifiPassword is the Schema for the wifi key API.
// +k8s:openapi-gen=true
type WifiPassword struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Data WifiPasswordData `json:"data"`
}

func (in *WifiPassword) New() resource.Resource {
	return &WifiPassword{}
}

func (in *WifiPassword) NewList() runtime.Object {
	return &WifiPasswordList{}
}

func (in *WifiPassword) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("wifipasswords")
}

func (in *WifiPassword) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*WifiPassword)
	if !ok {
		return fmt.Errorf("expected resource of type WifiPassword but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// WifiPasswordList contains a list of WifiPassword resources.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WifiPasswordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WifiPassword `json:"items"`
}
