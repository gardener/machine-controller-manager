// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMachines implements MachineInterface
type FakeMachines struct {
	Fake *FakeMachineV1alpha1
	ns   string
}

var machinesResource = v1alpha1.SchemeGroupVersion.WithResource("machines")

var machinesKind = v1alpha1.SchemeGroupVersion.WithKind("Machine")

// Get takes name of the machine, and returns the corresponding machine object, and an error if there is any.
func (c *FakeMachines) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Machine, err error) {
	emptyResult := &v1alpha1.Machine{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(machinesResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.Machine), err
}

// List takes label and field selectors, and returns the list of Machines that match those selectors.
func (c *FakeMachines) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.MachineList, err error) {
	emptyResult := &v1alpha1.MachineList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(machinesResource, machinesKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MachineList{ListMeta: obj.(*v1alpha1.MachineList).ListMeta}
	for _, item := range obj.(*v1alpha1.MachineList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested machines.
func (c *FakeMachines) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(machinesResource, c.ns, opts))

}

// Create takes the representation of a machine and creates it.  Returns the server's representation of the machine, and an error, if there is any.
func (c *FakeMachines) Create(ctx context.Context, machine *v1alpha1.Machine, opts v1.CreateOptions) (result *v1alpha1.Machine, err error) {
	emptyResult := &v1alpha1.Machine{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(machinesResource, c.ns, machine, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.Machine), err
}

// Update takes the representation of a machine and updates it. Returns the server's representation of the machine, and an error, if there is any.
func (c *FakeMachines) Update(ctx context.Context, machine *v1alpha1.Machine, opts v1.UpdateOptions) (result *v1alpha1.Machine, err error) {
	emptyResult := &v1alpha1.Machine{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(machinesResource, c.ns, machine, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.Machine), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMachines) UpdateStatus(ctx context.Context, machine *v1alpha1.Machine, opts v1.UpdateOptions) (result *v1alpha1.Machine, err error) {
	emptyResult := &v1alpha1.Machine{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(machinesResource, "status", c.ns, machine, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.Machine), err
}

// Delete takes name of the machine and deletes it. Returns an error if one occurs.
func (c *FakeMachines) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(machinesResource, c.ns, name, opts), &v1alpha1.Machine{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMachines) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(machinesResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.MachineList{})
	return err
}

// Patch applies the patch and returns the patched machine.
func (c *FakeMachines) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Machine, err error) {
	emptyResult := &v1alpha1.Machine{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(machinesResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.Machine), err
}
