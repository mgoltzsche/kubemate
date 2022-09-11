package v1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:openapi-gen=true
// WifiNetworkData defines the desired state of Cache
type WifiNetworkData struct {
	SSID string `json:"ssid"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WifiNetwork is the Schema for the devices API
// +k8s:openapi-gen=true
type WifiNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Data WifiNetworkData `json:"data"`
}

func (in *WifiNetwork) New() resource.Resource {
	return &WifiNetwork{}
}

func (in *WifiNetwork) NewList() runtime.Object {
	return &WifiNetworkList{}
}

func (in *WifiNetwork) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("wifinetworks")
}

func (in *WifiNetwork) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*WifiNetwork)
	if !ok {
		return fmt.Errorf("expected resource of type WifiNetwork but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// WifiNetworkList contains a list of Cache
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WifiNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WifiNetwork `json:"items"`
}
