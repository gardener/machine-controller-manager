package fake

import (
	node "code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeInstanceTemplates implements InstanceTemplateInterface
type FakeInstanceTemplates struct {
	Fake *FakeNode
	ns   string
}

var instancetemplatesResource = schema.GroupVersionResource{Group: "node.sapcloud.io", Version: "", Resource: "instancetemplates"}

var instancetemplatesKind = schema.GroupVersionKind{Group: "node.sapcloud.io", Version: "", Kind: "InstanceTemplate"}

// Get takes name of the instanceTemplate, and returns the corresponding instanceTemplate object, and an error if there is any.
func (c *FakeInstanceTemplates) Get(name string, options v1.GetOptions) (result *node.InstanceTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(instancetemplatesResource, c.ns, name), &node.InstanceTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceTemplate), err
}

// List takes label and field selectors, and returns the list of InstanceTemplates that match those selectors.
func (c *FakeInstanceTemplates) List(opts v1.ListOptions) (result *node.InstanceTemplateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(instancetemplatesResource, instancetemplatesKind, c.ns, opts), &node.InstanceTemplateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &node.InstanceTemplateList{}
	for _, item := range obj.(*node.InstanceTemplateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested instanceTemplates.
func (c *FakeInstanceTemplates) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(instancetemplatesResource, c.ns, opts))

}

// Create takes the representation of a instanceTemplate and creates it.  Returns the server's representation of the instanceTemplate, and an error, if there is any.
func (c *FakeInstanceTemplates) Create(instanceTemplate *node.InstanceTemplate) (result *node.InstanceTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(instancetemplatesResource, c.ns, instanceTemplate), &node.InstanceTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceTemplate), err
}

// Update takes the representation of a instanceTemplate and updates it. Returns the server's representation of the instanceTemplate, and an error, if there is any.
func (c *FakeInstanceTemplates) Update(instanceTemplate *node.InstanceTemplate) (result *node.InstanceTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(instancetemplatesResource, c.ns, instanceTemplate), &node.InstanceTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceTemplate), err
}

// Delete takes name of the instanceTemplate and deletes it. Returns an error if one occurs.
func (c *FakeInstanceTemplates) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(instancetemplatesResource, c.ns, name), &node.InstanceTemplate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeInstanceTemplates) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(instancetemplatesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &node.InstanceTemplateList{})
	return err
}

// Patch applies the patch and returns the patched instanceTemplate.
func (c *FakeInstanceTemplates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *node.InstanceTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(instancetemplatesResource, c.ns, name, data, subresources...), &node.InstanceTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceTemplate), err
}
