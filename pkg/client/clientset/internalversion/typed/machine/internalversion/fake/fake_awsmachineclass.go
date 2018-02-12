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

// FakeAWSMachineClasses implements AWSMachineClassInterface
type FakeAWSMachineClasses struct {
	Fake *FakeMachine
	ns   string
}

var awsmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "", Resource: "awsmachineclasses"}

var awsmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "", Kind: "AWSMachineClass"}

// Get takes name of the aWSMachineClass, and returns the corresponding aWSMachineClass object, and an error if there is any.
func (c *FakeAWSMachineClasses) Get(name string, options v1.GetOptions) (result *machine.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(awsmachineclassesResource, c.ns, name), &machine.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AWSMachineClass), err
}

// List takes label and field selectors, and returns the list of AWSMachineClasses that match those selectors.
func (c *FakeAWSMachineClasses) List(opts v1.ListOptions) (result *machine.AWSMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(awsmachineclassesResource, awsmachineclassesKind, c.ns, opts), &machine.AWSMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &machine.AWSMachineClassList{}
	for _, item := range obj.(*machine.AWSMachineClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested aWSMachineClasses.
func (c *FakeAWSMachineClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(awsmachineclassesResource, c.ns, opts))

}

// Create takes the representation of a aWSMachineClass and creates it.  Returns the server's representation of the aWSMachineClass, and an error, if there is any.
func (c *FakeAWSMachineClasses) Create(aWSMachineClass *machine.AWSMachineClass) (result *machine.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(awsmachineclassesResource, c.ns, aWSMachineClass), &machine.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AWSMachineClass), err
}

// Update takes the representation of a aWSMachineClass and updates it. Returns the server's representation of the aWSMachineClass, and an error, if there is any.
func (c *FakeAWSMachineClasses) Update(aWSMachineClass *machine.AWSMachineClass) (result *machine.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(awsmachineclassesResource, c.ns, aWSMachineClass), &machine.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AWSMachineClass), err
}

// Delete takes name of the aWSMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeAWSMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(awsmachineclassesResource, c.ns, name), &machine.AWSMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAWSMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(awsmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &machine.AWSMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched aWSMachineClass.
func (c *FakeAWSMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(awsmachineclassesResource, c.ns, name, data, subresources...), &machine.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AWSMachineClass), err
}
