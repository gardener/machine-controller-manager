package internalversion

import (
	node "code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node"
	scheme "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AWSInstanceClassesGetter has a method to return a AWSInstanceClassInterface.
// A group's client should implement this interface.
type AWSInstanceClassesGetter interface {
	AWSInstanceClasses() AWSInstanceClassInterface
}

// AWSInstanceClassInterface has methods to work with AWSInstanceClass resources.
type AWSInstanceClassInterface interface {
	Create(*node.AWSInstanceClass) (*node.AWSInstanceClass, error)
	Update(*node.AWSInstanceClass) (*node.AWSInstanceClass, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*node.AWSInstanceClass, error)
	List(opts v1.ListOptions) (*node.AWSInstanceClassList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *node.AWSInstanceClass, err error)
	AWSInstanceClassExpansion
}

// aWSInstanceClasses implements AWSInstanceClassInterface
type aWSInstanceClasses struct {
	client rest.Interface
}

// newAWSInstanceClasses returns a AWSInstanceClasses
func newAWSInstanceClasses(c *NodeClient) *aWSInstanceClasses {
	return &aWSInstanceClasses{
		client: c.RESTClient(),
	}
}

// Get takes name of the aWSInstanceClass, and returns the corresponding aWSInstanceClass object, and an error if there is any.
func (c *aWSInstanceClasses) Get(name string, options v1.GetOptions) (result *node.AWSInstanceClass, err error) {
	result = &node.AWSInstanceClass{}
	err = c.client.Get().
		Resource("awsinstanceclasses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AWSInstanceClasses that match those selectors.
func (c *aWSInstanceClasses) List(opts v1.ListOptions) (result *node.AWSInstanceClassList, err error) {
	result = &node.AWSInstanceClassList{}
	err = c.client.Get().
		Resource("awsinstanceclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested aWSInstanceClasses.
func (c *aWSInstanceClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("awsinstanceclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a aWSInstanceClass and creates it.  Returns the server's representation of the aWSInstanceClass, and an error, if there is any.
func (c *aWSInstanceClasses) Create(aWSInstanceClass *node.AWSInstanceClass) (result *node.AWSInstanceClass, err error) {
	result = &node.AWSInstanceClass{}
	err = c.client.Post().
		Resource("awsinstanceclasses").
		Body(aWSInstanceClass).
		Do().
		Into(result)
	return
}

// Update takes the representation of a aWSInstanceClass and updates it. Returns the server's representation of the aWSInstanceClass, and an error, if there is any.
func (c *aWSInstanceClasses) Update(aWSInstanceClass *node.AWSInstanceClass) (result *node.AWSInstanceClass, err error) {
	result = &node.AWSInstanceClass{}
	err = c.client.Put().
		Resource("awsinstanceclasses").
		Name(aWSInstanceClass.Name).
		Body(aWSInstanceClass).
		Do().
		Into(result)
	return
}

// Delete takes name of the aWSInstanceClass and deletes it. Returns an error if one occurs.
func (c *aWSInstanceClasses) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("awsinstanceclasses").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *aWSInstanceClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("awsinstanceclasses").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched aWSInstanceClass.
func (c *aWSInstanceClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *node.AWSInstanceClass, err error) {
	result = &node.AWSInstanceClass{}
	err = c.client.Patch(pt).
		Resource("awsinstanceclasses").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
