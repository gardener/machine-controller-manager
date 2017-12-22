package internalversion

import (
	"github.com/gardener/node-controller-manager/pkg/client/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type NodeInterface interface {
	RESTClient() rest.Interface
	AWSInstanceClassesGetter
	InstancesGetter
	InstanceDeploymentsGetter
	InstanceSetsGetter
	InstanceTemplatesGetter
	ScalesGetter
}

// NodeClient is used to interact with features provided by the node.sapcloud.io group.
type NodeClient struct {
	restClient rest.Interface
}

func (c *NodeClient) AWSInstanceClasses() AWSInstanceClassInterface {
	return newAWSInstanceClasses(c)
}

func (c *NodeClient) Instances() InstanceInterface {
	return newInstances(c)
}

func (c *NodeClient) InstanceDeployments() InstanceDeploymentInterface {
	return newInstanceDeployments(c)
}

func (c *NodeClient) InstanceSets() InstanceSetInterface {
	return newInstanceSets(c)
}

func (c *NodeClient) InstanceTemplates(namespace string) InstanceTemplateInterface {
	return newInstanceTemplates(c, namespace)
}

func (c *NodeClient) Scales(namespace string) ScaleInterface {
	return newScales(c, namespace)
}

// NewForConfig creates a new NodeClient for the given config.
func NewForConfig(c *rest.Config) (*NodeClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &NodeClient{client}, nil
}

// NewForConfigOrDie creates a new NodeClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *NodeClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new NodeClient for the given RESTClient.
func New(c rest.Interface) *NodeClient {
	return &NodeClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("node.sapcloud.io")
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
func (c *NodeClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
