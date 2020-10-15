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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

func (c *controller) machineToMachineClassAdd(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		klog.Warningf("Couldn't get machine from object: %+v", obj)
		return
	}
	if machine.Spec.Class.Kind == machineutils.MachineClassKind {
		c.machineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) machineToMachineClassUpdate(oldObj, newObj interface{}) {
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
		if newMachine.Spec.Class.Kind == machineutils.MachineClassKind {
			// Both old and new machine refer to the same machineClass object
			// And the correct kind so enqueuing only one of them.
			c.machineClassQueue.Add(newMachine.Spec.Class.Name)
		}
	} else {
		// If both are pointing to different machineClasses
		// we might have to enqueue both.
		if oldMachine.Spec.Class.Kind == machineutils.MachineClassKind {
			c.machineClassQueue.Add(oldMachine.Spec.Class.Name)
		}
		if newMachine.Spec.Class.Kind == machineutils.MachineClassKind {
			c.machineClassQueue.Add(newMachine.Spec.Class.Name)
		}
	}
}

func (c *controller) machineToMachineClassDelete(obj interface{}) {
	c.machineToMachineClassAdd(obj)
}

func (c *controller) machineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.machineClassQueue.Add(key)
}

func (c *controller) machineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.MachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := newObj.(*v1alpha1.MachineClass)
	if new == nil || !ok {
		return
	}

	c.machineClassAdd(newObj)
}

func (c *controller) machineClassDelete(obj interface{}) {
	c.machineClassAdd(obj)
}

// reconcileClusterMachineClassKey reconciles an machineClass due to controller resync
// or an event on the machineClass.
func (c *controller) reconcileClusterMachineClassKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.machineClassLister.MachineClasses(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.Infof("%s %q: Not doing work because it has been deleted", machineutils.MachineClassKind, key)
		return nil
	}
	if err != nil {
		klog.Infof("%s %q: Unable to retrieve object from store: %v", machineutils.MachineClassKind, key, err)
		return err
	}

	err = c.reconcileClusterMachineClass(class)
	if err != nil {
		// Re-enqueue after a 10s window
		c.enqueueMachineClassAfter(class, 10*time.Second)
	} else {
		// Re-enqueue periodically to avoid missing of events
		// TODO: Get ride of this logic
		c.enqueueMachineClassAfter(class, 10*time.Minute)
	}

	return nil
}

func (c *controller) reconcileClusterMachineClass(class *v1alpha1.MachineClass) error {
	klog.V(4).Info("Start Reconciling machineclass: ", class.Name)
	defer klog.V(4).Info("Stop Reconciling machineclass: ", class.Name)

	// Validate internal to external scheme conversion
	internalClass := &machine.MachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}

	// Add finalizer to avoid losing machineClass object
	if class.DeletionTimestamp == nil {
		err = c.addMachineClassFinalizers(class)
		if err != nil {
			return err
		}

		return nil
	}

	// fetch all machines referring the machineClass
	machines, err := c.findMachinesForClass(machineutils.MachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if len(machines) > 0 {
		// machines are still referring the machine class, please wait before deletion
		klog.V(3).Infof("Cannot remove finalizer on %s because still (%d) machines are referencing it", class.Name, len(machines))

		for _, machine := range machines {
			c.addMachine(machine)
		}

		return fmt.Errorf("Retry as machine objects are still referring the machineclass")
	}

	// delete machine class finalizer if exists
	return c.deleteMachineClassFinalizers(class)
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addMachineClassFinalizers(class *v1alpha1.MachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(MCMFinalizerName) {
		finalizers.Insert(MCMFinalizerName)
		return c.updateMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteMachineClassFinalizers(class *v1alpha1.MachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCMFinalizerName) {
		finalizers.Delete(MCMFinalizerName)
		return c.updateMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) updateMachineClassFinalizers(class *v1alpha1.MachineClass, finalizers []string) error {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.MachineClasses(class.Namespace).Get(class.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.MachineClasses(class.Namespace).Update(clone)
	if err != nil {
		klog.Warning("Updating machineClass failed, retrying. ", class.Name, err)
		return err
	}
	klog.V(3).Infof("Successfully added/removed finalizer on the machineclass %q", class.Name)
	return err
}

func (c *controller) enqueueMachineClassAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.machineClassQueue.AddAfter(key, after)
}
