package fake

import (
	v1alpha1 "code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeInstanceDeployments implements InstanceDeploymentInterface
type FakeInstanceDeployments struct {
	Fake *FakeNodeV1alpha1
}

var instancedeploymentsResource = schema.GroupVersionResource{Group: "node.sapcloud.io", Version: "v1alpha1", Resource: "instancedeployments"}

var instancedeploymentsKind = schema.GroupVersionKind{Group: "node.sapcloud.io", Version: "v1alpha1", Kind: "InstanceDeployment"}

// Get takes name of the instanceDeployment, and returns the corresponding instanceDeployment object, and an error if there is any.
func (c *FakeInstanceDeployments) Get(name string, options v1.GetOptions) (result *v1alpha1.InstanceDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(instancedeploymentsResource, name), &v1alpha1.InstanceDeployment{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InstanceDeployment), err
}

// List takes label and field selectors, and returns the list of InstanceDeployments that match those selectors.
func (c *FakeInstanceDeployments) List(opts v1.ListOptions) (result *v1alpha1.InstanceDeploymentList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(instancedeploymentsResource, instancedeploymentsKind, opts), &v1alpha1.InstanceDeploymentList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.InstanceDeploymentList{}
	for _, item := range obj.(*v1alpha1.InstanceDeploymentList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested instanceDeployments.
func (c *FakeInstanceDeployments) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(instancedeploymentsResource, opts))
}

// Create takes the representation of a instanceDeployment and creates it.  Returns the server's representation of the instanceDeployment, and an error, if there is any.
func (c *FakeInstanceDeployments) Create(instanceDeployment *v1alpha1.InstanceDeployment) (result *v1alpha1.InstanceDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(instancedeploymentsResource, instanceDeployment), &v1alpha1.InstanceDeployment{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InstanceDeployment), err
}

// Update takes the representation of a instanceDeployment and updates it. Returns the server's representation of the instanceDeployment, and an error, if there is any.
func (c *FakeInstanceDeployments) Update(instanceDeployment *v1alpha1.InstanceDeployment) (result *v1alpha1.InstanceDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(instancedeploymentsResource, instanceDeployment), &v1alpha1.InstanceDeployment{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InstanceDeployment), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeInstanceDeployments) UpdateStatus(instanceDeployment *v1alpha1.InstanceDeployment) (*v1alpha1.InstanceDeployment, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(instancedeploymentsResource, "status", instanceDeployment), &v1alpha1.InstanceDeployment{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InstanceDeployment), err
}

// Delete takes name of the instanceDeployment and deletes it. Returns an error if one occurs.
func (c *FakeInstanceDeployments) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(instancedeploymentsResource, name), &v1alpha1.InstanceDeployment{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeInstanceDeployments) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(instancedeploymentsResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.InstanceDeploymentList{})
	return err
}

// Patch applies the patch and returns the patched instanceDeployment.
func (c *FakeInstanceDeployments) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.InstanceDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(instancedeploymentsResource, name, data, subresources...), &v1alpha1.InstanceDeployment{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InstanceDeployment), err
}

// GetScale takes name of the instanceDeployment, and returns the corresponding scale object, and an error if there is any.
func (c *FakeInstanceDeployments) GetScale(instanceDeploymentName string, options v1.GetOptions) (result *v1alpha1.Scale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(instancedeploymentsResource, instanceDeploymentName), &v1alpha1.Scale{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Scale), err
}

// UpdateScale takes the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if there is any.
func (c *FakeInstanceDeployments) UpdateScale(instanceDeploymentName string, scale *v1alpha1.Scale) (result *v1alpha1.Scale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(instancedeploymentsResource, instanceDeployment), &v1alpha1.InstanceDeployment{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Scale), err
}
