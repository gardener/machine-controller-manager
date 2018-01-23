package internalversion

import (
	"github.com/gardener/node-controller-manager/pkg/client/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type MachineInterface interface {
	RESTClient() rest.Interface
	AWSMachineClassesGetter
	AzureMachineClassesGetter
	MachinesGetter
	MachineDeploymentsGetter
	MachineSetsGetter
	MachineTemplatesGetter
	ScalesGetter
}

// MachineClient is used to interact with features provided by the machine.sapcloud.io group.
type MachineClient struct {
	restClient rest.Interface
}

func (c *MachineClient) AWSMachineClasses() AWSMachineClassInterface {
	return newAWSMachineClasses(c)
}

func (c *MachineClient) AzureMachineClasses() AzureMachineClassInterface {
	return newAzureMachineClasses(c)
}

func (c *MachineClient) Machines() MachineInterface {
	return newMachines(c)
}

func (c *MachineClient) MachineDeployments() MachineDeploymentInterface {
	return newMachineDeployments(c)
}

func (c *MachineClient) MachineSets() MachineSetInterface {
	return newMachineSets(c)
}

func (c *MachineClient) MachineTemplates(namespace string) MachineTemplateInterface {
	return newMachineTemplates(c, namespace)
}

func (c *MachineClient) Scales(namespace string) ScaleInterface {
	return newScales(c, namespace)
}

// NewForConfig creates a new MachineClient for the given config.
func NewForConfig(c *rest.Config) (*MachineClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &MachineClient{client}, nil
}

// NewForConfigOrDie creates a new MachineClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *MachineClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new MachineClient for the given RESTClient.
func New(c rest.Interface) *MachineClient {
	return &MachineClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("machine.sapcloud.io")
	if err != nil {
		return err
	}

	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != g.GroupVersion.Group {
		gv := g.GroupVersion
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *MachineClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
