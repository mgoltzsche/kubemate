package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ParamTypeString   ParamType = "string"
	ParamTypeText     ParamType = "text"
	ParamTypePassword ParamType = "password"
	ParamTypeNumber   ParamType = "number"
	ParamTypeBoolean  ParamType = "boolean"
	ParamTypeEnum     ParamType = "enum"
)

// +enum
type ParamType string

// AppConfigSchemaSpec defines the configuration schema for an App.
// +k8s:openapi-gen=true
type AppConfigSchemaSpec struct {
	Params []ParameterDefinition `json:"params,omitempty"`
}

// ParameterDefinition defines an application parameter.
// +k8s:openapi-gen=true
type ParameterDefinition struct {
	Name        string    `json:"name"`
	Type        ParamType `json:"type,omitempty"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Category    string    `json:"category,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
}

// AppConfigSchema is the Schema for the apps API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +k8s:openapi-gen=true
type AppConfigSchema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AppConfigSchemaSpec `json:"spec"`
}

// AppConfigSchemaList contains a list of App
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type AppConfigSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppConfigSchema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppConfigSchema{}, &AppConfigSchemaList{})
}
