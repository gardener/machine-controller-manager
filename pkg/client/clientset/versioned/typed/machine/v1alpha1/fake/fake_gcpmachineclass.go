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

// FakeGCPMachineClasses implements GCPMachineClassInterface
type FakeGCPMachineClasses struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var gcpmachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "gcpmachineclasses"}

var gcpmachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "GCPMachineClass"}

// Get takes name of the gCPMachineClass, and returns the corresponding gCPMachineClass object, and an error if there is any.
func (c *FakeGCPMachineClasses) Get(name string, options v1.GetOptions) (result *v1alpha1.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(gcpmachineclassesResource, c.ns, name), &v1alpha1.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GCPMachineClass), err
}

// List takes label and field selectors, and returns the list of GCPMachineClasses that match those selectors.
func (c *FakeGCPMachineClasses) List(opts v1.ListOptions) (result *v1alpha1.GCPMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(gcpmachineclassesResource, gcpmachineclassesKind, c.ns, opts), &v1alpha1.GCPMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.GCPMachineClassList{}
	for _, item := range obj.(*v1alpha1.GCPMachineClassList).Items {
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
func (c *FakeGCPMachineClasses) Create(gCPMachineClass *v1alpha1.GCPMachineClass) (result *v1alpha1.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(gcpmachineclassesResource, c.ns, gCPMachineClass), &v1alpha1.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GCPMachineClass), err
}

// Update takes the representation of a gCPMachineClass and updates it. Returns the server's representation of the gCPMachineClass, and an error, if there is any.
func (c *FakeGCPMachineClasses) Update(gCPMachineClass *v1alpha1.GCPMachineClass) (result *v1alpha1.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(gcpmachineclassesResource, c.ns, gCPMachineClass), &v1alpha1.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GCPMachineClass), err
}

// Delete takes name of the gCPMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeGCPMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(gcpmachineclassesResource, c.ns, name), &v1alpha1.GCPMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGCPMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(gcpmachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.GCPMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched gCPMachineClass.
func (c *FakeGCPMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.GCPMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(gcpmachineclassesResource, c.ns, name, data, subresources...), &v1alpha1.GCPMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GCPMachineClass), err
}
