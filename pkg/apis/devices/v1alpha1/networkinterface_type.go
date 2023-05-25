package v1alpha1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NetworkInterfaceType specifies the type of a network interface.
// +enum
type NetworkInterfaceType string

// WifiMode specifies the operating mode of the wifi device.
// +enum
type WifiMode string

const (
	NetworkInterfaceTypeEther NetworkInterfaceType = "ether"
	NetworkInterfaceTypeWifi  NetworkInterfaceType = "wifi"
	WifiModeDisabled          WifiMode             = "disabled"
	WifiModeStation           WifiMode             = "station"
	WifiModeAccessPoint       WifiMode             = "accesspoint"
)

// NetworkInterfaceStatus defines the observed state of the network interface.
// +k8s:openapi-gen=true
type NetworkInterfaceStatus struct {
	Link  NetworkLinkStatus `json:"link,omitempty"`
	Error string            `json:"error,omitempty"`
}

// NetworkLinkStatus defines the observed state of the network link.
// +k8s:openapi-gen=true
type NetworkLinkStatus struct {
	Index int                  `json:"index,omitempty"`
	Type  NetworkInterfaceType `json:"type,omitempty"`
	Up    bool                 `json:"up"`
	MAC   string               `json:"mac,omitempty"`
	IP4   string               `json:"ip4,omitempty"`
	Error string               `json:"error,omitempty"`
}

// NetworkInterfaceSpec defines the network interface configuration.
// +k8s:openapi-gen=true
type NetworkInterfaceSpec struct {
	Wifi WifiSpec `json:"wifi,omitempty"`
}

// WifiSpec defines the wifi configuration for the device.
// +k8s:openapi-gen=true
type WifiSpec struct {
	Mode        WifiMode            `json:"mode"`
	CountryCode string              `json:"countryCode,omitempty"`
	Station     WifiStationSpec     `json:"station"`
	AccessPoint WifiAccessPointSpec `json:"accessPoint"`
}

// WifiStationSpec defines the wifi client configuration.
// +k8s:openapi-gen=true
type WifiStationSpec struct {
	SSID string `json:'"ssid,omitempty"`
}

// WifiAccessPointSpec defines the wifi access point configuration.
// +k8s:openapi-gen=true
type WifiAccessPointSpec struct {
	SSID string `json:'"ssid,omitempty"`
}

// NetworkInterface is the Schema for the network interface API.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   NetworkInterfaceSpec   `json:"spec"`
	Status NetworkInterfaceStatus `json:"status"`
}

func (in *NetworkInterface) New() resource.Resource {
	return &NetworkInterface{}
}

func (in *NetworkInterface) NewList() runtime.Object {
	return &NetworkInterfaceList{}
}

func (in *NetworkInterface) GetSingularName() string {
	return "NetworkInterface"
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

func (in *NetworkInterface) GetStatus() resource.SubResource {
	return &in.Status
}

// NetworkInterfaceList contains a list of network interfaces.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkInterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkInterface `json:"items"`
}
