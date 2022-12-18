package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "kubemate.mgoltzsche.github.com", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	//SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	//AddToScheme = SchemeBuilder.AddToScheme
)

func AddToScheme(s *runtime.Scheme) error {
	/*s.AddKnownTypeWithName(GroupVersion.WithKind("Device"), &Device{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("DeviceList"), &DeviceList{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("DeviceToken"), &DeviceToken{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("DeviceTokenList"), &DeviceTokenList{})*/
	metav1.AddToGroupVersion(s, GroupVersion)
	s.AddKnownTypes(GroupVersion,
		&Device{}, &DeviceList{},
		&DeviceToken{}, &DeviceTokenList{},
		&WifiNetwork{}, &WifiNetworkList{},
		&WifiPassword{}, &WifiPasswordList{},
	)
	return nil
}
