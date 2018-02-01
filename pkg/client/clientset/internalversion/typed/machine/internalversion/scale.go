package internalversion

import (
	rest "k8s.io/client-go/rest"
)

// ScalesGetter has a method to return a ScaleInterface.
// A group's client should implement this interface.
type ScalesGetter interface {
	Scales(namespace string) ScaleInterface
}

// ScaleInterface has methods to work with Scale resources.
type ScaleInterface interface {
	ScaleExpansion
}

// scales implements ScaleInterface
type scales struct {
	client rest.Interface
	ns     string
}

// newScales returns a Scales
func newScales(c *MachineClient, namespace string) *scales {
	return &scales{
		client: c.RESTClient(),
		ns:     namespace,
	}
}
