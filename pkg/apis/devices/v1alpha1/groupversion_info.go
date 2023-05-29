package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "kubemate.mgoltzsche.github.com", Version: "v1alpha1"}
)

func AddToScheme(s *runtime.Scheme) error {
	metav1.AddToGroupVersion(s, GroupVersion)
	s.AddKnownTypeWithName(GroupVersion.WithKind("NetworkInterface"), &NetworkInterface{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("Device"), &Device{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("DeviceDiscovery"), &DeviceDiscovery{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("DeviceToken"), &DeviceToken{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("WifiNetwork"), &WifiNetwork{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("WifiPassword"), &WifiPassword{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("Certificate"), &Certificate{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("UserAccount"), &UserAccount{})
	s.AddKnownTypes(GroupVersion,
		&NetworkInterfaceList{},
		&DeviceList{},
		&DeviceDiscoveryList{},
		&DeviceTokenList{},
		&WifiNetworkList{},
		&WifiPasswordList{},
		&CertificateList{},
		&UserAccountList{},
	)
	return nil
}
