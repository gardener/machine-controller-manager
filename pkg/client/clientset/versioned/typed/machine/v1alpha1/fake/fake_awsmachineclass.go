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

// FakeAWSMachineClasses implements AWSMachineClassInterface
type FakeAWSMachineClasses struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var awsmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "awsmachineclasses"}

var awsmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "AWSMachineClass"}

// Get takes name of the aWSMachineClass, and returns the corresponding aWSMachineClass object, and an error if there is any.
func (c *FakeAWSMachineClasses) Get(name string, options v1.GetOptions) (result *v1alpha1.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(awsmachineclassesResource, c.ns, name), &v1alpha1.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSMachineClass), err
}

// List takes label and field selectors, and returns the list of AWSMachineClasses that match those selectors.
func (c *FakeAWSMachineClasses) List(opts v1.ListOptions) (result *v1alpha1.AWSMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(awsmachineclassesResource, awsmachineclassesKind, c.ns, opts), &v1alpha1.AWSMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AWSMachineClassList{}
	for _, item := range obj.(*v1alpha1.AWSMachineClassList).Items {
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
func (c *FakeAWSMachineClasses) Create(aWSMachineClass *v1alpha1.AWSMachineClass) (result *v1alpha1.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(awsmachineclassesResource, c.ns, aWSMachineClass), &v1alpha1.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSMachineClass), err
}

// Update takes the representation of a aWSMachineClass and updates it. Returns the server's representation of the aWSMachineClass, and an error, if there is any.
func (c *FakeAWSMachineClasses) Update(aWSMachineClass *v1alpha1.AWSMachineClass) (result *v1alpha1.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(awsmachineclassesResource, c.ns, aWSMachineClass), &v1alpha1.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSMachineClass), err
}

// Delete takes name of the aWSMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeAWSMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(awsmachineclassesResource, c.ns, name), &v1alpha1.AWSMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAWSMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(awsmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.AWSMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched aWSMachineClass.
func (c *FakeAWSMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.AWSMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(awsmachineclassesResource, c.ns, name, data, subresources...), &v1alpha1.AWSMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSMachineClass), err
}
