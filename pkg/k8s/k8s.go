package k8s

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Environment contains the required properties to interact with the apiserver
type Environment struct {
	client.Client
	scheme      *runtime.Scheme
	Namespace   string
	BuildPrefix string
}

// New returns a new k8s environment
func New(mgr manager.Manager, namespace, buildprefix string) (*Environment, error) {
	return &Environment{mgr.GetClient(), mgr.GetScheme(), namespace, buildprefix}, nil
}
