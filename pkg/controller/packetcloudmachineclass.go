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
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

// PacketMachineClassKind is used to identify the machineClassKind as Packet
const PacketMachineClassKind = "PacketMachineClass"

func (c *controller) machineDeploymentToPacketMachineClassDelete(obj interface{}) {
	machineDeployment, ok := obj.(*v1alpha1.MachineDeployment)
	if machineDeployment == nil || !ok {
		return
	}
	if machineDeployment.Spec.Template.Spec.Class.Kind == PacketMachineClassKind {
		c.packetMachineClassQueue.Add(machineDeployment.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineSetToPacketMachineClassDelete(obj interface{}) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)
	if machineSet == nil || !ok {
		return
	}
	if machineSet.Spec.Template.Spec.Class.Kind == PacketMachineClassKind {
		c.packetMachineClassQueue.Add(machineSet.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineToPacketMachineClassDelete(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		return
	}
	if machine.Spec.Class.Kind == PacketMachineClassKind {
		c.packetMachineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) packetMachineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.packetMachineClassQueue.Add(key)
}

func (c *controller) packetMachineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.PacketMachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := newObj.(*v1alpha1.PacketMachineClass)
	if new == nil || !ok {
		return
	}

	c.packetMachineClassAdd(newObj)
}

// reconcileClusterPacketMachineClassKey reconciles a PacketMachineClass due to controller resync
// or an event on the packetMachineClass.
func (c *controller) reconcileClusterPacketMachineClassKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.packetMachineClassLister.PacketMachineClasses(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.Infof("%s %q: Not doing work because it has been deleted", PacketMachineClassKind, key)
		return nil
	}
	if err != nil {
		klog.Infof("%s %q: Unable to retrieve object from store: %v", PacketMachineClassKind, key, err)
		return err
	}

	err = c.reconcileClusterPacketMachineClass(class)
	if err != nil {
		c.enqueuePacketMachineClassAfter(class, 10*time.Second)
	} else {
		// Re-enqueue periodically to avoid missing of events
		// TODO: Infuture to get ride of this logic
		c.enqueuePacketMachineClassAfter(class, 10*time.Minute)
	}

	return nil
}

func (c *controller) reconcileClusterPacketMachineClass(class *v1alpha1.PacketMachineClass) error {
	klog.V(4).Info("Start Reconciling Packetmachineclass: ", class.Name)
	defer klog.V(4).Info("Stop Reconciling Packetmachineclass: ", class.Name)

	internalClass := &machine.PacketMachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}

	validationerr := validation.ValidatePacketMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		klog.Errorf("Validation of %s failed %s", PacketMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	machines, err := c.findMachinesForClass(PacketMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp == nil {
		// If deletion timestamp doesn't exist
		_, annotationPresent := class.Annotations[machineutils.MigratedMachineClass]

		if len(machines) > 0 {
			// If 1 or more machine objects are referring the machineClass
			err = c.addPacketMachineClassFinalizers(class)
			if err != nil {
				return err
			}
		} else if c.deleteMigratedMachineClass && annotationPresent {
			// If controller has deleteMigratedMachineClass flag set
			// and the migratedMachineClass annotation is set
			err = c.controlMachineClient.PacketMachineClasses(class.Namespace).Delete(class.Name, &metav1.DeleteOptions{})
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

	return c.deletePacketMachineClassFinalizers(class)
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addPacketMachineClassFinalizers(class *v1alpha1.PacketMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updatePacketMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) deletePacketMachineClassFinalizers(class *v1alpha1.PacketMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		return c.updatePacketMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) updatePacketMachineClassFinalizers(class *v1alpha1.PacketMachineClass, finalizers []string) error {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.PacketMachineClasses(class.Namespace).Get(class.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.PacketMachineClasses(class.Namespace).Update(clone)
	if err != nil {
		klog.Warning("Updating PacketMachineClass failed, retrying. ", class.Name, err)
		return err
	}
	klog.V(3).Infof("Successfully added/removed finalizer on the packetmachineclass %q", class.Name)
	return err
}

func (c *controller) enqueuePacketMachineClassAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.openStackMachineClassQueue.AddAfter(key, after)
}
