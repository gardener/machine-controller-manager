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

// FakeInstanceSets implements InstanceSetInterface
type FakeInstanceSets struct {
	Fake *FakeNode
}

var instancesetsResource = schema.GroupVersionResource{Group: "node.sapcloud.io", Version: "", Resource: "instancesets"}

var instancesetsKind = schema.GroupVersionKind{Group: "node.sapcloud.io", Version: "", Kind: "InstanceSet"}

// Get takes name of the instanceSet, and returns the corresponding instanceSet object, and an error if there is any.
func (c *FakeInstanceSets) Get(name string, options v1.GetOptions) (result *node.InstanceSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(instancesetsResource, name), &node.InstanceSet{})
	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceSet), err
}

// List takes label and field selectors, and returns the list of InstanceSets that match those selectors.
func (c *FakeInstanceSets) List(opts v1.ListOptions) (result *node.InstanceSetList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(instancesetsResource, instancesetsKind, opts), &node.InstanceSetList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &node.InstanceSetList{}
	for _, item := range obj.(*node.InstanceSetList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested instanceSets.
func (c *FakeInstanceSets) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(instancesetsResource, opts))
}

// Create takes the representation of a instanceSet and creates it.  Returns the server's representation of the instanceSet, and an error, if there is any.
func (c *FakeInstanceSets) Create(instanceSet *node.InstanceSet) (result *node.InstanceSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(instancesetsResource, instanceSet), &node.InstanceSet{})
	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceSet), err
}

// Update takes the representation of a instanceSet and updates it. Returns the server's representation of the instanceSet, and an error, if there is any.
func (c *FakeInstanceSets) Update(instanceSet *node.InstanceSet) (result *node.InstanceSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(instancesetsResource, instanceSet), &node.InstanceSet{})
	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceSet), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeInstanceSets) UpdateStatus(instanceSet *node.InstanceSet) (*node.InstanceSet, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(instancesetsResource, "status", instanceSet), &node.InstanceSet{})
	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceSet), err
}

// Delete takes name of the instanceSet and deletes it. Returns an error if one occurs.
func (c *FakeInstanceSets) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(instancesetsResource, name), &node.InstanceSet{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeInstanceSets) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(instancesetsResource, listOptions)

	_, err := c.Fake.Invokes(action, &node.InstanceSetList{})
	return err
}

// Patch applies the patch and returns the patched instanceSet.
func (c *FakeInstanceSets) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *node.InstanceSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(instancesetsResource, name, data, subresources...), &node.InstanceSet{})
	if obj == nil {
		return nil, err
	}
	return obj.(*node.InstanceSet), err
}
