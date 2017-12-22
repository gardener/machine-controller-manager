package internalversion

import (
	node "code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node"
	scheme "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// InstanceSetsGetter has a method to return a InstanceSetInterface.
// A group's client should implement this interface.
type InstanceSetsGetter interface {
	InstanceSets() InstanceSetInterface
}

// InstanceSetInterface has methods to work with InstanceSet resources.
type InstanceSetInterface interface {
	Create(*node.InstanceSet) (*node.InstanceSet, error)
	Update(*node.InstanceSet) (*node.InstanceSet, error)
	UpdateStatus(*node.InstanceSet) (*node.InstanceSet, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*node.InstanceSet, error)
	List(opts v1.ListOptions) (*node.InstanceSetList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *node.InstanceSet, err error)
	InstanceSetExpansion
}

// instanceSets implements InstanceSetInterface
type instanceSets struct {
	client rest.Interface
}

// newInstanceSets returns a InstanceSets
func newInstanceSets(c *NodeClient) *instanceSets {
	return &instanceSets{
		client: c.RESTClient(),
	}
}

// Get takes name of the instanceSet, and returns the corresponding instanceSet object, and an error if there is any.
func (c *instanceSets) Get(name string, options v1.GetOptions) (result *node.InstanceSet, err error) {
	result = &node.InstanceSet{}
	err = c.client.Get().
		Resource("instancesets").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of InstanceSets that match those selectors.
func (c *instanceSets) List(opts v1.ListOptions) (result *node.InstanceSetList, err error) {
	result = &node.InstanceSetList{}
	err = c.client.Get().
		Resource("instancesets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested instanceSets.
func (c *instanceSets) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("instancesets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a instanceSet and creates it.  Returns the server's representation of the instanceSet, and an error, if there is any.
func (c *instanceSets) Create(instanceSet *node.InstanceSet) (result *node.InstanceSet, err error) {
	result = &node.InstanceSet{}
	err = c.client.Post().
		Resource("instancesets").
		Body(instanceSet).
		Do().
		Into(result)
	return
}

// Update takes the representation of a instanceSet and updates it. Returns the server's representation of the instanceSet, and an error, if there is any.
func (c *instanceSets) Update(instanceSet *node.InstanceSet) (result *node.InstanceSet, err error) {
	result = &node.InstanceSet{}
	err = c.client.Put().
		Resource("instancesets").
		Name(instanceSet.Name).
		Body(instanceSet).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *instanceSets) UpdateStatus(instanceSet *node.InstanceSet) (result *node.InstanceSet, err error) {
	result = &node.InstanceSet{}
	err = c.client.Put().
		Resource("instancesets").
		Name(instanceSet.Name).
		SubResource("status").
		Body(instanceSet).
		Do().
		Into(result)
	return
}

// Delete takes name of the instanceSet and deletes it. Returns an error if one occurs.
func (c *instanceSets) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("instancesets").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *instanceSets) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("instancesets").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched instanceSet.
func (c *instanceSets) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *node.InstanceSet, err error) {
	result = &node.InstanceSet{}
	err = c.client.Patch(pt).
		Resource("instancesets").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
