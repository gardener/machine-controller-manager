package controller

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
)

// AliyunMachineClassKind is used to identify the machineClassKind as Aliyun
const AliyunMachineClassKind = "AliyunMachineClass"

func (c *controller) machineDeploymentToAliyunMachineClassDelete(obj interface{}) {
	machineDeployment, ok := obj.(*v1alpha1.MachineDeployment)
	if machineDeployment == nil || !ok {
		return
	}
	if machineDeployment.Spec.Template.Spec.Class.Kind == AliyunMachineClassKind {
		c.aliyunMachineClassQueue.Add(machineDeployment.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineSetToAliyunMachineClassDelete(obj interface{}) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)
	if machineSet == nil || !ok {
		return
	}
	if machineSet.Spec.Template.Spec.Class.Kind == AliyunMachineClassKind {
		c.aliyunMachineClassQueue.Add(machineSet.Spec.Template.Spec.Class.Name)
	}
}

func (c *controller) machineToAliyunMachineClassDelete(obj interface{}) {
	machine, ok := obj.(*v1alpha1.Machine)
	if machine == nil || !ok {
		return
	}
	if machine.Spec.Class.Kind == AliyunMachineClassKind {
		c.aliyunMachineClassQueue.Add(machine.Spec.Class.Name)
	}
}

func (c *controller) aliyunMachineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.aliyunMachineClassQueue.Add(key)
}

func (c *controller) aliyunMachineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.AliyunMachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := newObj.(*v1alpha1.AliyunMachineClass)
	if new == nil || !ok {
		return
	}

	c.aliyunMachineClassAdd(newObj)
}

// reconcileClusterAliyunMachineClassKey reconciles an AliyunMachineClass due to controller resync
// or an event on the aliyunMachineClass.
func (c *controller) reconcileClusterAliyunMachineClassKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	class, err := c.aliyunMachineClassLister.AliyunMachineClasses(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		glog.Infof("%s %q: Not doing work because it has been deleted", AliyunMachineClassKind, key)
		return nil
	}
	if err != nil {
		glog.Infof("%s %q: Unable to retrieve object from store: %v", AliyunMachineClassKind, key, err)
		return err
	}

	return c.reconcileClusterAliyunMachineClass(class)
}

func (c *controller) reconcileClusterAliyunMachineClass(class *v1alpha1.AliyunMachineClass) error {
	internalClass := &machine.AliyunMachineClass{}
	err := c.internalExternalScheme.Convert(class, internalClass, nil)
	if err != nil {
		return err
	}
	// TODO this should be put in own API server
	validationerr := validation.ValidateAliyunMachineClass(internalClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of %s failed %s", AliyunMachineClassKind, validationerr.ToAggregate().Error())
		return nil
	}

	// Manipulate finalizers
	if class.DeletionTimestamp == nil {
		c.addAliyunMachineClassFinalizers(class)
	}

	machines, err := c.findMachinesForClass(AliyunMachineClassKind, class.Name)
	if err != nil {
		return err
	}

	if class.DeletionTimestamp != nil {
		if finalizers := sets.NewString(class.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			return nil
		}

		machineDeployments, err := c.findMachineDeploymentsForClass(AliyunMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		machineSets, err := c.findMachineSetsForClass(AliyunMachineClassKind, class.Name)
		if err != nil {
			return err
		}
		if len(machineDeployments) == 0 && len(machineSets) == 0 && len(machines) == 0 {
			c.deleteAliyunMachineClassFinalizers(class)
			return nil
		}

		glog.V(4).Infof("Cannot remove finalizer of %s because still Machine[s|Sets|Deployments] are referencing it", AliyunMachineClassKind, class.Name)
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

func (c *controller) addAliyunMachineClassFinalizers(class *v1alpha1.AliyunMachineClass) {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateAliyunMachineClassFinalizers(clone, finalizers.List())
	}
}

func (c *controller) deleteAliyunMachineClassFinalizers(class *v1alpha1.AliyunMachineClass) {
	clone := class.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateAliyunMachineClassFinalizers(clone, finalizers.List())
	}
}

func (c *controller) updateAliyunMachineClassFinalizers(class *v1alpha1.AliyunMachineClass, finalizers []string) {
	// Get the latest version of the class so that we can avoid conflicts
	class, err := c.controlMachineClient.AliyunMachineClasses(class.Namespace).Get(class.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := class.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.AliyunMachineClasses(class.Namespace).Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.Warning("Updated failed, retrying: %v", err)
		c.updateAliyunMachineClassFinalizers(class, finalizers)
	}
}
