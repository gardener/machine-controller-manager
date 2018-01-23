package fake

import (
	v1alpha1 "github.com/gardener/node-controller-manager/pkg/client/clientset/typed/machine/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeMachineV1alpha1 struct {
	*testing.Fake
}

func (c *FakeMachineV1alpha1) AWSMachineClasses() v1alpha1.AWSMachineClassInterface {
	return &FakeAWSMachineClasses{c}
}

func (c *FakeMachineV1alpha1) AzureMachineClasses() v1alpha1.AzureMachineClassInterface {
	return &FakeAzureMachineClasses{c}
}

func (c *FakeMachineV1alpha1) Machines() v1alpha1.MachineInterface {
	return &FakeMachines{c}
}

func (c *FakeMachineV1alpha1) MachineDeployments() v1alpha1.MachineDeploymentInterface {
	return &FakeMachineDeployments{c}
}

func (c *FakeMachineV1alpha1) MachineSets() v1alpha1.MachineSetInterface {
	return &FakeMachineSets{c}
}

func (c *FakeMachineV1alpha1) MachineTemplates(namespace string) v1alpha1.MachineTemplateInterface {
	return &FakeMachineTemplates{c, namespace}
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
