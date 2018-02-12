package fake

import (
	internalversion "github.com/gardener/machine-controller-manager/pkg/client/clientset/internalversion/typed/machine/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeMachine struct {
	*testing.Fake
}

func (c *FakeMachine) AWSMachineClasses(namespace string) internalversion.AWSMachineClassInterface {
	return &FakeAWSMachineClasses{c, namespace}
}

func (c *FakeMachine) AzureMachineClasses(namespace string) internalversion.AzureMachineClassInterface {
	return &FakeAzureMachineClasses{c, namespace}
}

func (c *FakeMachine) GCPMachineClasses(namespace string) internalversion.GCPMachineClassInterface {
	return &FakeGCPMachineClasses{c, namespace}
}

func (c *FakeMachine) Machines(namespace string) internalversion.MachineInterface {
	return &FakeMachines{c, namespace}
}

func (c *FakeMachine) MachineDeployments(namespace string) internalversion.MachineDeploymentInterface {
	return &FakeMachineDeployments{c, namespace}
}

func (c *FakeMachine) MachineSets(namespace string) internalversion.MachineSetInterface {
	return &FakeMachineSets{c, namespace}
}

func (c *FakeMachine) MachineTemplates(namespace string) internalversion.MachineTemplateInterface {
	return &FakeMachineTemplates{c, namespace}
}

func (c *FakeMachine) OpenStackMachineClasses(namespace string) internalversion.OpenStackMachineClassInterface {
	return &FakeOpenStackMachineClasses{c, namespace}
}

func (c *FakeMachine) Scales(namespace string) internalversion.ScaleInterface {
	return &FakeScales{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeMachine) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
