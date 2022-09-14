package v1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DeviceState specifies the state of a device.
// +enum
type DeviceState string

// DeviceMode specifies the operating mode of a device.
// +enum
type DeviceMode string

// WifiMode specifies the operating mode of the wifi device.
// +enum
type WifiMode string

const (
	DeviceStateUnknown     DeviceState = "unknown"
	DeviceStateStarting    DeviceState = "starting"
	DeviceStateRunning     DeviceState = "running"
	DeviceStateError       DeviceState = "error"
	DeviceStateExited      DeviceState = "exited"
	DeviceStateTerminating DeviceState = "terminating"
	DeviceModeServer       DeviceMode  = "server"
	DeviceModeAgent        DeviceMode  = "agent"
	WifiModeDisabled       WifiMode    = "disabled"
	WifiModeStation        WifiMode    = "station"
	WifiModeAccessPoint    WifiMode    = "accesspoint"
)

// +k8s:openapi-gen=true
// DeviceSpec defines the desired state of the Device.
type DeviceSpec struct {
	Mode   DeviceMode `json:"mode"`
	Server string     `json:"server,omitempty"`
	Wifi   WifiConfig `json:"wifi"`
}

// +k8s:openapi-gen=true
// WifiConfig defines the wifi configuration for the device.
type WifiConfig struct {
	Mode        WifiMode            `json:"mode,omitempty"`
	CountryCode string              `json:"countryCode,omitempty"`
	Station     WifiStationConf     `json:"station"`
	AccessPoint WifiAccessPointConf `json:"accessPoint"`
}

// +k8s:openapi-gen=true
// WifiStationConf defines the wifi client configuration.
type WifiStationConf struct {
	SSID string `json:'"ssid,omitempty"`
}

// +k8s:openapi-gen=true
// WifiAccessPointConf defines the wifi access point configuration.
type WifiAccessPointConf struct {
	SSID string `json:'"ssid,omitempty"`
}

// +k8s:openapi-gen=true
// DeviceStatus defines the observed state of Cache
type DeviceStatus struct {
	Generation  int64       `json:"generation,omitempty"`
	Current     bool        `json:"current"`
	State       DeviceState `json:"state,omitempty"`
	Message     string      `json:"message,omitempty"`
	Address     string      `json:"address,omitempty"`
	JoinAddress string      `json:"joinAddress,omitempty"`
	// TODO: add ips (currently this makes the code generation fail):
	//IPs []string `json:"ips,omitempty"`
}

// Device is the Schema for the devices API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
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
