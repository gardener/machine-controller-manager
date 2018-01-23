package fake

import (
	internalversion "github.com/gardener/node-controller-manager/pkg/client/internalclientset/typed/machine/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeMachine struct {
	*testing.Fake
}

func (c *FakeMachine) AWSMachineClasses() internalversion.AWSMachineClassInterface {
	return &FakeAWSMachineClasses{c}
}

func (c *FakeMachine) AzureMachineClasses() internalversion.AzureMachineClassInterface {
	return &FakeAzureMachineClasses{c}
}

func (c *FakeMachine) Machines() internalversion.MachineInterface {
	return &FakeMachines{c}
}

func (c *FakeMachine) MachineDeployments() internalversion.MachineDeploymentInterface {
	return &FakeMachineDeployments{c}
}

func (c *FakeMachine) MachineSets() internalversion.MachineSetInterface {
	return &FakeMachineSets{c}
}

func (c *FakeMachine) MachineTemplates(namespace string) internalversion.MachineTemplateInterface {
	return &FakeMachineTemplates{c, namespace}
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
