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

// GCPMachineClassKind is used to identify the machineClassKind as GCP
const GCPMachineClassKind = "GCPMachineClass"

func (c *controller) machineDeploymentToGCPMachineClassDelete(obj interface{}) {
	machineDeployment, ok := obj.(*v1alpha1.MachineDeployment)
	if machineDeployment == nil || !ok {
		return
	}
	if machineDeployment.Spec.Template.Spec.Class.Kind == GCPMachineClassKind {
		c.gcpMachineClassQueue.Add(machineDeployment.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineSetToGCPMachineClassDelete(obj interface{}) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)
	if machineSet == nil || !ok {
		return
	}
	if machineSet.Spec.Template.Spec.Class.Kind == GCPMachineClassKind {
		c.gcpMachineClassQueue.Add(machineSet.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineToGCPMachineClassDelete(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		return
	}
	if machine.Spec.Class.Kind == GCPMachineClassKind {
		c.gcpMachineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) gcpMachineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.gcpMachineClassQueue.Add(key)
}

func (c *controller) gcpMachineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.GCPMachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := newObj.(*v1alpha1.GCPMachineClass)
	if new == nil || !ok {
		return
	}

	c.gcpMachineClassAdd(newObj)
}

// reconcileClusterGCPMachineClassKey reconciles an GCPMachineClass due to controller resync
// or an event on the gcpMachineClass.
func (c *controller) reconcileClusterGCPMachineClassKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.gcpMachineClassLister.GCPMachineClasses(c.namespace).Get(name)

	if errors.IsNotFound(err) {
		klog.Infof("%s %q: Not doing work because it has been deleted", GCPMachineClassKind, key)
		return nil
	}
	if err != nil {
		klog.Infof("%s %q: Unable to retrieve object from store: %v", GCPMachineClassKind, key, err)
		return err
	}

	return c.reconcileClusterGCPMachineClass(class)
}

func (c *controller) reconcileClusterGCPMachineClass(class *v1alpha1.GCPMachineClass) error {

	klog.V(4).Info("Start Reconciling gcpmachineclass: ", class.Name)
	defer func() {
		c.enqueueGcpMachineClassAfter(class, 10*time.Minute)
		klog.V(4).Info("Stop Reconciling gcpmachineclass: ", class.Name)
	}()

	internalClass := &machine.GCPMachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}
	// TODO this should be put in own API server
	validationerr := validation.ValidateGCPMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		klog.Errorf("Validation of %s failed %s", GCPMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	// Manipulate finalizers
	if class.DeletionTimestamp == nil {
		err = c.addGCPMachineClassFinalizers(class)
		if err != nil {
			return err
		}
	}

	machines, err := c.findMachinesForClass(GCPMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp != nil {
		if finalizers := sets.NewString(class.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			return nil
		}

		machineDeployments, err := c.findMachineDeploymentsForClass(GCPMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		machineSets, err := c.findMachineSetsForClass(GCPMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		if len(machineDeployments) == 0 && len(machineSets) == 0 && len(machines) == 0 {
			return c.deleteGCPMachineClassFinalizers(class)
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

func (c *controller) addGCPMachineClassFinalizers(class *v1alpha1.GCPMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updateGCPMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteGCPMachineClassFinalizers(class *v1alpha1.GCPMachineClass) error {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		return c.updateGCPMachineClassFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) updateGCPMachineClassFinalizers(class *v1alpha1.GCPMachineClass, finalizers []string) error {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.GCPMachineClasses(class.Namespace).Get(class.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.GCPMachineClasses(class.Namespace).Update(clone)
	if err != nil {
		klog.Warning("Updating GCPMachineClass failed, retrying. ", class.Name, err)
		return err
	}
	klog.V(3).Infof("Successfully added/removed finalizer on the gcpmachineclass %q", class.Name)
	return err
}

func (c *controller) enqueueGcpMachineClassAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.gcpMachineClassQueue.AddAfter(key, after)
}
