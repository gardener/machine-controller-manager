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

// FakeAliyunMachineClasses implements AliyunMachineClassInterface
type FakeAliyunMachineClasses struct {
	Fake *FakeMachine
	ns   string
}

var aliyunmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "", Resource: "aliyunmachineclasses"}

var aliyunmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "", Kind: "AliyunMachineClass"}

// Get takes name of the aliyunMachineClass, and returns the corresponding aliyunMachineClass object, and an error if there is any.
func (c *FakeAliyunMachineClasses) Get(name string, options v1.GetOptions) (result *machine.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(aliyunmachineclassesResource, c.ns, name), &machine.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AliyunMachineClass), err
}

// List takes label and field selectors, and returns the list of AliyunMachineClasses that match those selectors.
func (c *FakeAliyunMachineClasses) List(opts v1.ListOptions) (result *machine.AliyunMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(aliyunmachineclassesResource, aliyunmachineclassesKind, c.ns, opts), &machine.AliyunMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &machine.AliyunMachineClassList{}
	for _, item := range obj.(*machine.AliyunMachineClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested aliyunMachineClasses.
func (c *FakeAliyunMachineClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(aliyunmachineclassesResource, c.ns, opts))

}

// Create takes the representation of a aliyunMachineClass and creates it.  Returns the server's representation of the aliyunMachineClass, and an error, if there is any.
func (c *FakeAliyunMachineClasses) Create(aliyunMachineClass *machine.AliyunMachineClass) (result *machine.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(aliyunmachineclassesResource, c.ns, aliyunMachineClass), &machine.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AliyunMachineClass), err
}

// Update takes the representation of a aliyunMachineClass and updates it. Returns the server's representation of the aliyunMachineClass, and an error, if there is any.
func (c *FakeAliyunMachineClasses) Update(aliyunMachineClass *machine.AliyunMachineClass) (result *machine.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(aliyunmachineclassesResource, c.ns, aliyunMachineClass), &machine.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AliyunMachineClass), err
}

// Delete takes name of the aliyunMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeAliyunMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(aliyunmachineclassesResource, c.ns, name), &machine.AliyunMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAliyunMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(aliyunmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &machine.AliyunMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched aliyunMachineClass.
func (c *FakeAliyunMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *machine.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(aliyunmachineclassesResource, c.ns, name, data, subresources...), &machine.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*machine.AliyunMachineClass), err
}
