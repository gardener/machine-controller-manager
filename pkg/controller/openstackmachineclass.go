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
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
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

func (c *controller) machineToOpenStackMachineClassDelete(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		return
	}
	if machine.Spec.Class.Kind == OpenStackMachineClassKind {
		c.openStackMachineClassQueue.Add(machine.Spec.Class.Name)
	}
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

// reconcileClusterOpenStackMachineClassKey reconciles an OpenStackMachineClass due to controller resync
// or an event on the openStackMachineClass.
func (c *controller) reconcileClusterOpenStackMachineClassKey(key string) error {
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

	return c.reconcileClusterOpenStackMachineClass(class)
}

func (c *controller) reconcileClusterOpenStackMachineClass(class *v1alpha1.OpenStackMachineClass) error {
	klog.V(4).Info("Start Reconciling openStackmachineclass: ", class.Name)
	defer func() {
		c.enqueueOpenStackMachineClassAfter(class, 10*time.Minute)
		klog.V(4).Info("Stop Reconciling openStackmachineclass: ", class.Name)
	}()

	internalClass := &machine.OpenStackMachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}

	// TODO this should be put in own API server
	validationerr := validation.ValidateOpenStackMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		klog.Errorf("Validation of %s failed %s", OpenStackMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	// Manipulate finalizers
	if class.DeletionTimestamp == nil {
		err := c.addOpenStackMachineClassFinalizers(class)
		if err != nil {
			return err
		}
	}

	machines, err := c.findMachinesForClass(OpenStackMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp != nil {
		if finalizers := sets.NewString(class.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			return nil
		}

		machineDeployments, err := c.findMachineDeploymentsForClass(OpenStackMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		machineSets, err := c.findMachineSetsForClass(OpenStackMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		if len(machineDeployments) == 0 && len(machineSets) == 0 && len(machines) == 0 {
			return c.deleteOpenStackMachineClassFinalizers(class)
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

func (c *controller) addOpenStackMachineClassFinalizers(class *v1alpha1.OpenStackMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updateOpenStackMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteOpenStackMachineClassFinalizers(class *v1alpha1.OpenStackMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		return c.updateOpenStackMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) updateOpenStackMachineClassFinalizers(class *v1alpha1.OpenStackMachineClass, finalizers []string) error {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.OpenStackMachineClasses(class.Namespace).Get(class.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.OpenStackMachineClasses(class.Namespace).Update(clone)
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
