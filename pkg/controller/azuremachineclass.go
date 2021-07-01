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

// AzureMachineClassKind is used to identify the machineClassKind as Azure
const AzureMachineClassKind = "AzureMachineClass"

func (c *controller) machineDeploymentToAzureMachineClassDelete(obj interface{}) {
	machineDeployment, ok := obj.(*v1alpha1.MachineDeployment)
	if machineDeployment == nil || !ok {
		return
	}
	if machineDeployment.Spec.Template.Spec.Class.Kind == AzureMachineClassKind {
		c.azureMachineClassQueue.Add(machineDeployment.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineSetToAzureMachineClassDelete(obj interface{}) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)
	if machineSet == nil || !ok {
		return
	}
	if machineSet.Spec.Template.Spec.Class.Kind == AzureMachineClassKind {
		c.azureMachineClassQueue.Add(machineSet.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineToAzureMachineClassAdd(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		klog.Warningf("Couldn't get machine from object: %+v", obj)
		return
	}
	if machine.Spec.Class.Kind == AzureMachineClassKind {
		c.azureMachineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) machineToAzureMachineClassUpdate(oldObj, newObj interface{}) {
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
		if newMachine.Spec.Class.Kind == AzureMachineClassKind {
			// Both old and new machine refer to the same machineClass object
			// And the correct kind so enqueuing only one of them.
			c.azureMachineClassQueue.Add(newMachine.Spec.Class.Name)
		}
	} else {
		// If both are pointing to different machineClasses
		// we might have to enqueue both.
		if oldMachine.Spec.Class.Kind == AzureMachineClassKind {
			c.azureMachineClassQueue.Add(oldMachine.Spec.Class.Name)
		}
		if newMachine.Spec.Class.Kind == AzureMachineClassKind {
			c.azureMachineClassQueue.Add(newMachine.Spec.Class.Name)
		}
	}
}

func (c *controller) machineToAzureMachineClassDelete(obj interface{}) {
	c.machineToAzureMachineClassAdd(obj)
}

func (c *controller) azureMachineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.azureMachineClassQueue.Add(key)
}

func (c *controller) azureMachineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.AzureMachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := newObj.(*v1alpha1.AzureMachineClass)
	if new == nil || !ok {
		return
	}

	c.azureMachineClassAdd(newObj)
}

func (c *controller) azureMachineClassDelete(obj interface{}) {
	c.azureMachineClassAdd(obj)
}

// reconcileClusterAzureMachineClassKey reconciles an AzureMachineClass due to controller resync
// or an event on the azureMachineClass.
func (c *controller) reconcileClusterAzureMachineClassKey(key string) error {
	ctx := context.Background()
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.azureMachineClassLister.AzureMachineClasses(c.namespace).Get(name)

	if errors.IsNotFound(err) {
		klog.Infof("%s %q: Not doing work because it has been deleted", AzureMachineClassKind, key)
		return nil
	}
	if err != nil {
		klog.Infof("%s %q: Unable to retrieve object from store: %v", AzureMachineClassKind, key, err)
		return err
	}

	err = c.reconcileClusterAzureMachineClass(ctx, class)
	if err != nil {
		c.enqueueAzureMachineClassAfter(class, 10*time.Second)
	} else {
		// Re-enqueue periodically to avoid missing of events
		// TODO: Infuture to get ride of this logic
		c.enqueueAzureMachineClassAfter(class, 10*time.Minute)
	}

	return nil
}

func (c *controller) reconcileClusterAzureMachineClass(ctx context.Context, class *v1alpha1.AzureMachineClass) error {
	klog.V(4).Info("Start Reconciling Azuremachineclass: ", class.Name)
	defer klog.V(4).Info("Stop Reconciling Azuremachineclass: ", class.Name)

	internalClass := &machine.AzureMachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}

	validationerr := validation.ValidateAzureMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		klog.Errorf("Validation of %s failed %s", AzureMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	// Add finalizer to avoid losing machineClass object
	if class.DeletionTimestamp == nil {
		err = c.addAzureMachineClassFinalizers(ctx, class)
		if err != nil {
			return err
		}
	}

	machines, err := c.findMachinesForClass(AzureMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp == nil {
		// If deletion timestamp doesn't exist
		_, annotationPresent := class.Annotations[machineutils.MigratedMachineClass]

		if c.deleteMigratedMachineClass && annotationPresent && len(machines) == 0 {
			// If controller has deleteMigratedMachineClass flag set
			// and the migratedMachineClass annotation is set
			err = c.controlMachineClient.AzureMachineClasses(class.Namespace).Delete(ctx, class.Name, metav1.DeleteOptions{})
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

	return c.deleteAzureMachineClassFinalizers(ctx, class)
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addAzureMachineClassFinalizers(ctx context.Context, class *v1alpha1.AzureMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updateAzureMachineClassFinalizers(ctx, clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteAzureMachineClassFinalizers(ctx context.Context, class *v1alpha1.AzureMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		return c.updateAzureMachineClassFinalizers(ctx, clone, finalizers.List())
	}
	return nil
}

func (c *controller) updateAzureMachineClassFinalizers(ctx context.Context, class *v1alpha1.AzureMachineClass, finalizers []string) error {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.AzureMachineClasses(class.Namespace).Get(ctx, class.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.AzureMachineClasses(class.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Warning("Updating AzureMachineClass failed, retrying. ", class.Name, err)
		return err
	}
	klog.V(3).Infof("Successfully added/removed finalizer on the azuremachineclass %q", class.Name)
	return err
}

func (c *controller) enqueueAzureMachineClassAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.azureMachineClassQueue.AddAfter(key, after)
}
