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

// FakeMachineTemplates implements MachineTemplateInterface
type FakeMachineTemplates struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var machinetemplatesResource = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "machinetemplates"}

var machinetemplatesKind = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineTemplate"}

// Get takes name of the machineTemplate, and returns the corresponding machineTemplate object, and an error if there is any.
func (c *FakeMachineTemplates) Get(name string, options v1.GetOptions) (result *v1alpha1.MachineTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(machinetemplatesResource, c.ns, name), &v1alpha1.MachineTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MachineTemplate), err
}

// List takes label and field selectors, and returns the list of MachineTemplates that match those selectors.
func (c *FakeMachineTemplates) List(opts v1.ListOptions) (result *v1alpha1.MachineTemplateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(machinetemplatesResource, machinetemplatesKind, c.ns, opts), &v1alpha1.MachineTemplateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MachineTemplateList{}
	for _, item := range obj.(*v1alpha1.MachineTemplateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested machineTemplates.
func (c *FakeMachineTemplates) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(machinetemplatesResource, c.ns, opts))

}

// Create takes the representation of a machineTemplate and creates it.  Returns the server's representation of the machineTemplate, and an error, if there is any.
func (c *FakeMachineTemplates) Create(machineTemplate *v1alpha1.MachineTemplate) (result *v1alpha1.MachineTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(machinetemplatesResource, c.ns, machineTemplate), &v1alpha1.MachineTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MachineTemplate), err
}

// Update takes the representation of a machineTemplate and updates it. Returns the server's representation of the machineTemplate, and an error, if there is any.
func (c *FakeMachineTemplates) Update(machineTemplate *v1alpha1.MachineTemplate) (result *v1alpha1.MachineTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(machinetemplatesResource, c.ns, machineTemplate), &v1alpha1.MachineTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MachineTemplate), err
}

// Delete takes name of the machineTemplate and deletes it. Returns an error if one occurs.
func (c *FakeMachineTemplates) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(machinetemplatesResource, c.ns, name), &v1alpha1.MachineTemplate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMachineTemplates) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(machinetemplatesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.MachineTemplateList{})
	return err
}

// Patch applies the patch and returns the patched machineTemplate.
func (c *FakeMachineTemplates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MachineTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(machinetemplatesResource, c.ns, name, data, subresources...), &v1alpha1.MachineTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MachineTemplate), err
}
