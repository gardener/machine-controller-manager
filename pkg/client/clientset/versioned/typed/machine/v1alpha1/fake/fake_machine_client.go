package fake

import (
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeMachineV1alpha1 struct {
	*testing.Fake
}

func (c *FakeMachineV1alpha1) AWSMachineClasses(namespace string) v1alpha1.AWSMachineClassInterface {
	return &FakeAWSMachineClasses{c, namespace}
}

func (c *FakeMachineV1alpha1) AzureMachineClasses(namespace string) v1alpha1.AzureMachineClassInterface {
	return &FakeAzureMachineClasses{c, namespace}
}

func (c *FakeMachineV1alpha1) GCPMachineClasses(namespace string) v1alpha1.GCPMachineClassInterface {
	return &FakeGCPMachineClasses{c, namespace}
}

func (c *FakeMachineV1alpha1) Machines(namespace string) v1alpha1.MachineInterface {
	return &FakeMachines{c, namespace}
}

func (c *FakeMachineV1alpha1) MachineDeployments(namespace string) v1alpha1.MachineDeploymentInterface {
	return &FakeMachineDeployments{c, namespace}
}

func (c *FakeMachineV1alpha1) MachineSets(namespace string) v1alpha1.MachineSetInterface {
	return &FakeMachineSets{c, namespace}
}

func (c *FakeMachineV1alpha1) MachineTemplates(namespace string) v1alpha1.MachineTemplateInterface {
	return &FakeMachineTemplates{c, namespace}
}

func (c *FakeMachineV1alpha1) OpenStackMachineClasses(namespace string) v1alpha1.OpenStackMachineClassInterface {
	return &FakeOpenStackMachineClasses{c, namespace}
}

func (c *FakeMachineV1alpha1) Scales(namespace string) v1alpha1.ScaleInterface {
	return &FakeScales{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeMachineV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
