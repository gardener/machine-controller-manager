// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	scheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// MachineClassesGetter has a method to return a MachineClassInterface.
// A group's client should implement this interface.
type MachineClassesGetter interface {
	MachineClasses(namespace string) MachineClassInterface
}

// MachineClassInterface has methods to work with MachineClass resources.
type MachineClassInterface interface {
	Create(ctx context.Context, machineClass *v1alpha1.MachineClass, opts v1.CreateOptions) (*v1alpha1.MachineClass, error)
	Update(ctx context.Context, machineClass *v1alpha1.MachineClass, opts v1.UpdateOptions) (*v1alpha1.MachineClass, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.MachineClass, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.MachineClassList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.MachineClass, err error)
	MachineClassExpansion
}

// machineClasses implements MachineClassInterface
type machineClasses struct {
	*gentype.ClientWithList[*v1alpha1.MachineClass, *v1alpha1.MachineClassList]
}

// newMachineClasses returns a MachineClasses
func newMachineClasses(c *MachineV1alpha1Client, namespace string) *machineClasses {
	return &machineClasses{
		gentype.NewClientWithList[*v1alpha1.MachineClass, *v1alpha1.MachineClassList](
			"machineclasses",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *v1alpha1.MachineClass { return &v1alpha1.MachineClass{} },
			func() *v1alpha1.MachineClassList { return &v1alpha1.MachineClassList{} }),
	}
}
