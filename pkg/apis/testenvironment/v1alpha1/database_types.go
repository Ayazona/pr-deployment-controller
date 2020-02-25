package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DatabasePhase describes the different phases a database can be in.
type DatabasePhase string

var (
	DatabasePending      DatabasePhase = "pending"
	DatabaseProvisioning DatabasePhase = "provisioning"
	DatabaseReady        DatabasePhase = "ready"
	DatabaseClaimed      DatabasePhase = "claimed"
)

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	TemplateName string `json:"templateName"`
}

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
	BuildName    string        `json:"buildName,omitempty"`
	Phase        DatabasePhase `json:"phase"`
	Host         string        `json:"host,omitempty"`
	Port         int64         `json:"port,omitempty"`
	Username     string        `json:"username,omitempty"`
	Password     string        `json:"password,omitempty"`
	DatabaseName string        `json:"databaseName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Database is the Schema for the databases API
// +k8s:openapi-gen=true
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
