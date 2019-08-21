/*
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

	"github.com/golang/glog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
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
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
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
		glog.Infof("%s %q: Not doing work because it has been deleted", PacketMachineClassKind, key)
		return nil
	}
	if err != nil {
		glog.Infof("%s %q: Unable to retrieve object from store: %v", PacketMachineClassKind, key, err)
		return err
	}

	return c.reconcileClusterPacketMachineClass(class)
}

func (c *controller) reconcileClusterPacketMachineClass(class *v1alpha1.PacketMachineClass) error {
	glog.V(4).Info("Start Reconciling packetmachineclass: ", class.Name)
	defer func() {
		c.enqueuePacketMachineClassAfter(class, 10*time.Minute)
		glog.V(4).Info("Stop Reconciling packetmachineclass: ", class.Name)
	}()

	internalClass := &machine.PacketMachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}
	// TODO this should be put in own API server
	validationerr := validation.ValidatePacketMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of %s failed %s", PacketMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	// Manipulate finalizers
	if class.DeletionTimestamp == nil {
		err = c.addPacketMachineClassFinalizers(class)
		if err != nil {
			return err
		}
	}

	machines, err := c.findMachinesForClass(PacketMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp != nil {
		if finalizers := sets.NewString(class.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			return nil
		}

		machineDeployments, err := c.findMachineDeploymentsForClass(PacketMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		machineSets, err := c.findMachineSetsForClass(PacketMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		if len(machineDeployments) == 0 && len(machineSets) == 0 && len(machines) == 0 {
			return c.deletePacketMachineClassFinalizers(class)
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
		glog.Warning("Updating PacketMachineClass failed, retrying. ", class.Name, err)
		return err
	}
	glog.V(3).Infof("Successfully added/removed finalizer on the packetmachineclass %q", class.Name)
	return err
}

func (c *controller) enqueuePacketMachineClassAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.openStackMachineClassQueue.AddAfter(key, after)
}
