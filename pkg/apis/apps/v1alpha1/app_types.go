package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AppStateNotInstalled AppState = "NotInstalled"
	AppStateInstalling   AppState = "Installing"
	AppStateUpgrading    AppState = "Upgrading"
	AppStateInstalled    AppState = "Installed"
	AppStateDeinstalling AppState = "Deinstalling"
	AppStateError        AppState = "Error"
)

// +enum
type AppState string

// AppSpec defines the desired state of the App.
// +k8s:openapi-gen=true
type AppSpec struct {
	Enabled       *bool              `json:"enabled,omitempty"`
	Kustomization *KustomizationSpec `json:"kustomization,omitempty"`
}

// KustomizationSpec specifies the kustomization that should be installed.
// +k8s:openapi-gen=true
type KustomizationSpec struct {
	// Reference of the source where the kustomization file is.
	// +required
	SourceRef CrossNamespaceSourceReference `json:"sourceRef"`
	// Path points to the kustomization directory within the sourceRef.
	Path string `json:"path,omitempty"`
	// TargetNamespace specifies the Kubernetes Namespace the kustomization should be installed to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Optional
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// Namespace specifies the default Kubernetes Namespace that śhould be used by the kustomization.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
	// +kubebuilder:validation:Optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// CrossNamespaceSourceReference contains enough information to let you locate the typed Kubernetes resource object at cluster level.
// +k8s:openapi-gen=true
type CrossNamespaceSourceReference struct {
	// API version of the referent.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent.
	// +kubebuilder:validation:Enum=GitRepository;Bucket
	// +required
	Kind string `json:"kind"`

	// Name of the referent.
	// +required
	Name string `json:"name"`

	// Namespace of the referent, defaults to the namespace of the Kubernetes resource object that contains the reference.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// AppStatus defines the observed state of the App.
// +k8s:openapi-gen=true
type AppStatus struct {
	ObservedGeneration    int64    `json:"observedGeneration,omitempty"`
	Namespace             string   `json:"namespace,omitempty"`
	State                 AppState `json:"state,omitempty"`
	Message               string   `json:"message,omitempty"`
	LastAppliedRevision   string   `json:"lastAppliedRevision,omitempty"`
	LastAttemptedRevision string   `json:"lastAttemptedRevision,omitempty"`
}

// App is the Schema for the apps API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.lastAppliedRevision`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec"`
	Status AppStatus `json:"status,omitempty"`
}

func (a *App) Namespace() string {
	if a.Spec.Kustomization != nil && a.Spec.Kustomization.Namespace != "" {
		return a.Spec.Kustomization.Namespace
	}
	return a.ObjectMeta.Namespace
}

func (s *CrossNamespaceSourceReference) String() string {
	if s.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name)
	}
	return fmt.Sprintf("%s/%s", s.Kind, s.Name)
}

// AppList contains a list of App
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}
