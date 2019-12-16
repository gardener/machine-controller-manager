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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
)

// MachineClassKind is used to identify the machineClassKind as
const MachineClassKind = "MachineClass"

func (c *controller) machineDeploymentToMachineClassDelete(obj interface{}) {
	machineDeployment, ok := obj.(*v1alpha1.MachineDeployment)
	if machineDeployment == nil || !ok {
		return
	}
	if machineDeployment.Spec.Template.Spec.Class.Kind == MachineClassKind {
		c.machineClassQueue.Add(machineDeployment.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineSetToMachineClassDelete(obj interface{}) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)
	if machineSet == nil || !ok {
		return
	}
	if machineSet.Spec.Template.Spec.Class.Kind == MachineClassKind {
		c.machineClassQueue.Add(machineSet.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineToMachineClassDelete(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		return
	}
	if machine.Spec.Class.Kind == MachineClassKind {
		c.machineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) machineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
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
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.machineClassQueue.Add(key)
}

// reconcileClusterMachineClassKey reconciles an MachineClass due to controller resync
// or an event on the MachineClass.
func (c *controller) reconcileClusterMachineClassKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.machineClassLister.MachineClasses(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		glog.Infof("%s %q: Not doing work because it has been deleted", MachineClassKind, key)
		return nil
	}
	if err != nil {
		glog.Infof("%s %q: Unable to retrieve object from store: %v", MachineClassKind, key, err)
		return err
	}

	return c.reconcileClusterMachineClass(class)
}

func (c *controller) enqueueMachineClassAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.machineClassQueue.AddAfter(key, after)
}

func (c *controller) reconcileClusterMachineClass(class *v1alpha1.MachineClass) error {
	glog.V(4).Info("Start Reconciling machineClass: ", class.Name)
	defer func() {
		c.enqueueMachineClassAfter(class, 10*time.Minute)
		glog.V(4).Info("Stop Reconciling machineClass: ", class.Name)
	}()

	internalClass := &machine.MachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}
	// TODO this should be put in own API server
	validationerr := validation.ValidateMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.Errorf("Validation of %s failed %s", MachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	machines, err := c.findMachinesForClass(MachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp == nil {
		err = c.addMachineClassFinalizers(class)
		if err != nil {
			return err
		}
	} else {
		machineDeployments, err := c.findMachineDeploymentsForClass(MachineClassKind, class.Name)
		if err != nil {
			return err
		}
		machineSets, err := c.findMachineSetsForClass(MachineClassKind, class.Name)
		if err != nil {
			return err
		}
		if len(machineDeployments) == 0 && len(machineSets) == 0 && len(machines) == 0 {
			return c.deleteMachineClassFinalizers(class)
		}

		glog.V(3).Infof("Cannot remove finalizer of %s because still Machine[s|Sets|Deployments] are referencing it", class.Name)
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
	if finalizers := sets.NewString(class.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updateMachineClassFinalizers(class, finalizers.List())
	}
	return nil
}

func (c *controller) deleteMachineClassFinalizers(class *v1alpha1.MachineClass) error {
	if finalizers := sets.NewString(class.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		return c.updateMachineClassFinalizers(class, finalizers.List())
	}
	return nil
}

func (c *controller) updateMachineClassFinalizers(class *v1alpha1.MachineClass, finalizers []string) error {
	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err := c.controlMachineClient.MachineClasses(class.Namespace).Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.Warningf("Updating MachineClass failed for %q, retrying: %s", class.Name, err)
		return err
	}

	glog.V(3).Infof("Successfully added/removed finalizer on the machineclass %q", class.Name)
	return err
}
