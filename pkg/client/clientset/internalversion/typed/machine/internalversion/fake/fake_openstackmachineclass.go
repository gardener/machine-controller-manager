package fake

import (
	machine "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeOpenStackMachineClasses implements OpenStackMachineClassInterface
type FakeOpenStackMachineClasses struct {
	Fake *FakeMachine
	ns   string
}

var openstackmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "", Resource: "openstackmachineclasses"}

var openstackmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "", Kind: "OpenStackMachineClass"}

// Get takes name of the openStackMachineClass, and returns the corresponding openStackMachineClass object, and an error if there is any.
func (c *FakeOpenStackMachineClasses) Get(name string, options v1.GetOptions) (result *machine.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(openstackmachineclassesResource, c.ns, name), &machine.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.OpenStackMachineClass), err
}

// List takes label and field selectors, and returns the list of OpenStackMachineClasses that match those selectors.
func (c *FakeOpenStackMachineClasses) List(opts v1.ListOptions) (result *machine.OpenStackMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(openstackmachineclassesResource, openstackmachineclassesKind, c.ns, opts), &machine.OpenStackMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &machine.OpenStackMachineClassList{}
	for _, item := range obj.(*machine.OpenStackMachineClassList).Items {
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
func (c *FakeOpenStackMachineClasses) Create(openStackMachineClass *machine.OpenStackMachineClass) (result *machine.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(openstackmachineclassesResource, c.ns, openStackMachineClass), &machine.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.OpenStackMachineClass), err
}

// Update takes the representation of a openStackMachineClass and updates it. Returns the server's representation of the openStackMachineClass, and an error, if there is any.
func (c *FakeOpenStackMachineClasses) Update(openStackMachineClass *machine.OpenStackMachineClass) (result *machine.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(openstackmachineclassesResource, c.ns, openStackMachineClass), &machine.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.OpenStackMachineClass), err
}

// Delete takes name of the openStackMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeOpenStackMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(openstackmachineclassesResource, c.ns, name), &machine.OpenStackMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOpenStackMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(openstackmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &machine.OpenStackMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched openStackMachineClass.
func (c *FakeOpenStackMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.OpenStackMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(openstackmachineclassesResource, c.ns, name, data, subresources...), &machine.OpenStackMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.OpenStackMachineClass), err
}
