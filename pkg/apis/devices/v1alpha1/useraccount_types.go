package v1alpha1

import (
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	AdminUserAccount = "admin"
)

// UserAccountData specifies the user account.
// +k8s:openapi-gen=true
type UserAccountData struct {
	// PasswordString allows to submit a new password in plain text to make the server bcrypt-encode it.
	PasswordString string `json:"passwordString,omitempty"`
	// Password holds the bcrypt-encoded password.
	Password string `json:"password,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserAccount is the schema for UserAccount resources.
// +k8s:openapi-gen=true
type UserAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Data UserAccountData `json:"data"`
}

func (in *UserAccount) New() resource.Resource {
	return &UserAccount{}
}

func (in *UserAccount) NewList() runtime.Object {
	return &UserAccountList{}
}

func (in *UserAccount) GetGroupVersionResource() schema.GroupVersionResource {
	return GroupVersion.WithResource("useraccounts")
}

func (in *UserAccount) DeepCopyIntoResource(res resource.Resource) error {
	d, ok := res.(*UserAccount)
	if !ok {
		return fmt.Errorf("expected resource of type UserAccount but received %T", res)
	}
	in.DeepCopyInto(d)
	return nil
}

// UserAccountList contains a list of UserAccount resources.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserAccount `json:"items"`
}
