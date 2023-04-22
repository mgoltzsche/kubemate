package v1alpha1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CertificateSpec defines a certificate.
// +k8s:openapi-gen=true
type CertificateSpec struct {
	CACert string `json:"caCert,omitempty"`
}

// Certificate is the Schema for the certificate API.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec CertificateSpec `json:"spec"`
}

func (in *Certificate) New() resource.Resource {
	return &Certificate{}
}

func (in *Certificate) NewList() runtime.Object {
	return &CertificateList{}
}

func (in *Certificate) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("certificates")
}

func (in *Certificate) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*Certificate)
	if !ok {
		return fmt.Errorf("expected resource of type Certificate but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// CertificateList contains a list of Certificate resources.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}
