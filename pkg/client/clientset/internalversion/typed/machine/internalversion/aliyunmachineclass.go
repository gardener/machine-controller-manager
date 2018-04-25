package internalversion

import (
	machine "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	scheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/internalversion/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AliyunMachineClassesGetter has a method to return a AliyunMachineClassInterface.
// A group's client should implement this interface.
type AliyunMachineClassesGetter interface {
	AliyunMachineClasses(namespace string) AliyunMachineClassInterface
}

// AliyunMachineClassInterface has methods to work with AliyunMachineClass resources.
type AliyunMachineClassInterface interface {
	Create(*machine.AliyunMachineClass) (*machine.AliyunMachineClass, error)
	Update(*machine.AliyunMachineClass) (*machine.AliyunMachineClass, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*machine.AliyunMachineClass, error)
	List(opts v1.ListOptions) (*machine.AliyunMachineClassList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.AliyunMachineClass, err error)
	AliyunMachineClassExpansion
}

// aliyunMachineClasses implements AliyunMachineClassInterface
type aliyunMachineClasses struct {
	client rest.Interface
	ns     string
}

// newAliyunMachineClasses returns a AliyunMachineClasses
func newAliyunMachineClasses(c *MachineClient, namespace string) *aliyunMachineClasses {
	return &aliyunMachineClasses{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the aliyunMachineClass, and returns the corresponding aliyunMachineClass object, and an error if there is any.
func (c *aliyunMachineClasses) Get(name string, options v1.GetOptions) (result *machine.AliyunMachineClass, err error) {
	result = &machine.AliyunMachineClass{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AliyunMachineClasses that match those selectors.
func (c *aliyunMachineClasses) List(opts v1.ListOptions) (result *machine.AliyunMachineClassList, err error) {
	result = &machine.AliyunMachineClassList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested aliyunMachineClasses.
func (c *aliyunMachineClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a aliyunMachineClass and creates it.  Returns the server's representation of the aliyunMachineClass, and an error, if there is any.
func (c *aliyunMachineClasses) Create(aliyunMachineClass *machine.AliyunMachineClass) (result *machine.AliyunMachineClass, err error) {
	result = &machine.AliyunMachineClass{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		Body(aliyunMachineClass).
		Do().
		Into(result)
	return
}

// Update takes the representation of a aliyunMachineClass and updates it. Returns the server's representation of the aliyunMachineClass, and an error, if there is any.
func (c *aliyunMachineClasses) Update(aliyunMachineClass *machine.AliyunMachineClass) (result *machine.AliyunMachineClass, err error) {
	result = &machine.AliyunMachineClass{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		Name(aliyunMachineClass.Name).
		Body(aliyunMachineClass).
		Do().
		Into(result)
	return
}

// Delete takes name of the aliyunMachineClass and deletes it. Returns an error if one occurs.
func (c *aliyunMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *aliyunMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched aliyunMachineClass.
func (c *aliyunMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.AliyunMachineClass, err error) {
	result = &machine.AliyunMachineClass{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("aliyunmachineclasses").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
