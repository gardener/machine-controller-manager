/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog/v2"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

// OpenStackMachineClassKind is used to identify the machineClassKind as OpenStack
const OpenStackMachineClassKind = "OpenStackMachineClass"

func (c *controller) machineDeploymentToOpenStackMachineClassDelete(obj interface{}) {
	machineDeployment, ok := obj.(*v1alpha1.MachineDeployment)
	if machineDeployment == nil || !ok {
		return
	}
	if machineDeployment.Spec.Template.Spec.Class.Kind == OpenStackMachineClassKind {
		c.openStackMachineClassQueue.Add(machineDeployment.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineSetToOpenStackMachineClassDelete(obj interface{}) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)
	if machineSet == nil || !ok {
		return
	}
	if machineSet.Spec.Template.Spec.Class.Kind == OpenStackMachineClassKind {
		c.openStackMachineClassQueue.Add(machineSet.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineToOpenStackMachineClassAdd(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		klog.Warningf("Couldn't get machine from object: %+v", obj)
		return
	}
	if machine.Spec.Class.Kind == OpenStackMachineClassKind {
		c.openStackMachineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) machineToOpenStackMachineClassUpdate(oldObj, newObj interface{}) {
	oldMachine, ok := oldObj.(*v1alpha1.Machine)
	if oldMachine == nil || !ok {
		klog.Warningf("Couldn't get machine from object: %+v", oldObj)
		return
	}
	newMachine, ok := newObj.(*v1alpha1.Machine)
	if newMachine == nil || !ok {
		klog.Warningf("Couldn't get machine from object: %+v", newObj)
		return
	}

	if oldMachine.Spec.Class.Kind == newMachine.Spec.Class.Kind {
		if newMachine.Spec.Class.Kind == OpenStackMachineClassKind {
			// Both old and new machine refer to the same machineClass object
			// And the correct kind so enqueuing only one of them.
			c.openStackMachineClassQueue.Add(newMachine.Spec.Class.Name)
		}
	} else {
		// If both are pointing to different machineClasses
		// we might have to enqueue both.
		if oldMachine.Spec.Class.Kind == OpenStackMachineClassKind {
			c.openStackMachineClassQueue.Add(oldMachine.Spec.Class.Name)
		}
		if newMachine.Spec.Class.Kind == OpenStackMachineClassKind {
			c.openStackMachineClassQueue.Add(newMachine.Spec.Class.Name)
		}
	}
}

func (c *controller) machineToOpenStackMachineClassDelete(obj interface{}) {
	c.machineToOpenStackMachineClassAdd(obj)
}

func (c *controller) openStackMachineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.openStackMachineClassQueue.Add(key)
}

func (c *controller) openStackMachineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.OpenStackMachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := newObj.(*v1alpha1.OpenStackMachineClass)
	if new == nil || !ok {
		return
	}

	c.openStackMachineClassAdd(newObj)
}

func (c *controller) openStackMachineClassDelete(obj interface{}) {
	c.openStackMachineClassAdd(obj)
}

// reconcileClusterOpenStackMachineClassKey reconciles an OpenStackMachineClass due to controller resync
// or an event on the openStackMachineClass.
func (c *controller) reconcileClusterOpenStackMachineClassKey(key string) error {
	ctx := context.Background()
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.openStackMachineClassLister.OpenStackMachineClasses(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("%s %q: Not doing work because it has been deleted", OpenStackMachineClassKind, key)
		return nil
	}
	if err != nil {
		klog.V(4).Infof("%s %q: Unable to retrieve object from store: %v", OpenStackMachineClassKind, key, err)
		return err
	}

	err = c.reconcileClusterOpenStackMachineClass(ctx, class)
	if err != nil {
		c.enqueueOpenStackMachineClassAfter(class, 10*time.Second)
	} else {
		// Re-enqueue periodically to avoid missing of events
		// TODO: Infuture to get ride of this logic
		c.enqueueOpenStackMachineClassAfter(class, 10*time.Minute)
	}

	return nil
}

func (c *controller) reconcileClusterOpenStackMachineClass(ctx context.Context, class *v1alpha1.OpenStackMachineClass) error {
	klog.V(4).Info("Start Reconciling OpenStackmachineclass: ", class.Name)
	defer klog.V(4).Info("Stop Reconciling OpenStackmachineclass: ", class.Name)

	internalClass := &machine.OpenStackMachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}

	validationerr := validation.ValidateOpenStackMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		klog.Errorf("Validation of %s failed %s", OpenStackMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	// Add finalizer to avoid losing machineClass object
	if class.DeletionTimestamp == nil {
		err := c.addOpenStackMachineClassFinalizers(ctx, class)
		if err != nil {
			return err
		}
	}

	machines, err := c.findMachinesForClass(OpenStackMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp == nil {
		// If deletion timestamp doesn't exist
		_, annotationPresent := class.Annotations[machineutils.MigratedMachineClass]

		if c.deleteMigratedMachineClass && annotationPresent && len(machines) == 0 {
			// If controller has deleteMigratedMachineClass flag set
			// and the migratedMachineClass annotation is set
			err = c.controlMachineClient.OpenStackMachineClasses(class.Namespace).Delete(ctx, class.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			return fmt.Errorf("Retry deletion as deletion timestamp is now set")
		}

		return nil
	}

	if len(machines) > 0 {
		// machines are still referring the machine class, please wait before deletion
		klog.V(3).Infof("Cannot remove finalizer on %s because still (%d) machines are referencing it", class.Name, len(machines))

		for _, machine := range machines {
			c.addMachine(machine)
		}

		return fmt.Errorf("Retry as machine objects are still referring the machineclass")
	}

	return c.deleteOpenStackMachineClassFinalizers(ctx, class)
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addOpenStackMachineClassFinalizers(ctx context.Context, class *v1alpha1.OpenStackMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updateOpenStackMachineClassFinalizers(ctx, clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteOpenStackMachineClassFinalizers(ctx context.Context, class *v1alpha1.OpenStackMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		return c.updateOpenStackMachineClassFinalizers(ctx, clone, finalizers.List())
	}
	return nil
}

func (c *controller) updateOpenStackMachineClassFinalizers(ctx context.Context, class *v1alpha1.OpenStackMachineClass, finalizers []string) error {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.OpenStackMachineClasses(class.Namespace).Get(ctx, class.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.OpenStackMachineClasses(class.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Warning("Updating OpenStackMachineClass failed, retrying. ", class.Name, err)
		return err
	}
	klog.V(3).Infof("Successfully added/removed finalizer on the openstackmachineclass %q", class.Name)
	return err
}

func (c *controller) enqueueOpenStackMachineClassAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.openStackMachineClassQueue.AddAfter(key, after)
}
