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

// FakeAzureMachineClasses implements AzureMachineClassInterface
type FakeAzureMachineClasses struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var azuremachineclassesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "azuremachineclasses"}

var azuremachineclassesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "AzureMachineClass"}

// Get takes name of the azureMachineClass, and returns the corresponding azureMachineClass object, and an error if there is any.
func (c *FakeAzureMachineClasses) Get(name string, options v1.GetOptions) (result *v1alpha1.AzureMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(azuremachineclassesResource, c.ns, name), &v1alpha1.AzureMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureMachineClass), err
}

// List takes label and field selectors, and returns the list of AzureMachineClasses that match those selectors.
func (c *FakeAzureMachineClasses) List(opts v1.ListOptions) (result *v1alpha1.AzureMachineClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(azuremachineclassesResource, azuremachineclassesKind, c.ns, opts), &v1alpha1.AzureMachineClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AzureMachineClassList{}
	for _, item := range obj.(*v1alpha1.AzureMachineClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested azureMachineClasses.
func (c *FakeAzureMachineClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(azuremachineclassesResource, c.ns, opts))

}

// Create takes the representation of a azureMachineClass and creates it.  Returns the server's representation of the azureMachineClass, and an error, if there is any.
func (c *FakeAzureMachineClasses) Create(azureMachineClass *v1alpha1.AzureMachineClass) (result *v1alpha1.AzureMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(azuremachineclassesResource, c.ns, azureMachineClass), &v1alpha1.AzureMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureMachineClass), err
}

// Update takes the representation of a azureMachineClass and updates it. Returns the server's representation of the azureMachineClass, and an error, if there is any.
func (c *FakeAzureMachineClasses) Update(azureMachineClass *v1alpha1.AzureMachineClass) (result *v1alpha1.AzureMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(azuremachineclassesResource, c.ns, azureMachineClass), &v1alpha1.AzureMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureMachineClass), err
}

// Delete takes name of the azureMachineClass and deletes it. Returns an error if one occurs.
func (c *FakeAzureMachineClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(azuremachineclassesResource, c.ns, name), &v1alpha1.AzureMachineClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAzureMachineClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(azuremachineclassesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.AzureMachineClassList{})
	return err
}

// Patch applies the patch and returns the patched azureMachineClass.
func (c *FakeAzureMachineClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.AzureMachineClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(azuremachineclassesResource, c.ns, name, data, subresources...), &v1alpha1.AzureMachineClass{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AzureMachineClass), err
}
