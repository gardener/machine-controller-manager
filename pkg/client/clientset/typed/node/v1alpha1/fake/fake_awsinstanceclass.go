package fake

import (
	v1alpha1 "github.com/gardener/node-controller-manager/pkg/apis/node/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeAWSInstanceClasses implements AWSInstanceClassInterface
type FakeAWSInstanceClasses struct {
	Fake *FakeNodeV1alpha1
}

var awsinstanceclassesResource = schema.GroupVersionResource{Group: "node.sapcloud.io", Version: "v1alpha1", Resource: "awsinstanceclasses"}

var awsinstanceclassesKind = schema.GroupVersionKind{Group: "node.sapcloud.io", Version: "v1alpha1", Kind: "AWSInstanceClass"}

// Get takes name of the aWSInstanceClass, and returns the corresponding aWSInstanceClass object, and an error if there is any.
func (c *FakeAWSInstanceClasses) Get(name string, options v1.GetOptions) (result *v1alpha1.AWSInstanceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(awsinstanceclassesResource, name), &v1alpha1.AWSInstanceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSInstanceClass), err
}

// List takes label and field selectors, and returns the list of AWSInstanceClasses that match those selectors.
func (c *FakeAWSInstanceClasses) List(opts v1.ListOptions) (result *v1alpha1.AWSInstanceClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(awsinstanceclassesResource, awsinstanceclassesKind, opts), &v1alpha1.AWSInstanceClassList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AWSInstanceClassList{}
	for _, item := range obj.(*v1alpha1.AWSInstanceClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested aWSInstanceClasses.
func (c *FakeAWSInstanceClasses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(awsinstanceclassesResource, opts))
}

// Create takes the representation of a aWSInstanceClass and creates it.  Returns the server's representation of the aWSInstanceClass, and an error, if there is any.
func (c *FakeAWSInstanceClasses) Create(aWSInstanceClass *v1alpha1.AWSInstanceClass) (result *v1alpha1.AWSInstanceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(awsinstanceclassesResource, aWSInstanceClass), &v1alpha1.AWSInstanceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSInstanceClass), err
}

// Update takes the representation of a aWSInstanceClass and updates it. Returns the server's representation of the aWSInstanceClass, and an error, if there is any.
func (c *FakeAWSInstanceClasses) Update(aWSInstanceClass *v1alpha1.AWSInstanceClass) (result *v1alpha1.AWSInstanceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(awsinstanceclassesResource, aWSInstanceClass), &v1alpha1.AWSInstanceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSInstanceClass), err
}

// Delete takes name of the aWSInstanceClass and deletes it. Returns an error if one occurs.
func (c *FakeAWSInstanceClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(awsinstanceclassesResource, name), &v1alpha1.AWSInstanceClass{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAWSInstanceClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(awsinstanceclassesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.AWSInstanceClassList{})
	return err
}

// Patch applies the patch and returns the patched aWSInstanceClass.
func (c *FakeAWSInstanceClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.AWSInstanceClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(awsinstanceclassesResource, name, data, subresources...), &v1alpha1.AWSInstanceClass{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AWSInstanceClass), err
}
