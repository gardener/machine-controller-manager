/*
Copyright 2017 The Gardener Authors.

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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"

	"github.com/golang/glog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
)

// AWSMachineClassKind is used to identify the machineClassKind as AWS
const AWSMachineClassKind = "AWSMachineClass"

func (c *controller) machineDeploymentToAWSMachineClassDelete(obj interface{}) {
	machineDeployment, ok := obj.(*v1alpha1.MachineDeployment)
	if machineDeployment == nil || !ok {
		return
	}
	if machineDeployment.Spec.Template.Spec.Class.Kind == AWSMachineClassKind {
		c.awsMachineClassQueue.Add(machineDeployment.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineSetToAWSMachineClassDelete(obj interface{}) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)
	if machineSet == nil || !ok {
		return
	}
	if machineSet.Spec.Template.Spec.Class.Kind == AWSMachineClassKind {
		c.awsMachineClassQueue.Add(machineSet.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineToAWSMachineClassDelete(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		return
	}
	if machine.Spec.Class.Kind == AWSMachineClassKind {
		c.awsMachineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) awsMachineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.awsMachineClassQueue.Add(key)
}

func (c *controller) awsMachineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.AWSMachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := oldObj.(*v1alpha1.AWSMachineClass)
	if new == nil || !ok {
		return
	}

	c.awsMachineClassAdd(newObj)
}

// reconcileClusterAWSMachineClassKey reconciles an AWSMachineClass due to controller resync
// or an event on the awsMachineClass.
func (c *controller) reconcileClusterAWSMachineClassKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.awsMachineClassLister.AWSMachineClasses(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		glog.Infof("%s %q: Not doing work because it has been deleted", AWSMachineClassKind, key)
		return nil
	}
	if err != nil {
		glog.Infof("%s %q: Unable to retrieve object from store: %v", AWSMachineClassKind, key, err)
		return err
	}

	return c.reconcileClusterAWSMachineClass(class)
}

func (c *controller) reconcileClusterAWSMachineClass(class *v1alpha1.AWSMachineClass) error {
	internalClass := &machine.AWSMachineClass{}
	err := api.Scheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}
	// TODO this should be put in own API server
	validationerr := validation.ValidateAWSMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of %s failed %s", AWSMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	// Manipulate finalizers
	if class.DeletionTimestamp == nil {
		c.addAWSMachineClassFinalizers(class)
	}

	machines, err := c.findMachinesForClass(AWSMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp != nil {
		if finalizers := sets.NewString(class.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			return nil
		}

		machineDeployments, err := c.findMachineDeploymentsForClass(AWSMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		machineSets, err := c.findMachineSetsForClass(AWSMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		if len(machineDeployments) == 0 && len(machineSets) == 0 && len(machines) == 0 {
			c.deleteAWSMachineClassFinalizers(class)
			return nil
		}

		glog.V(4).Infof("Cannot remove finalizer of %s because still Machine[s|Sets|Deployments] are referencing it", AWSMachineClassKind, class.Name)
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

func (c *controller) addAWSMachineClassFinalizers(class *v1alpha1.AWSMachineClass) {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateAWSMachineClassFinalizers(clone, finalizers.List())
	}
}

func (c *controller) deleteAWSMachineClassFinalizers(class *v1alpha1.AWSMachineClass) {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateAWSMachineClassFinalizers(clone, finalizers.List())
	}
}

func (c *controller) updateAWSMachineClassFinalizers(class *v1alpha1.AWSMachineClass, finalizers []string) {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.AWSMachineClasses(class.Namespace).Get(class.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.AWSMachineClasses(class.Namespace).Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.Warning("Updated failed, retrying: %v", err)
		c.updateAWSMachineClassFinalizers(class, finalizers)
	}
}
