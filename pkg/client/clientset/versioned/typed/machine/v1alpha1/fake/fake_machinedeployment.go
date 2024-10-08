// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMachineDeployments implements MachineDeploymentInterface
type FakeMachineDeployments struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var machinedeploymentsResource = v1alpha1.SchemeGroupVersion.WithResource("machinedeployments")

var machinedeploymentsKind = v1alpha1.SchemeGroupVersion.WithKind("MachineDeployment")

// Get takes name of the machineDeployment, and returns the corresponding machineDeployment object, and an error if there is any.
func (c *FakeMachineDeployments) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.MachineDeployment, err error) {
	emptyResult := &v1alpha1.MachineDeployment{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(machinedeploymentsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.MachineDeployment), err
}

// List takes label and field selectors, and returns the list of MachineDeployments that match those selectors.
func (c *FakeMachineDeployments) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.MachineDeploymentList, err error) {
	emptyResult := &v1alpha1.MachineDeploymentList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(machinedeploymentsResource, machinedeploymentsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MachineDeploymentList{ListMeta: obj.(*v1alpha1.MachineDeploymentList).ListMeta}
	for _, item := range obj.(*v1alpha1.MachineDeploymentList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested machineDeployments.
func (c *FakeMachineDeployments) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(machinedeploymentsResource, c.ns, opts))

}

// Create takes the representation of a machineDeployment and creates it.  Returns the server's representation of the machineDeployment, and an error, if there is any.
func (c *FakeMachineDeployments) Create(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment, opts v1.CreateOptions) (result *v1alpha1.MachineDeployment, err error) {
	emptyResult := &v1alpha1.MachineDeployment{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(machinedeploymentsResource, c.ns, machineDeployment, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.MachineDeployment), err
}

// Update takes the representation of a machineDeployment and updates it. Returns the server's representation of the machineDeployment, and an error, if there is any.
func (c *FakeMachineDeployments) Update(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment, opts v1.UpdateOptions) (result *v1alpha1.MachineDeployment, err error) {
	emptyResult := &v1alpha1.MachineDeployment{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(machinedeploymentsResource, c.ns, machineDeployment, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.MachineDeployment), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMachineDeployments) UpdateStatus(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment, opts v1.UpdateOptions) (result *v1alpha1.MachineDeployment, err error) {
	emptyResult := &v1alpha1.MachineDeployment{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(machinedeploymentsResource, "status", c.ns, machineDeployment, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.MachineDeployment), err
}

// Delete takes name of the machineDeployment and deletes it. Returns an error if one occurs.
func (c *FakeMachineDeployments) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(machinedeploymentsResource, c.ns, name, opts), &v1alpha1.MachineDeployment{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMachineDeployments) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(machinedeploymentsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.MachineDeploymentList{})
	return err
}

// Patch applies the patch and returns the patched machineDeployment.
func (c *FakeMachineDeployments) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.MachineDeployment, err error) {
	emptyResult := &v1alpha1.MachineDeployment{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(machinedeploymentsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.MachineDeployment), err
}

// UpdateScale takes the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if there is any.
func (c *FakeMachineDeployments) UpdateScale(ctx context.Context, machineDeploymentName string, scale *autoscalingv1.Scale, opts v1.UpdateOptions) (result *autoscalingv1.Scale, err error) {
	emptyResult := &autoscalingv1.Scale{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(machinedeploymentsResource, "scale", c.ns, scale, opts), &autoscalingv1.Scale{})

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*autoscalingv1.Scale), err
}
