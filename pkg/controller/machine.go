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
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"

	v1 "k8s.io/api/core/v1"
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
	glog.V(4).Infof("Add/Update/Delete machine object %q", key)
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

func (c *controller) enqueueMachine(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.machineQueue.Add(key)
}

func (c *controller) enqueueMachineAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
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
		c.enqueueMachineAfter(machine, 10*time.Minute)
		glog.V(4).Info("Stop Reconciling machine: ", machine.Name)
	}()

	if c.safetyOptions.MachineControllerFrozen && machine.DeletionTimestamp == nil {
		message := "Machine controller has frozen. Retrying reconcile after 10 minutes"
		glog.V(3).Info(message)
		return errors.New(message)
	}

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
		glog.Errorf("Validation of Machine failed %s", validationerr.ToAggregate().Error())
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

	machine, err = c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Could not fetch machine object %s", err)
		return err
	}

	machine, err = c.updateMachineState(machine)
	if err != nil {
		glog.Errorf("Could not update machine state for: %s", machine.Name)
		return err
	}

	// Sync nodeTemplate between machine and node-objects.
	node, _ := c.nodeLister.Get(machine.Status.Node)
	if node != nil {
		err = c.syncMachineNodeTemplates(machine)
		if err != nil {
			glog.Errorf("Could not update nodeTemplate for machine %s err: %q", machine.Name, err)
			return err
		}
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

	machine, err := c.getMachineFromNode(key)
	if err != nil {
		glog.Errorf("Couldn't fetch machine %s, Error: %s", key, err)
		return
	} else if machine == nil {
		return
	}

	glog.V(4).Infof("Add machine object backing node %q", machine.Name)
	c.enqueueMachine(machine)
}

func (c *controller) updateNodeToMachine(oldObj, newObj interface{}) {
	c.addNodeToMachine(newObj)
}

func (c *controller) deleteNodeToMachine(obj interface{}) {
	c.addNodeToMachine(obj)
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

func (c *controller) updateMachineState(machine *v1alpha1.Machine) (*v1alpha1.Machine, error) {

	if machine.Status.Node == "" {
		// There are no objects mapped to this machine object
		// Hence node status need not be propogated to machine object
		return machine, nil
	}

	node, err := c.nodeLister.Get(machine.Status.Node)
	if err != nil && apierrors.IsNotFound(err) {
		// Node object is not found

		if len(machine.Status.Conditions) > 0 &&
			machine.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
			// If machine has conditions on it,
			// and corresponding node object went missing
			// and machine is still healthy
			msg := fmt.Sprintf(
				"Node object went missing. Machine %s is unhealthy - changing MachineState to Unknown",
				machine.Name,
			)
			glog.Warning(msg)

			currentStatus := v1alpha1.CurrentStatus{
				Phase:          v1alpha1.MachineUnknown,
				TimeoutActive:  true,
				LastUpdateTime: metav1.Now(),
			}
			lastOperation := v1alpha1.LastOperation{
				Description:    msg,
				State:          v1alpha1.MachineStateProcessing,
				Type:           v1alpha1.MachineOperationHealthCheck,
				LastUpdateTime: metav1.Now(),
			}
			clone, err := c.updateMachineStatus(machine, lastOperation, currentStatus)
			if err != nil {
				glog.Errorf("Machine updated failed for %s, Error: %q", machine.Name, err)
				return machine, err
			}
			return clone, nil
		}
		// Cannot update node status as node doesn't exist
		// Hence returning
		return machine, nil
	} else if err != nil {
		// Any other types of errors while fetching node object
		glog.Errorf("Could not fetch node object for machine %s", machine.Name)
		return machine, err
	}

	machine, err = c.updateMachineConditions(machine, node.Status.Conditions)
	if err != nil {
		return machine, err
	}

	clone := machine.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}

	if _, ok := clone.Labels["node"]; !ok {
		clone.Labels["node"] = machine.Status.Node
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			glog.Warningf("Machine update failed. Retrying, error: %s", err)
			return machine, err
		}
	}

	return machine, nil
}

/*
	SECTION
	Machine operations - Create, Update, Delete
*/

func (c *controller) machineCreate(machine *v1alpha1.Machine, driver driver.Driver) error {
	glog.V(2).Infof("Creating machine %q, please wait!", machine.Name)

	actualProviderID, nodeName, err := driver.Create()
	if err != nil {
		glog.Errorf("Error while creating machine %s: %s", machine.Name, err.Error())
		lastOperation := v1alpha1.LastOperation{
			Description:    "Cloud provider message - " + err.Error(),
			State:          v1alpha1.MachineStateFailed,
			Type:           v1alpha1.MachineOperationCreate,
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
	glog.V(2).Infof("Created machine: %q, MachineID: %s", machine.Name, actualProviderID)

	for {
		machineName := machine.Name
		// Get the latest version of the machine so that we can avoid conflicts
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
		if err != nil {
			glog.Warningf("Machine GET failed for %q. Retrying, error: %s", machineName, err)
			continue
		}

		lastOperation := v1alpha1.LastOperation{
			Description:    "Creating machine on cloud provider",
			State:          v1alpha1.MachineStateProcessing,
			Type:           v1alpha1.MachineOperationCreate,
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
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			glog.Warningf("Machine UPDATE failed for %q. Retrying, error: %s", machineName, err)
			continue
		}

		clone = machine.DeepCopy()
		clone.Status.Node = nodeName
		clone.Status.LastOperation = lastOperation
		clone.Status.CurrentStatus = currentStatus
		_, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
		if err != nil {
			glog.Warningf("Machine/status UPDATE failed for %q. Retrying, error: %s", machineName, err)
			continue
		}
		// Update went through, exit out of infinite loop
		break
	}

	return nil
}

func (c *controller) machineUpdate(machine *v1alpha1.Machine, actualProviderID string) error {
	glog.V(2).Infof("Setting MachineId of %s to %s", machine.Name, actualProviderID)

	for {
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
		if err != nil {
			glog.Errorf("Could not fetch machine object while setting up MachineId %s for Machine %s due to error %s", actualProviderID, machine.Name, err)
			return err
		}

		clone := machine.DeepCopy()
		clone.Spec.ProviderID = actualProviderID
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			glog.Warningf("Machine update failed. Retrying, error: %s", err)
			continue
		}

		clone = machine.DeepCopy()
		lastOperation := v1alpha1.LastOperation{
			Description:    "Updated provider ID",
			State:          v1alpha1.MachineStateSuccessful,
			Type:           v1alpha1.MachineOperationUpdate,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.LastOperation = lastOperation
		_, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
		if err != nil {
			glog.Warningf("Machine/status update failed. Retrying, error: %s", err)
			continue
		}
		// Update went through, exit out of infinite loop
		break
	}

	return nil
}

func (c *controller) machineDelete(machine *v1alpha1.Machine, driver driver.Driver) error {
	var err error
	nodeName := machine.Status.Node

	if finalizers := sets.NewString(machine.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		glog.V(2).Infof("Deleting Machine %q", machine.Name)

		// If machine was created on the cloud provider
		machineID, _ := driver.GetExisting()

		if machine.Status.CurrentStatus.Phase != v1alpha1.MachineTerminating {
			lastOperation := v1alpha1.LastOperation{
				Description:    "Deleting machine from cloud provider",
				State:          v1alpha1.MachineStateProcessing,
				Type:           v1alpha1.MachineOperationDelete,
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
			var (
				forceDeletePods         = false
				forceDeleteMachine      = false
				timeOutOccurred         = false
				maxEvictRetries         = c.safetyOptions.MaxEvictRetries
				pvDetachTimeOut         = c.safetyOptions.PvDetachTimeout.Duration
				timeOutDuration         = c.safetyOptions.MachineDrainTimeout.Duration
				forceDeleteLabelPresent = machine.Labels["force-deletion"] == "True"
				lastDrainFailed         = machine.Status.LastOperation.Description[0:12] == "Drain failed"
			)

			// Timeout value obtained by subtracting last operation with expected time out period
			timeOut := metav1.Now().Add(-timeOutDuration).Sub(machine.Status.CurrentStatus.LastUpdateTime.Time)
			timeOutOccurred = timeOut > 0

			if forceDeleteLabelPresent || timeOutOccurred || lastDrainFailed {
				// To perform forceful machine drain/delete either one of the below conditions must be satified
				// 1. force-deletion: "True" label must be present
				// 2. Deletion operation is more than drain-timeout minutes old
				// 3. Last machine drain had failed
				forceDeleteMachine = true
				forceDeletePods = true
				timeOutDuration = 1 * time.Minute
				maxEvictRetries = 1

				glog.V(2).Infof(
					"Force deletion has been triggerred for machine %q due to Label:%t, timeout:%t, lastDrainFailed:%t",
					machine.Name,
					forceDeleteLabelPresent,
					timeOutOccurred,
					lastDrainFailed,
				)
			}

			buf := bytes.NewBuffer([]byte{})
			errBuf := bytes.NewBuffer([]byte{})

			drainOptions := NewDrainOptions(
				c.targetCoreClient,
				timeOutDuration,
				maxEvictRetries,
				pvDetachTimeOut,
				nodeName,
				-1,
				forceDeletePods,
				true,
				true,
				true,
				buf,
				errBuf,
				driver,
				c.pvcLister,
				c.pvLister,
			)
			err = drainOptions.RunDrain()
			if err == nil {
				// Drain successful
				glog.V(2).Infof("Drain successful for machine %q. \nBuf:%v \nErrBuf:%v", machine.Name, buf, errBuf)

			} else if err != nil && forceDeleteMachine {
				// Drain failed on force deletion
				glog.Warningf("Drain failed for machine %q. However, since it's a force deletion shall continue deletion of VM. \nBuf:%v \nErrBuf:%v \nErr-Message:%v", machine.Name, buf, errBuf, err)

			} else {
				// Drain failed on normal (non-force) deletion, return error for retry
				lastOperation := v1alpha1.LastOperation{
					Description:    "Drain failed - " + err.Error(),
					State:          v1alpha1.MachineStateFailed,
					Type:           v1alpha1.MachineOperationDelete,
					LastUpdateTime: metav1.Now(),
				}
				c.updateMachineStatus(machine, lastOperation, machine.Status.CurrentStatus)

				glog.Warningf("Drain failed for machine %q. \nBuf:%v \nErrBuf:%v \nErr-Message:%v", machine.Name, buf, errBuf, err)
				return err
			}

			err = driver.Delete()
		}

		if err != nil {
			// When machine deletion fails
			glog.Errorf("Deletion failed for machine %q: %s", machine.Name, err)

			lastOperation := v1alpha1.LastOperation{
				Description:    "Cloud provider message - " + err.Error(),
				State:          v1alpha1.MachineStateFailed,
				Type:           v1alpha1.MachineOperationDelete,
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

		if nodeName != "" {
			// Delete node object
			err = c.targetCoreClient.CoreV1().Nodes().Delete(nodeName, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				// If its an error, and anyother error than object not found
				message := fmt.Sprintf("Deletion of Node Object %q failed due to error: %s", nodeName, err)
				lastOperation := v1alpha1.LastOperation{
					Description:    message,
					State:          v1alpha1.MachineStateFailed,
					Type:           v1alpha1.MachineOperationDelete,
					LastUpdateTime: metav1.Now(),
				}
				c.updateMachineStatus(machine, lastOperation, machine.Status.CurrentStatus)
				glog.Errorf(message)
				return err
			}
		}

		// Remove finalizers from machine object
		c.deleteMachineFinalizers(machine)

		// Delete machine object
		err = c.controlMachineClient.Machines(machine.Namespace).Delete(machine.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// If its an error, and anyother error than object not found
			glog.Errorf("Deletion of Machine Object %q failed due to error: %s", machine.Name, err)
			return err
		}

		glog.V(2).Infof("Machine %q deleted successfully", machine.Name)
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

	clone, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(3).Infof("Warning: Updated failed, retrying, error: %q", err)
		return c.updateMachineStatus(machine, lastOperation, currentStatus)
	}
	return clone, nil
}

func (c *controller) updateMachineConditions(machine *v1alpha1.Machine, conditions []v1.NodeCondition) (*v1alpha1.Machine, error) {

	var (
		msg                  string
		lastOperationType    v1alpha1.MachineOperationType
		objectRequiresUpdate bool
	)

	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return machine, err
	}

	clone := machine.DeepCopy()

	if nodeConditionsHaveChanged(clone.Status.Conditions, conditions) {
		clone.Status.Conditions = conditions
		objectRequiresUpdate = true
	}

	if clone.Status.CurrentStatus.Phase == v1alpha1.MachineTerminating {
		// If machine is already in terminating state, don't update health status

	} else if !c.isHealthy(clone) && clone.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
		// If machine is not healthy, and current state is running,
		// change the machinePhase to unknown and activate health check timeout
		msg = fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", clone.Name)
		glog.Warning(msg)

		clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineUnknown,
			TimeoutActive:  true,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.LastOperation = v1alpha1.LastOperation{
			Description:    msg,
			State:          v1alpha1.MachineStateProcessing,
			Type:           v1alpha1.MachineOperationHealthCheck,
			LastUpdateTime: metav1.Now(),
		}
		objectRequiresUpdate = true

	} else if c.isHealthy(clone) && clone.Status.CurrentStatus.Phase != v1alpha1.MachineRunning {
		// If machine is healhy and current machinePhase is not running.
		// indicates that the machine is not healthy and status needs to be updated.

		if clone.Status.LastOperation.Type == v1alpha1.MachineOperationCreate &&
			clone.Status.LastOperation.State != v1alpha1.MachineStateSuccessful {
			// When machine creation went through
			msg = fmt.Sprintf("Machine %s successfully joined the cluster", clone.Name)
			lastOperationType = v1alpha1.MachineOperationCreate
		} else {
			// Machine rejoined the cluster after a healthcheck
			msg = fmt.Sprintf("Machine %s successfully re-joined the cluster", clone.Name)
			lastOperationType = v1alpha1.MachineOperationHealthCheck
		}
		glog.V(2).Infof(msg)

		// Machine is ready and has joined/re-joined the cluster
		clone.Status.LastOperation = v1alpha1.LastOperation{
			Description:    msg,
			State:          v1alpha1.MachineStateSuccessful,
			Type:           lastOperationType,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineRunning,
			TimeoutActive:  false,
			LastUpdateTime: metav1.Now(),
		}
		objectRequiresUpdate = true

	}

	if objectRequiresUpdate {
		clone, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
		if err != nil {
			// Keep retrying until update goes through
			glog.Warningf("Updated failed, retrying, error: %q", err)
			return c.updateMachineConditions(machine, conditions)
		}

		return clone, nil
	}

	return machine, nil
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
		glog.Warningf("Warning: Updated failed, retrying, error: %q", err)
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
		// Kubernetes node object for this machine hasn't been received
		return false
	}

	for _, condition := range machine.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
			// If Kubelet is not ready
			return false
		}
		conditions := strings.Split(c.nodeConditions, ",")
		for _, c := range conditions {
			if string(condition.Type) == c && condition.Status != v1.ConditionFalse {
				return false
			}
		}
	}
	return true
}

func (c *controller) getSecret(ref *v1.SecretReference, MachineClassName string) (*v1.Secret, error) {
	secretRef, err := c.secretLister.Secrets(ref.Namespace).Get(ref.Name)
	if apierrors.IsNotFound(err) {
		glog.V(3).Infof("No secret %q: found for MachineClass %q", ref, MachineClassName)
		return nil, nil
	}
	if err != nil {
		glog.Errorf("Unable get secret %q for MachineClass %q: %v", MachineClassName, ref, err)
		return nil, err
	}
	return secretRef, err
}

func (c *controller) checkMachineTimeout(machine *v1alpha1.Machine) {

	// If machine phase is running already, ignore this loop
	if machine.Status.CurrentStatus.Phase != v1alpha1.MachineRunning {

		var (
			description     string
			lastOperation   v1alpha1.LastOperation
			currentStatus   v1alpha1.CurrentStatus
			timeOutDuration time.Duration
		)

		checkCreationTimeout := machine.Status.CurrentStatus.Phase == v1alpha1.MachinePending
		sleepTime := 1 * time.Minute

		if checkCreationTimeout {
			timeOutDuration = c.safetyOptions.MachineCreationTimeout.Duration
		} else {
			timeOutDuration = c.safetyOptions.MachineHealthTimeout.Duration
		}

		// Timeout value obtained by subtracting last operation with expected time out period
		timeOut := metav1.Now().Add(-timeOutDuration).Sub(machine.Status.CurrentStatus.LastUpdateTime.Time)
		if timeOut > 0 {
			// Machine health timeout occurs while joining or rejoining of machine

			if checkCreationTimeout {
				// Timeout occurred while machine creation
				description = fmt.Sprintf(
					"Machine %s failed to join the cluster in %s minutes.",
					machine.Name,
					timeOutDuration,
				)
			} else {
				// Timeour occurred due to machine being unhealthy for too long
				description = fmt.Sprintf(
					"Machine %s is not healthy since %s minutes. Changing status to failed. Node Conditions: %+v",
					machine.Name,
					timeOutDuration,
					machine.Status.Conditions,
				)
			}

			lastOperation = v1alpha1.LastOperation{
				Description:    description,
				State:          v1alpha1.MachineStateFailed,
				Type:           machine.Status.LastOperation.Type,
				LastUpdateTime: metav1.Now(),
			}
			currentStatus = v1alpha1.CurrentStatus{
				Phase:          v1alpha1.MachineFailed,
				TimeoutActive:  false,
				LastUpdateTime: metav1.Now(),
			}
			// Log the error message for machine failure
			glog.Error(description)

			// Update the machine status to reflect the changes
			c.updateMachineStatus(machine, lastOperation, currentStatus)

		} else {
			// If timeout has not occurred, re-enqueue the machine
			// after a specified sleep time
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
