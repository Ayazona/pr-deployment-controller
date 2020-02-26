package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EnvSpec defines a set of env variables used by services
type EnvSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PortSpec defines a port exposed by a service
type PortSpec struct {
	Name string `json:"name"`
	Port int64  `json:"port"`
}

// InitContainerSpec stores initial containers
type InitContainerSpec struct {
	Name    string          `json:"name"`
	Image   string          `json:"image"`
	Env     []corev1.EnvVar `json:"env,omitempty"`
	Command []string        `json:"command,omitempty"`
	Args    []string        `json:"args,omitempty"`
}

// ServiceSpec defines a service required by the environment
type ServiceSpec struct {
	Name           string                  `json:"name"`
	Image          string                  `json:"image"`
	Ports          []PortSpec              `json:"ports,omitempty"`
	Env            []corev1.EnvVar         `json:"env,omitempty"`
	Args           []string                `json:"args,omitempty"`
	Protected      bool                    `json:"protected"`
	ReadinessProbe *v1.Probe               `json:"readinessProbe,omitempty"`
	LivenessProbe  *v1.Probe               `json:"livenessProbe,omitempty"`
	Resources      v1.ResourceRequirements `json:"resources,omitempty"`
	InitContainers []InitContainerSpec     `json:"initContainers,omitempty"`
	SharedDirs     []string                `json:"sharedDirs,omitempty"`
}

// TaskSpec defines the tasks based on the build image to run (migrations)
type TaskSpec struct {
	Name      string                  `json:"name"`
	Env       []corev1.EnvVar         `json:"env,omitempty"`
	Args      []string                `json:"args,omitempty"`
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
}

// ExecSpec defines a command that is available through the remote terminal
type ExecSpec struct {
	Name string   `json:"name"`
	Cmd  []string `json:"cmd"`
}

// ContainerSpec defines the containers to start based on the build image
type ContainerSpec struct {
	Name           string                  `json:"name"`
	Ports          []PortSpec              `json:"ports,omitempty"`
	Env            []corev1.EnvVar         `json:"env,omitempty"`
	Args           []string                `json:"args,omitempty"`
	ReadinessProbe *v1.Probe               `json:"readinessProbe,omitempty"`
	LivenessProbe  *v1.Probe               `json:"livenessProbe,omitempty"`
	Resources      v1.ResourceRequirements `json:"resources,omitempty"`
	RemoteTerminal []ExecSpec              `json:"remoteTerminal,omitempty"`
}

// RoutingSpec defines the routing rules into the environment
type RoutingSpec struct {
	ContainerName string `json:"containerName"`
	Port          int64  `json:"port"`
	URLPrefix     string `json:"urlPrefix,omitempty"`
}

// RedirectSpec defines redirecs to other locations
type RedirectSpec struct {
	URLPrefix   string `json:"urlPrefix"`
	Destination string `json:"destination"`
}

// LinkSpec defines a link that the pr-deployment-controller
// adds to the PR comment
type LinkSpec struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// EnvironmentSpec defines the desired state of Environment
type EnvironmentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Required background services, databases, caches
	Services []ServiceSpec `json:"services,omitempty"`
	// Environment variables shared by all tasks and containers
	SharedEnv []EnvSpec `json:"sharedEnv,omitempty"`
	// Tasks to execute before starting the long-running containers
	Tasks []TaskSpec `json:"tasks,omitempty"`
	// Container to execute based on the build image
	Containers []ContainerSpec `json:"containers"`
	// Routing rules used to reach the environment containers
	Routing []RoutingSpec `json:"routing"`
	// Redirect rules used to direct traffic to other locations
	Redirects []RedirectSpec `json:"redirects,omitempty"`
	// Allow scheduling of pods on nodes with labels matching this map
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Claim database based on a template
	DatabaseTemplate *string `json:"databaseTemplate,omitempty"`
	// Links included in the PR comment
	Links []LinkSpec `json:"links,omitempty"`
	// Dont build prs on the first commit from these users
	IgnoredUsers []string `json:"ignoredUsers,omitempty"`
	// Dont deploy on demand builds automatically
	OnDemand bool `json:"onDemand,omitempty"`
}

// EnvironmentStatus defines the observed state of Environment
type EnvironmentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Environment is the Schema for the environments API
// +k8s:openapi-gen=true
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EnvironmentList contains a list of Environment
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
