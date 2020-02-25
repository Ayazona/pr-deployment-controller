package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StringMatch struct {
	Prefix string `json:"prefix"`
}

type PortSelector struct {
	Number int64 `json:"number"`
}

type Destination struct {
	Host string       `json:"host"`
	Port PortSelector `json:"port"`
}

type HTTPMatchRequest struct {
	URI StringMatch `json:"uri"`
}

type DestinationWeight struct {
	Destination Destination `json:"destination"`
}

type HTTPRedirect struct {
	URI string `json:"uri"`
}

type HTTPRoute struct {
	Match            []HTTPMatchRequest  `json:"match"`
	Destination      []DestinationWeight `json:"route,omitempty"`
	Redirect         *HTTPRedirect       `json:"redirect,omitempty"`
	WebsocketUpgrade bool                `json:"websocketUpgrade"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VirtualServiceSpec defines the desired state of VirtualService
type VirtualServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Hosts    []string    `json:"hosts"`
	Gateways []string    `json:"gateways,omitempty"`
	HTTP     []HTTPRoute `json:"http"`
}

// VirtualServiceStatus defines the observed state of VirtualService
type VirtualServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualService is the Schema for the virtualservices API
// +k8s:openapi-gen=true
type VirtualService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualServiceSpec   `json:"spec,omitempty"`
	Status VirtualServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualServiceList contains a list of VirtualService
type VirtualServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualService{}, &VirtualServiceList{})
}
