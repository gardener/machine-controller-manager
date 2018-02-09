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

// FakeGCPMachineClasses implements GCPMachineClassInterface
type FakeGCPMachineClasses struct {
	Fake *FakeMachine
	ns   string
}

var gcpmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "", Resource: "gcpmachineclasses"}

var gcpmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "", Kind: "GCPMachineClass"}

// Get takes name of the gCPMachineClass, and returns the corresponding gCPMachineClass object, and an error if there is any.
func (c *FakeGCPMachineClasses) Get(name string, options v1.GetOptions) (result *machine.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(gcpmachineclassesResource, c.ns, name), &machine.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.GCPMachineClass), err
}

// List takes label and field selectors, and returns the list of GCPMachineClasses that match those selectors.
func (c *FakeGCPMachineClasses) List(opts v1.ListOptions) (result *machine.GCPMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(gcpmachineclassesResource, gcpmachineclassesKind, c.ns, opts), &machine.GCPMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &machine.GCPMachineClassList{}
	for _, item := range obj.(*machine.GCPMachineClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested gCPMachineClasses.
func (c *FakeGCPMachineClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(gcpmachineclassesResource, c.ns, opts))

}

// Create takes the representation of a gCPMachineClass and creates it.  Returns the server's representation of the gCPMachineClass, and an error, if there is any.
func (c *FakeGCPMachineClasses) Create(gCPMachineClass *machine.GCPMachineClass) (result *machine.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(gcpmachineclassesResource, c.ns, gCPMachineClass), &machine.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.GCPMachineClass), err
}

// Update takes the representation of a gCPMachineClass and updates it. Returns the server's representation of the gCPMachineClass, and an error, if there is any.
func (c *FakeGCPMachineClasses) Update(gCPMachineClass *machine.GCPMachineClass) (result *machine.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(gcpmachineclassesResource, c.ns, gCPMachineClass), &machine.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.GCPMachineClass), err
}

// Delete takes name of the gCPMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeGCPMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(gcpmachineclassesResource, c.ns, name), &machine.GCPMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGCPMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(gcpmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &machine.GCPMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched gCPMachineClass.
func (c *FakeGCPMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(gcpmachineclassesResource, c.ns, name, data, subresources...), &machine.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.GCPMachineClass), err
}
