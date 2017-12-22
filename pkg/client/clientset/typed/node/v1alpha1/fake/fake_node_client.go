package fake

import (
	v1alpha1 "github.com/gardener/node-controller-manager/pkg/client/clientset/typed/node/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeNodeV1alpha1 struct {
	*testing.Fake
}

func (c *FakeNodeV1alpha1) AWSInstanceClasses() v1alpha1.AWSInstanceClassInterface {
	return &FakeAWSInstanceClasses{c}
}

func (c *FakeNodeV1alpha1) Instances() v1alpha1.InstanceInterface {
	return &FakeInstances{c}
}

func (c *FakeNodeV1alpha1) InstanceDeployments() v1alpha1.InstanceDeploymentInterface {
	return &FakeInstanceDeployments{c}
}

func (c *FakeNodeV1alpha1) InstanceSets() v1alpha1.InstanceSetInterface {
	return &FakeInstanceSets{c}
}

func (c *FakeNodeV1alpha1) InstanceTemplates(namespace string) v1alpha1.InstanceTemplateInterface {
	return &FakeInstanceTemplates{c, namespace}
}

func (c *FakeNodeV1alpha1) Scales(namespace string) v1alpha1.ScaleInterface {
	return &FakeScales{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeNodeV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
