package internalversion

import (
	machine "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	scheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/internalversion/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AzureMachineClassesGetter has a method to return a AzureMachineClassInterface.
// A group's client should implement this interface.
type AzureMachineClassesGetter interface {
	AzureMachineClasses(namespace string) AzureMachineClassInterface
}

// AzureMachineClassInterface has methods to work with AzureMachineClass resources.
type AzureMachineClassInterface interface {
	Create(*machine.AzureMachineClass) (*machine.AzureMachineClass, error)
	Update(*machine.AzureMachineClass) (*machine.AzureMachineClass, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*machine.AzureMachineClass, error)
	List(opts v1.ListOptions) (*machine.AzureMachineClassList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.AzureMachineClass, err error)
	AzureMachineClassExpansion
}

// azureMachineClasses implements AzureMachineClassInterface
type azureMachineClasses struct {
	client rest.Interface
	ns     string
}

// newAzureMachineClasses returns a AzureMachineClasses
func newAzureMachineClasses(c *MachineClient, namespace string) *azureMachineClasses {
	return &azureMachineClasses{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the azureMachineClass, and returns the corresponding azureMachineClass object, and an error if there is any.
func (c *azureMachineClasses) Get(name string, options v1.GetOptions) (result *machine.AzureMachineClass, err error) {
	result = &machine.AzureMachineClass{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("azuremachineclasses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AzureMachineClasses that match those selectors.
func (c *azureMachineClasses) List(opts v1.ListOptions) (result *machine.AzureMachineClassList, err error) {
	result = &machine.AzureMachineClassList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("azuremachineclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested azureMachineClasses.
func (c *azureMachineClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("azuremachineclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a azureMachineClass and creates it.  Returns the server's representation of the azureMachineClass, and an error, if there is any.
func (c *azureMachineClasses) Create(azureMachineClass *machine.AzureMachineClass) (result *machine.AzureMachineClass, err error) {
	result = &machine.AzureMachineClass{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("azuremachineclasses").
		Body(azureMachineClass).
		Do().
		Into(result)
	return
}

// Update takes the representation of a azureMachineClass and updates it. Returns the server's representation of the azureMachineClass, and an error, if there is any.
func (c *azureMachineClasses) Update(azureMachineClass *machine.AzureMachineClass) (result *machine.AzureMachineClass, err error) {
	result = &machine.AzureMachineClass{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("azuremachineclasses").
		Name(azureMachineClass.Name).
		Body(azureMachineClass).
		Do().
		Into(result)
	return
}

// Delete takes name of the azureMachineClass and deletes it. Returns an error if one occurs.
func (c *azureMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("azuremachineclasses").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *azureMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("azuremachineclasses").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched azureMachineClass.
func (c *azureMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.AzureMachineClass, err error) {
	result = &machine.AzureMachineClass{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("azuremachineclasses").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
