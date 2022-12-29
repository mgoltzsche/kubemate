package fake

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "fakegroup", Version: "v1alpha1"}
)

func AddToScheme(s *runtime.Scheme) error {
	metav1.AddToGroupVersion(s, GroupVersion)
	s.AddKnownTypes(GroupVersion, &FakeResourceList{})
	s.AddKnownTypeWithName(GroupVersion.WithKind("FakeResource"), &FakeResource{})
	return nil
}
