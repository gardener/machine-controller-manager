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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

// machineClassKind is used to identify the machineClassKind for generic machineClasses
const machineClassKind = "MachineClass"

func (c *controller) machineToMachineClassDelete(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		return
	}
	if machine.Spec.Class.Kind == machineClassKind {
		c.machineClassQueue.Add(machine.Spec.Class.Name)
	}
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

// reconcileClusterMachineClassKey reconciles an machineClass due to controller resync
// or an event on the machineClass.
func (c *controller) reconcileClusterMachineClassKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.machineClassLister.MachineClasses(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.Infof("%s %q: Not doing work because it has been deleted", machineClassKind, key)
		return nil
	}
	if err != nil {
		klog.Infof("%s %q: Unable to retrieve object from store: %v", machineClassKind, key, err)
		return err
	}

	return c.reconcileClusterMachineClass(class)
}

func (c *controller) reconcileClusterMachineClass(class *v1alpha1.MachineClass) error {
	klog.V(4).Info("Start Reconciling machineclass: ", class.Name)
	defer func() {
		c.enqueueMachineClassAfter(class, 10*time.Minute)
		klog.V(4).Info("Stop Reconciling machineclass: ", class.Name)
	}()

	internalClass := &machine.MachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}

	machines, err := c.findMachinesForClass(machineClassKind, class.Name)
	if err != nil {
		return err
	}

	// Manipulate finalizers
	if class.DeletionTimestamp == nil && len(machines) > 0 {
		err = c.addMachineClassFinalizers(class)
		if err != nil {
			return err
		}
	} else {
		if finalizers := sets.NewString(class.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			return nil
		}

		if len(machines) == 0 {
			return c.deleteMachineClassFinalizers(class)
		}

		klog.V(3).Infof("Cannot remove finalizer of %s because still Machine[s|Sets|Deployments] are referencing it", class.Name)
		return nil
	}

	for _, machine := range machines {
		c.addMachine(machine)
	}
	return nil
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addMachineClassFinalizers(class *v1alpha1.MachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updateMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteMachineClassFinalizers(class *v1alpha1.MachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
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
