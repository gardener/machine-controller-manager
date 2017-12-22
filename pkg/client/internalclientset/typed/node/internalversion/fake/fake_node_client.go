package fake

import (
	internalversion "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/internalclientset/typed/node/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeNode struct {
	*testing.Fake
}

func (c *FakeNode) AWSInstanceClasses() internalversion.AWSInstanceClassInterface {
	return &FakeAWSInstanceClasses{c}
}

func (c *FakeNode) Instances() internalversion.InstanceInterface {
	return &FakeInstances{c}
}

func (c *FakeNode) InstanceDeployments() internalversion.InstanceDeploymentInterface {
	return &FakeInstanceDeployments{c}
}

func (c *FakeNode) InstanceSets() internalversion.InstanceSetInterface {
	return &FakeInstanceSets{c}
}

func (c *FakeNode) InstanceTemplates(namespace string) internalversion.InstanceTemplateInterface {
	return &FakeInstanceTemplates{c, namespace}
}

func (c *FakeNode) Scales(namespace string) internalversion.ScaleInterface {
	return &FakeScales{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeNode) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
