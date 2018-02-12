package fake

import (
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeOpenStackMachineClasses implements OpenStackMachineClassInterface
type FakeOpenStackMachineClasses struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var openstackmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "openstackmachineclasses"}

var openstackmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "OpenStackMachineClass"}

// Get takes name of the openStackMachineClass, and returns the corresponding openStackMachineClass object, and an error if there is any.
func (c *FakeOpenStackMachineClasses) Get(name string, options v1.GetOptions) (result *v1alpha1.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(openstackmachineclassesResource, c.ns, name), &v1alpha1.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenStackMachineClass), err
}

// List takes label and field selectors, and returns the list of OpenStackMachineClasses that match those selectors.
func (c *FakeOpenStackMachineClasses) List(opts v1.ListOptions) (result *v1alpha1.OpenStackMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(openstackmachineclassesResource, openstackmachineclassesKind, c.ns, opts), &v1alpha1.OpenStackMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.OpenStackMachineClassList{}
	for _, item := range obj.(*v1alpha1.OpenStackMachineClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested openStackMachineClasses.
func (c *FakeOpenStackMachineClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(openstackmachineclassesResource, c.ns, opts))

}

// Create takes the representation of a openStackMachineClass and creates it.  Returns the server's representation of the openStackMachineClass, and an error, if there is any.
func (c *FakeOpenStackMachineClasses) Create(openStackMachineClass *v1alpha1.OpenStackMachineClass) (result *v1alpha1.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(openstackmachineclassesResource, c.ns, openStackMachineClass), &v1alpha1.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenStackMachineClass), err
}

// Update takes the representation of a openStackMachineClass and updates it. Returns the server's representation of the openStackMachineClass, and an error, if there is any.
func (c *FakeOpenStackMachineClasses) Update(openStackMachineClass *v1alpha1.OpenStackMachineClass) (result *v1alpha1.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(openstackmachineclassesResource, c.ns, openStackMachineClass), &v1alpha1.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenStackMachineClass), err
}

// Delete takes name of the openStackMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeOpenStackMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(openstackmachineclassesResource, c.ns, name), &v1alpha1.OpenStackMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOpenStackMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(openstackmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.OpenStackMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched openStackMachineClass.
func (c *FakeOpenStackMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(openstackmachineclassesResource, c.ns, name, data, subresources...), &v1alpha1.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenStackMachineClass), err
}
