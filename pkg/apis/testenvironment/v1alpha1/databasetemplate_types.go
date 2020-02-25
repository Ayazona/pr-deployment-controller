package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DatabaseTemplateSpec defines the desired state of DatabaseTemplate
type DatabaseTemplateSpec struct {
	DumpFile        string `json:"dumpFile"`
	Credentials     string `json:"credentials"`
	RefreshInterval string `json:"refreshInterval"`
	BufferSize      int64  `json:"bufferSize"`

	DatabaseName    string `json:"databaseName"`
	DatabaseUser    string `json:"databaseUser"`
	DatabaseVersion string `json:"databaseVersion"`

	Resources      v1.ResourceRequirements `json:"resources,omitempty"`
	VolumeCapacity v1.ResourceRequirements `json:"volumeCapacity,omitempty"`
	NodeSelector   map[string]string       `json:"nodeSelector,omitempty"`
}

// DatabaseTemplateStatus defines the observed state of DatabaseTemplate
type DatabaseTemplateStatus struct{}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DatabaseTemplate is the Schema for the databasetemplates API
// +k8s:openapi-gen=true
type DatabaseTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseTemplateSpec   `json:"spec,omitempty"`
	Status DatabaseTemplateStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DatabaseTemplateList contains a list of DatabaseTemplate
type DatabaseTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DatabaseTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DatabaseTemplate{}, &DatabaseTemplateList{})
}
