package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "kubemate.mgoltzsche.github.com", Version: "v1"}
)

func AddToScheme(s *runtime.Scheme) error {
	metav1.AddToGroupVersion(s, GroupVersion)
	s.AddKnownTypes(GroupVersion,
		&Device{}, &DeviceList{},
		&DeviceDiscovery{}, &DeviceDiscoveryList{},
		&DeviceToken{}, &DeviceTokenList{},
		&WifiNetwork{}, &WifiNetworkList{},
		&WifiPassword{}, &WifiPasswordList{},
	)
	return nil
}
