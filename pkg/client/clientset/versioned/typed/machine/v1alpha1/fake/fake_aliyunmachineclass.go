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

// FakeAliyunMachineClasses implements AliyunMachineClassInterface
type FakeAliyunMachineClasses struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var aliyunmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "aliyunmachineclasses"}

var aliyunmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "AliyunMachineClass"}

// Get takes name of the aliyunMachineClass, and returns the corresponding aliyunMachineClass object, and an error if there is any.
func (c *FakeAliyunMachineClasses) Get(name string, options v1.GetOptions) (result *v1alpha1.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(aliyunmachineclassesResource, c.ns, name), &v1alpha1.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AliyunMachineClass), err
}

// List takes label and field selectors, and returns the list of AliyunMachineClasses that match those selectors.
func (c *FakeAliyunMachineClasses) List(opts v1.ListOptions) (result *v1alpha1.AliyunMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(aliyunmachineclassesResource, aliyunmachineclassesKind, c.ns, opts), &v1alpha1.AliyunMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AliyunMachineClassList{}
	for _, item := range obj.(*v1alpha1.AliyunMachineClassList).Items {
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
func (c *FakeAliyunMachineClasses) Create(aliyunMachineClass *v1alpha1.AliyunMachineClass) (result *v1alpha1.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(aliyunmachineclassesResource, c.ns, aliyunMachineClass), &v1alpha1.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AliyunMachineClass), err
}

// Update takes the representation of a aliyunMachineClass and updates it. Returns the server's representation of the aliyunMachineClass, and an error, if there is any.
func (c *FakeAliyunMachineClasses) Update(aliyunMachineClass *v1alpha1.AliyunMachineClass) (result *v1alpha1.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(aliyunmachineclassesResource, c.ns, aliyunMachineClass), &v1alpha1.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AliyunMachineClass), err
}

// Delete takes name of the aliyunMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeAliyunMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(aliyunmachineclassesResource, c.ns, name), &v1alpha1.AliyunMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAliyunMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(aliyunmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.AliyunMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched aliyunMachineClass.
func (c *FakeAliyunMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.AliyunMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(aliyunmachineclassesResource, c.ns, name, data, subresources...), &v1alpha1.AliyunMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AliyunMachineClass), err
}
