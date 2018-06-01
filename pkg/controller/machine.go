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
	"bytes"
	"errors"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/gardener/machine-controller-manager/pkg/driver"
)

const (
	// MachinePriority is the annotation used to specify priority
	// associated with a machine while deleting it. The less its
	// priority the more likely it is to be deleted first
	// Default priority for a machine is set to 3
	MachinePriority = "machinepriority.machine.sapcloud.io"
)

/*
	SECTION
	Machine controller - Machine add, update, delete watches
*/
func (c *controller) addMachine(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	glog.V(4).Info("Adding machine object")
	c.machineQueue.Add(key)
}

func (c *controller) updateMachine(oldObj, newObj interface{}) {
	glog.V(4).Info("Updating machine object")
	c.addMachine(newObj)
}

func (c *controller) deleteMachine(obj interface{}) {
	glog.V(4).Info("Deleting machine object")
	c.addMachine(obj)
}

func (c *controller) enqueueMachineAfter(obj interface{}, after time.Duration) {
	key, err := KeyFunc(obj)
	if err != nil {
		return
	}
	c.machineQueue.AddAfter(key, after)
}

func (c *controller) reconcileClusterMachineKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	machine, err := c.machineLister.Machines(c.namespace).Get(name)
	if apierrors.IsNotFound(err) {
		glog.V(4).Infof("Machine %q: Not doing work because it is not found", key)
		return nil
	}

	if err != nil {
		glog.Errorf("ClusterMachine %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterMachine(machine)
}

func (c *controller) reconcileClusterMachine(machine *v1alpha1.Machine) error {
	glog.V(4).Info("Start Reconciling machine: ", machine.Name)
	defer func() {
		glog.V(4).Info("Stop Reconciling machine: ", machine.Name)
	}()

	if !shouldReconcileMachine(machine, time.Now()) {
		return nil
	}

	// Validate Machine
	internalMachine := &machineapi.Machine{}
	err := c.internalExternalScheme.Convert(machine, internalMachine, nil)
	if err != nil {
		return err
	}
	validationerr := validation.ValidateMachine(internalMachine)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of Machine failed %s", validationerr.ToAggregate().Error())
		return nil
	}

	// Validate MachineClass
	MachineClass, secretRef, err := c.validateMachineClass(&machine.Spec.Class)
	if err != nil || secretRef == nil {
		return err
	}

	driver := driver.NewDriver(machine.Spec.ProviderID, secretRef, machine.Spec.Class.Kind, MachineClass, machine.Name)
	actualProviderID, err := driver.GetExisting()
	if err != nil {
		return err
	} else if actualProviderID == "fake" {
		glog.Warning("Fake driver type")
		return nil
	}

	//glog.Info("REACHED ", actualProviderID, " ", machineID)
	// Get the latest version of the machine so that we can avoid conflicts
	machine, err = c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if machine.DeletionTimestamp != nil {
		// Processing of delete event
		if err := c.machineDelete(machine, driver); err != nil {
			return err
		}
	} else if machine.Status.CurrentStatus.TimeoutActive {
		// Processing machine
		c.checkMachineTimeout(machine)
	} else {
		// Processing of create or update event
		c.addMachineFinalizers(machine)

		if machine.Status.CurrentStatus.Phase == v1alpha1.MachineFailed {
			return nil
		} else if actualProviderID == "" {
			if err := c.machineCreate(machine, driver); err != nil {
				return err
			}
		} else if actualProviderID != machine.Spec.ProviderID {
			if err := c.machineUpdate(machine, actualProviderID); err != nil {
				return err
			}
		}
	}

	return nil
}

/*
	SECTION
	Machine controller - nodeToMachine
*/
func (c *controller) addNodeToMachine(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.nodeToMachineQueue.Add(key)
}

func (c *controller) updateNodeToMachine(oldObj, newObj interface{}) {
	c.addNodeToMachine(newObj)
}

func (c *controller) deleteNodeToMachine(obj interface{}) {
	c.addNodeToMachine(obj)
}

func (c *controller) reconcileClusterNodeToMachineKey(key string) error {
	node, err := c.nodeLister.Get(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		glog.Errorf("ClusterNode %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterNodeToMachine(node)
}

func (c *controller) reconcileClusterNodeToMachine(node *v1.Node) error {
	machine, err := c.getMachineFromNode(node.Name)

	if err != nil {
		glog.Error("Couldn't fetch machine: ", err)
		return err
	} else if machine == nil {
		return nil
	}

	err = c.updateMachineState(machine, node)
	if err != nil {
		return err
	}

	return nil
}

/*
	SECTION
	NodeToMachine operations
*/

func (c *controller) getMachineFromNode(nodeName string) (*v1alpha1.Machine, error) {
	var (
		list     = []string{nodeName}
		selector = labels.NewSelector()
		req, _   = labels.NewRequirement("node", selection.Equals, list)
	)

	selector = selector.Add(*req)
	machines, _ := c.machineLister.List(selector)

	if len(machines) > 1 {
		return nil, errors.New("Multiple machines matching node")
	} else if len(machines) < 1 {
		return nil, nil
	}

	return machines[0], nil
}

func (c *controller) updateMachineState(machine *v1alpha1.Machine, node *v1.Node) error {
	machine = c.updateMachineConditions(machine, node.Status.Conditions)

	if machine.Status.LastOperation.State != "Successful" {

		if machine.Status.LastOperation.Type == "Create" &&
			machine.Status.Conditions != nil {
			/*
				TODO: Fix this
				if machine.Status.LastOperation.Description == "Creating machine on cloud provider" {
					// Machine is ready but yet to join the cluster
					lastOperation := v1alpha1.LastOperation {
						Description: 	"Waiting for machine to join the cluster (Not Ready)",
						State: 			"Processing",
						Type:			"Create",
						LastUpdateTime: metav1.Now(),
					}
					currentStatus := v1alpha1.CurrentStatus {
						Phase:			v1alpha1.MachineAvailable,
						TimeoutActive:	true,
						LastUpdateTime: machine.Status.CurrentStatus.LastUpdateTime,
					}
					c.updateMachineStatus(machine, lastOperation, currentStatus)

				} else*/
			for _, v := range machine.Status.Conditions {
				if v.Reason == "KubeletReady" && v.Status == "True" {
					// Machine is ready and has joined the cluster
					lastOperation := v1alpha1.LastOperation{
						Description:    "Machine is now ready",
						State:          "Successful",
						Type:           "Create",
						LastUpdateTime: metav1.Now(),
					}
					c.updateMachineStatus(machine, lastOperation, machine.Status.CurrentStatus)
					break

				}
			}

		}
	}
	return nil
}

/*
	SECTION
	Machine operations - Create, Update, Delete
*/

func (c *controller) machineCreate(machine *v1alpha1.Machine, driver driver.Driver) error {
	glog.V(2).Infof("Creating machine %s, please wait!", machine.Name)

	actualProviderID, nodeName, err := driver.Create()
	if err != nil {
		glog.V(2).Infof("Error while creating machine %s: %s", machine.Name, err.Error())
		lastOperation := v1alpha1.LastOperation{
			Description:    "Cloud provider message - " + err.Error(),
			State:          "Failed",
			Type:           "Create",
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineFailed,
			TimeoutActive:  false,
			LastUpdateTime: metav1.Now(),
		}
		c.updateMachineStatus(machine, lastOperation, currentStatus)
		return err
	}
	glog.V(2).Infof("Created machine: %s, MachineID: %s", machine.Name, actualProviderID)

	for {
		// Get the latest version of the machine so that we can avoid conflicts
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		lastOperation := v1alpha1.LastOperation{
			Description:    "Creating machine on cloud provider",
			State:          "Processing",
			Type:           "Create",
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachinePending,
			TimeoutActive:  true,
			LastUpdateTime: metav1.Now(),
		}

		clone := machine.DeepCopy()

		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}
		clone.Labels["node"] = nodeName
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		if clone.Annotations[MachinePriority] == "" {
			clone.Annotations[MachinePriority] = "3"
		}

		clone.Spec.ProviderID = actualProviderID
		clone.Status.Node = nodeName
		clone.Status.LastOperation = lastOperation
		clone.Status.CurrentStatus = currentStatus

		_, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err == nil {
			break
		}
		glog.Warning("Updated failed, retrying, error: %q", err)
	}

	return nil
}

func (c *controller) machineUpdate(machine *v1alpha1.Machine, actualProviderID string) error {
	glog.V(2).Infof("Setting MachineId of %s to %s", machine.Name, actualProviderID)

	for {
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		clone := machine.DeepCopy()
		clone.Spec.ProviderID = actualProviderID
		lastOperation := v1alpha1.LastOperation{
			Description:    "Updated provider ID",
			State:          "Successful",
			Type:           "Update",
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.LastOperation = lastOperation

		_, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err == nil {
			break
		}
		glog.Warning("Updated failed, retrying, error: %q", err)
	}

	return nil
}

func (c *controller) machineDelete(machine *v1alpha1.Machine, driver driver.Driver) error {
	var err error

	if finalizers := sets.NewString(machine.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		glog.V(2).Infof("Deleting Machine %s", machine.Name)

		// If machine was created on the cloud provider
		machineID, _ := driver.GetExisting()

		if machine.Status.CurrentStatus.Phase != v1alpha1.MachineTerminating {
			lastOperation := v1alpha1.LastOperation{
				Description:    "Deleting machine from cloud provider",
				State:          "Processing",
				Type:           "Delete",
				LastUpdateTime: metav1.Now(),
			}
			currentStatus := v1alpha1.CurrentStatus{
				Phase:          v1alpha1.MachineTerminating,
				TimeoutActive:  false,
				LastUpdateTime: metav1.Now(),
			}
			machine, err = c.updateMachineStatus(machine, lastOperation, currentStatus)
			if err != nil && apierrors.IsNotFound(err) {
				// Object no longer exists and has been deleted
				glog.Warning(err)
				return nil
			} else if err != nil {
				// Any other type of errors
				glog.Error(err)
				return err
			}
		}

		if machineID != "" {
			timeOutDuration := time.Duration(c.safetyOptions.MachineDrainTimeout) * time.Minute
			// Timeout value obtained by subtracting last operation with expected time out period
			timeOut := metav1.Now().Add(-timeOutDuration).Sub(machine.Status.CurrentStatus.LastUpdateTime.Time)

			// To perform drain 2 conditions must be satified
			// 1. force-deletion: "True" label must not be present
			// 2. Deletion operation must be less than 5 minutes old
			if machine.Labels["force-deletion"] != "True" && timeOut < 0 {
				buf := bytes.NewBuffer([]byte{})
				errBuf := bytes.NewBuffer([]byte{})

				nodeName := machine.Labels["node"]
				drainOptions := NewDrainOptions(
					c.targetCoreClient,
					timeOutDuration, // TODO: Will need to configure timeout
					nodeName,
					-1,
					true,
					true,
					true,
					buf,
					errBuf,
				)
				err = drainOptions.RunDrain()
				if err != nil {
					lastOperation := v1alpha1.LastOperation{
						Description:    "Drain failed - " + err.Error(),
						State:          "Failed",
						Type:           "Delete",
						LastUpdateTime: metav1.Now(),
					}
					c.updateMachineStatus(machine, lastOperation, machine.Status.CurrentStatus)

					// Machine still tries to terminate after drain failure
					glog.V(2).Infof("Drain failed for machine %s - \nBuf:%v \nErrBuf:%v \nErr-Message:%v", machine.Name, buf, errBuf, err)
					return err
				}
				glog.V(2).Infof("Drain successful for machine %s - %v %v", machine.Name, buf, errBuf)
			}
			err = driver.Delete()
		}

		if err != nil {
			// When machine deletion fails
			glog.V(2).Infof("Deletion failed: %s", err)

			lastOperation := v1alpha1.LastOperation{
				Description:    "Cloud provider message - " + err.Error(),
				State:          "Failed",
				Type:           "Delete",
				LastUpdateTime: metav1.Now(),
			}
			currentStatus := v1alpha1.CurrentStatus{
				Phase:          v1alpha1.MachineFailed,
				TimeoutActive:  false,
				LastUpdateTime: metav1.Now(),
			}
			c.updateMachineStatus(machine, lastOperation, currentStatus)

			return err
		}

		c.deleteMachineFinalizers(machine)
		c.controlMachineClient.Machines(machine.Namespace).Delete(machine.Name, &metav1.DeleteOptions{})
		glog.V(2).Infof("Machine %s deleted successfullly", machine.Name)
	}
	return nil
}

/*
	SECTION
	Update machine object
*/

func (c *controller) updateMachineStatus(
	machine *v1alpha1.Machine,
	lastOperation v1alpha1.LastOperation,
	currentStatus v1alpha1.CurrentStatus,
) (*v1alpha1.Machine, error) {
	// Get the latest version of the machine so that we can avoid conflicts
	clone, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return machine, err
	}

	clone = clone.DeepCopy()
	clone.Status.LastOperation = lastOperation
	clone.Status.CurrentStatus = currentStatus

	clone, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(3).Infof("Warning: Updated failed, retrying, error: %q", err)
		return c.updateMachineStatus(machine, lastOperation, currentStatus)
	}
	return clone, nil
}

func (c *controller) updateMachineConditions(machine *v1alpha1.Machine, conditions []v1.NodeCondition) *v1alpha1.Machine {
	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return machine
	}

	clone := machine.DeepCopy()
	clone.Status.Conditions = conditions

	//glog.Info(c.isHealthy(clone))

	if clone.Status.CurrentStatus.Phase == v1alpha1.MachineTerminating {
		// If machine is already in terminating state, don't update
	} else if !c.isHealthy(clone) && clone.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
		currentStatus := v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineUnknown,
			TimeoutActive:  true,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = currentStatus

	} else if c.isHealthy(clone) && clone.Status.CurrentStatus.Phase != v1alpha1.MachineRunning {
		currentStatus := v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineRunning,
			TimeoutActive:  false,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = currentStatus

	}

	clone, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(2).Infof("Warning: Updated failed, retrying, error: %q", err)
		c.updateMachineConditions(machine, conditions)
		return machine
	}

	return clone
}

func (c *controller) updateMachineFinalizers(machine *v1alpha1.Machine, finalizers []string) {
	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := machine.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(4).Info("Warning: Updated failed, retrying, error: %q", err)
		c.updateMachineFinalizers(machine, finalizers)
	}
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addMachineFinalizers(machine *v1alpha1.Machine) {
	clone := machine.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateMachineFinalizers(clone, finalizers.List())
	}
}

func (c *controller) deleteMachineFinalizers(machine *v1alpha1.Machine) {
	clone := machine.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateMachineFinalizers(clone, finalizers.List())
	}
}

/*
	SECTION
	Helper Functions
*/
func (c *controller) isHealthy(machine *v1alpha1.Machine) bool {
	numOfConditions := len(machine.Status.Conditions)

	if numOfConditions == 0 {
		// Kubernetes node object for this machine hasn't been recieved
		return false
	}

	for _, condition := range machine.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
			// If Kubelet is not ready
			return false
		} else if condition.Type == v1.NodeDiskPressure && condition.Status != v1.ConditionFalse {
			// If DiskPressure has occurred on node
			return false
		}
	}
	return true
}

func (c *controller) getSecret(ref *v1.SecretReference, MachineClassName string) (*v1.Secret, error) {
	secretRef, err := c.secretLister.Secrets(ref.Namespace).Get(ref.Name)
	if apierrors.IsNotFound(err) {
		glog.V(2).Infof("No secret %q: found for MachineClass %q", ref, MachineClassName)
		return nil, nil
	}
	if err != nil {
		glog.Errorf("Unable get secret %q for MachineClass %q: %v", MachineClassName, ref, err)
		return nil, err
	}
	return secretRef, err
}

func (c *controller) checkMachineTimeout(machine *v1alpha1.Machine) {
	if machine.Status.CurrentStatus.Phase != v1alpha1.MachineRunning {

		timeOutDuration := time.Duration(c.safetyOptions.MachineHealthTimeout) * time.Minute
		sleepTime := 1 * time.Minute

		// Timeout value obtained by subtracting last operation with expected time out period
		timeOut := metav1.Now().Add(-timeOutDuration).Sub(machine.Status.CurrentStatus.LastUpdateTime.Time)
		//glog.V(2).Info("TIMEOUT: ", machine.Name, " ", timeOut)

		if timeOut > 0 {

			currentStatus := v1alpha1.CurrentStatus{
				Phase:          v1alpha1.MachineFailed,
				TimeoutActive:  false,
				LastUpdateTime: metav1.Now(),
			}

			if machine.Status.CurrentStatus.Phase == v1alpha1.MachinePending {
				lastOperation := v1alpha1.LastOperation{
					Description:    "Machine could not join the cluster. Operation timed out",
					State:          "Failed",
					Type:           machine.Status.LastOperation.Type,
					LastUpdateTime: metav1.Now(),
				}
				c.updateMachineStatus(machine, lastOperation, currentStatus)

			} else {
				c.updateMachineStatus(machine, machine.Status.LastOperation, currentStatus)

			}
		} else {
			currentStatus := v1alpha1.CurrentStatus{
				Phase:          machine.Status.CurrentStatus.Phase,
				TimeoutActive:  true,
				LastUpdateTime: machine.Status.CurrentStatus.LastUpdateTime,
			}
			c.updateMachineStatus(machine, machine.Status.LastOperation, currentStatus)
			c.enqueueMachineAfter(machine, sleepTime)
		}
	}
}

func shouldReconcileMachine(machine *v1alpha1.Machine, now time.Time) bool {
	if machine.DeletionTimestamp != nil {
		return true
	}
	if machine.Spec.ProviderID == "" {
		return true
	}
	// TODO add more cases where this will be false

	return true
}
