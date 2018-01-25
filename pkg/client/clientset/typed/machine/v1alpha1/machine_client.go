package v1alpha1

import (
	v1alpha1 "github.com/gardener/node-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/node-controller-manager/pkg/client/clientset/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type MachineV1alpha1Interface interface {
	RESTClient() rest.Interface
	AWSMachineClassesGetter
	AzureMachineClassesGetter
	MachinesGetter
	MachineDeploymentsGetter
	MachineSetsGetter
	MachineTemplatesGetter
	ScalesGetter
}

// MachineV1alpha1Client is used to interact with features provided by the machine.sapcloud.io group.
type MachineV1alpha1Client struct {
	restClient rest.Interface
}

func (c *MachineV1alpha1Client) AWSMachineClasses() AWSMachineClassInterface {
	return newAWSMachineClasses(c)
}

func (c *MachineV1alpha1Client) AzureMachineClasses() AzureMachineClassInterface {
	return newAzureMachineClasses(c)
}

func (c *MachineV1alpha1Client) Machines() MachineInterface {
	return newMachines(c)
}

func (c *MachineV1alpha1Client) MachineDeployments() MachineDeploymentInterface {
	return newMachineDeployments(c)
}

func (c *MachineV1alpha1Client) MachineSets() MachineSetInterface {
	return newMachineSets(c)
}

func (c *MachineV1alpha1Client) MachineTemplates(namespace string) MachineTemplateInterface {
	return newMachineTemplates(c, namespace)
}

func (c *MachineV1alpha1Client) Scales(namespace string) ScaleInterface {
	return newScales(c, namespace)
}

// NewForConfig creates a new MachineV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*MachineV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &MachineV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new MachineV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *MachineV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new MachineV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *MachineV1alpha1Client {
	return &MachineV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *MachineV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
