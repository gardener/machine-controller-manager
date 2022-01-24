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
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/gardener/machine-controller-manager/pkg/driver"
	utiltime "github.com/gardener/machine-controller-manager/pkg/util/time"
)

const (
	// MachinePriority is the annotation used to specify priority
	// associated with a machine while deleting it. The less its
	// priority the more likely it is to be deleted first
	// Default priority for a machine is set to 3
	MachinePriority = "machinepriority.machine.sapcloud.io"

	// MachineEnqueueRetryPeriod period after which a machine object is re-enqueued upon creation or deletion failures.
	MachineEnqueueRetryPeriod = 2 * time.Minute
)

/*
	SECTION
	Machine controller - Machine add, update, delete watches
*/
func (c *controller) addMachine(obj interface{}) {
	klog.V(4).Infof("Adding machine object")
	c.enqueueMachine(obj)
}

func (c *controller) updateMachine(oldObj, newObj interface{}) {
	klog.V(4).Info("Updating machine object")
	c.enqueueMachine(newObj)
}

func (c *controller) deleteMachine(obj interface{}) {
	klog.V(4).Info("Deleting machine object")
	c.enqueueMachine(obj)
}

func (c *controller) enqueueMachine(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	machine := obj.(*v1alpha1.Machine)
	switch machine.Spec.Class.Kind {
	case AlicloudMachineClassKind, AWSMachineClassKind, AzureMachineClassKind, GCPMachineClassKind, OpenStackMachineClassKind, PacketMachineClassKind:
		// Checking if machineClass is to be processed by MCM, and then only enqueue the machine object
		klog.V(4).Infof("Adding machine object to the queue %q", key)
		c.machineQueue.Add(key)
	default:
		klog.V(4).Infof("ClassKind %q not found. Machine maybe be processed by external controller", machine.Spec.Class.Kind)
	}
}

func (c *controller) enqueueMachineAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	machine := obj.(*v1alpha1.Machine)

	switch machine.Spec.Class.Kind {
	case AlicloudMachineClassKind, AWSMachineClassKind, AzureMachineClassKind, GCPMachineClassKind, OpenStackMachineClassKind, PacketMachineClassKind:
		// Checking if machineClass is to be processed by MCM, and then only enqueue the machine object
		klog.V(4).Infof("Adding machine object to the queue %q after %s", key, after)
		c.machineQueue.AddAfter(key, after)
	default:
		klog.V(4).Infof("ClassKind %q not found. Machine maybe be processed by external controller", machine.Spec.Class.Kind)
	}
}

func (c *controller) reconcileClusterMachineKey(key string) error {
	ctx := context.Background()
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	machine, err := c.machineLister.Machines(c.namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(4).Infof("Machine %q: Not doing work because it is not found", key)
		return nil
	}

	if err != nil {
		klog.Errorf("ClusterMachine %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterMachine(ctx, machine)
}

func (c *controller) reconcileClusterMachine(ctx context.Context, machine *v1alpha1.Machine) error {
	klog.V(4).Info("Start Reconciling machine: ", machine.Name)
	defer func() {
		c.enqueueMachineAfter(machine, 10*time.Minute)
		klog.V(4).Info("Stop Reconciling machine: ", machine.Name)
	}()

	if c.safetyOptions.MachineControllerFrozen && machine.DeletionTimestamp == nil {
		message := "Machine controller has frozen. Retrying reconcile after 10 minutes"
		klog.V(3).Info(message)
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
		klog.Errorf("Validation of Machine failed %s", validationerr.ToAggregate().Error())
		return nil
	}

	// Validate MachineClass
	MachineClass, secretData, err := c.validateMachineClass(&machine.Spec.Class)
	if err != nil || secretData == nil {
		c.enqueueMachineAfter(machine, MachineEnqueueRetryPeriod)
		return nil
	}

	driver := driver.NewDriver(machine.Spec.ProviderID, secretData, machine.Spec.Class.Kind, MachineClass, machine.Name)
	actualProviderID, err := driver.GetExisting()
	if err != nil {
		return err
	} else if actualProviderID == "fake" {
		klog.Warning("Fake driver type")
		return nil
	}

	machine, err = c.machineLister.Machines(machine.Namespace).Get(machine.Name)
	if err != nil {
		klog.Errorf("Could not fetch machine object %s", err)
		if apierrors.IsNotFound(err) {
			// Ignore the error "Not Found"
			return nil
		}

		return err
	}

	machine, err = c.updateMachineState(ctx, machine)
	if err != nil {
		klog.Errorf("Could not update machine state for: %s", machine.Name)
		return err
	}

	// Sync nodeTemplate between machine and node-objects.
	node, _ := c.nodeLister.Get(machine.Status.Node)
	if node != nil {
		err = c.syncMachineNodeTemplates(ctx, machine)
		if err != nil {
			klog.Errorf("Could not update nodeTemplate for machine %s err: %q", machine.Name, err)
			return err
		}
	}

	if machine.DeletionTimestamp != nil {
		// Processing of delete event
		if err := c.machineDelete(ctx, machine, driver); err != nil {
			c.enqueueMachineAfter(machine, MachineEnqueueRetryPeriod)
			return nil
		}
	} else if machine.Status.CurrentStatus.TimeoutActive {
		// Processing machine
		c.checkMachineTimeout(ctx, machine)
	} else {
		// Processing of create or update event
		c.addMachineFinalizers(ctx, machine)

		if machine.Status.CurrentStatus.Phase == v1alpha1.MachineFailed {
			return nil
		} else if actualProviderID == "" {
			if err := c.machineCreate(ctx, machine, driver); err != nil {
				c.enqueueMachineAfter(machine, MachineEnqueueRetryPeriod)
				return nil
			}
		} else if actualProviderID != machine.Spec.ProviderID {
			if err := c.machineUpdate(ctx, machine, actualProviderID); err != nil {
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
	node := obj.(*corev1.Node)
	if node == nil {
		klog.Errorf("Couldn't convert to node from object")
		return
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	machine, err := c.getMachineFromNode(key)
	if err != nil {
		klog.Errorf("Couldn't fetch machine %s, Error: %s", key, err)
		return
	} else if machine == nil {
		return
	}

	if machine.Status.CurrentStatus.Phase != v1alpha1.MachineCrashLoopBackOff && nodeConditionsHaveChanged(machine.Status.Conditions, node.Status.Conditions) {
		klog.V(4).Infof("Enqueue machine object %q as backing node's conditions have changed", machine.Name)
		c.enqueueMachine(machine)
	}
}

func (c *controller) updateNodeToMachine(oldObj, newObj interface{}) {
	c.addNodeToMachine(newObj)
}

func (c *controller) deleteNodeToMachine(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	machine, err := c.getMachineFromNode(key)
	if err != nil {
		klog.Errorf("Couldn't fetch machine %s, Error: %s", key, err)
		return
	} else if machine == nil {
		return
	}

	c.enqueueMachine(machine)
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

func (c *controller) updateMachineState(ctx context.Context, machine *v1alpha1.Machine) (*v1alpha1.Machine, error) {
	nodeName := machine.Status.Node

	if nodeName == "" {
		// Check if any existing node-object can be adopted.
		nodeList, err := c.nodeLister.List(labels.Everything())
		if err != nil {
			klog.Errorf("Could not list the nodes due to error: %v", err)
			return machine, err
		}
		for _, node := range nodeList {
			nID, mID := decodeMachineID(node.Spec.ProviderID), decodeMachineID(machine.Spec.ProviderID)
			if nID == "" {
				continue
			}

			if nID == mID {
				klog.V(2).Infof("Adopting the node object %s for machine %s", node.Name, machine.Name)
				nodeName = node.Name
				clone := machine.DeepCopy()
				clone.Status.Node = nodeName
				clone, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
				if err != nil {
					klog.Errorf("Could not update status of the machine-object %s due to error %v", machine.Name, err)
					return machine, err
				}
				break
			}
		}
		// Couldnt adopt any node-object.
		if nodeName == "" {
			// There are no objects mapped to this machine object
			// Hence node status need not be propogated to machine object
			return machine, nil
		}
	}

	node, err := c.nodeLister.Get(nodeName)
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
			klog.Warning(msg)

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
			clone, err := c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)
			if err != nil {
				klog.Errorf("Machine updated failed for %s, Error: %q", machine.Name, err)
				return machine, err
			}
			return clone, nil
		}
		// Cannot update node status as node doesn't exist
		// Hence returning
		return machine, nil
	} else if err != nil {
		// Any other types of errors while fetching node object
		klog.Errorf("Could not fetch node object for machine %s", machine.Name)
		return machine, err
	}

	machine, err = c.updateMachineConditions(ctx, machine, node.Status.Conditions)
	if err != nil {
		return machine, err
	}

	clone := machine.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}

	if n := clone.Labels["node"]; n == "" {
		clone.Labels["node"] = machine.Status.Node
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine update failed. Retrying, error: %s", err)
			return machine, err
		}
	}

	return machine, nil
}

/*
	SECTION
	Machine operations - Create, Update, Delete
*/

func (c *controller) machineCreate(ctx context.Context, machine *v1alpha1.Machine, driver driver.Driver) error {
	klog.V(2).Infof("Creating machine %q, please wait!", machine.Name)
	var actualProviderID, nodeName string

	err := c.addBootstrapTokenToUserData(ctx, machine.Name, driver)
	if err != nil {
		klog.Errorf("Error while creating bootstrap token for machine %s: %s", machine.Name, err.Error())
		lastOperation := v1alpha1.LastOperation{
			Description:    "MCM message - " + err.Error(),
			State:          v1alpha1.MachineStateFailed,
			Type:           v1alpha1.MachineOperationCreate,
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineFailed,
			TimeoutActive:  false,
			LastUpdateTime: metav1.Now(),
		}
		c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)
		return err
	}
	// Before actually creating the machine, we should once check and adopt if the virtual machine already exists.
	VMList, err := driver.GetVMs("")
	if err != nil {

		klog.Errorf("Error while listing machine %s: %s", machine.Name, err.Error())
		lastOperation := v1alpha1.LastOperation{
			Description:    "Cloud provider message - " + err.Error(),
			State:          v1alpha1.MachineStateFailed,
			Type:           v1alpha1.MachineOperationCreate,
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineCrashLoopBackOff,
			TimeoutActive:  false,
			LastUpdateTime: metav1.Now(),
		}
		c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)

		// Delete the bootstrap token
		if err := c.deleteBootstrapToken(ctx, machine.Name); err != nil {
			klog.Warning(err)
		}

		klog.Errorf("Failed to list VMs before creating machine %q %+v", machine.Name, err)
		return err
	}
	for providerID, machineName := range VMList {
		if machineName == machine.Name {
			klog.V(2).Infof("Adopted an existing VM %s for machine object %s.", providerID, machineName)
			actualProviderID = providerID
		}
	}
	if actualProviderID == "" {
		actualProviderID, nodeName, err = driver.Create()
	}

	if err != nil {
		klog.Errorf("Error while creating machine %s: %s", machine.Name, err.Error())
		lastOperation := v1alpha1.LastOperation{
			Description:    "Cloud provider message - " + err.Error(),
			State:          v1alpha1.MachineStateFailed,
			Type:           v1alpha1.MachineOperationCreate,
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachineCrashLoopBackOff,
			TimeoutActive:  false,
			LastUpdateTime: metav1.Now(),
		}
		c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)

		// Delete the bootstrap token
		if err := c.deleteBootstrapToken(ctx, machine.Name); err != nil {
			klog.Warning(err)
		}

		return err
	}
	klog.V(2).Infof("Created/Adopted machine: %q, MachineID: %s", machine.Name, actualProviderID)

	for {
		machineName := machine.Name
		// Get the latest version of the machine so that we can avoid conflicts
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(ctx, machine.Name, metav1.GetOptions{})
		if err != nil {

			if apierrors.IsNotFound(err) {
				klog.Infof("Machine %q not found on APIServer anymore. Deleting created (orphan) VM", machineName)

				if err = driver.Delete(actualProviderID); err != nil {
					klog.Errorf(
						"Deletion failed for orphan machine %q with provider-ID %q: %s",
						machine.Name,
						actualProviderID,
						err,
					)
				}

				// Return with error
				return fmt.Errorf("Couldn't find machine object, hence deleted orphan VM")
			}

			klog.Warningf("Machine GET failed for %q. Retrying, error: %s", machineName, err)
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
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine UPDATE failed for %q. Retrying, error: %s", machineName, err)
			continue
		}

		clone = machine.DeepCopy()
		clone.Status.Node = nodeName
		clone.Status.LastOperation = lastOperation
		clone.Status.CurrentStatus = currentStatus
		_, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine/status UPDATE failed for %q. Retrying, error: %s", machineName, err)
			continue
		}
		// Update went through, exit out of infinite loop
		break
	}

	return nil
}

func (c *controller) machineUpdate(ctx context.Context, machine *v1alpha1.Machine, actualProviderID string) error {
	klog.V(2).Infof("Setting MachineId of %s to %s", machine.Name, actualProviderID)

	for {
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(ctx, machine.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Could not fetch machine object while setting up MachineId %s for Machine %s due to error %s", actualProviderID, machine.Name, err)
			return err
		}

		clone := machine.DeepCopy()
		clone.Spec.ProviderID = actualProviderID
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine update failed. Retrying, error: %s", err)
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
		_, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine/status update failed. Retrying, error: %s", err)
			continue
		}
		// Update went through, exit out of infinite loop
		break
	}

	return nil
}

func (c *controller) machineDelete(ctx context.Context, machine *v1alpha1.Machine, driver driver.Driver) error {
	var err error
	nodeName := machine.Status.Node

	if finalizers := sets.NewString(machine.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		klog.V(2).Infof("Deleting Machine %q", machine.Name)
		var (
			forceDeletePods         = false
			forceDeleteMachine      = false
			timeOutOccurred         = false
			maxEvictRetries         = int32(math.Min(float64(*c.getEffectiveMaxEvictRetries(machine)), c.getEffectiveDrainTimeout(machine).Seconds()/PodEvictionRetryInterval.Seconds()))
			pvDetachTimeOut         = c.safetyOptions.PvDetachTimeout.Duration
			timeOutDuration         = c.getEffectiveDrainTimeout(machine).Duration
			forceDeleteLabelPresent = machine.Labels["force-deletion"] == "True"
		)

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

			err = c.deleteBootstrapToken(ctx, machine.Name)
			if err != nil {
				klog.Warning(err)
			}

			machine, err = c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)
			if err != nil && apierrors.IsNotFound(err) {
				// Object no longer exists and has been deleted
				klog.Warning(err)
				return nil
			} else if err != nil {
				// Any other type of errors
				klog.Error(err)
				return err
			}
		}

		timeOutOccurred = utiltime.HasTimeOutOccurred(*machine.DeletionTimestamp, timeOutDuration)

		if forceDeleteLabelPresent || timeOutOccurred {
			// To perform forceful machine drain/delete either one of the below conditions must be satified
			// 1. force-deletion: "True" label must be present
			// 2. Deletion operation is more than drain-timeout minutes old
			forceDeleteMachine = true
			forceDeletePods = true
			timeOutDuration = 1 * time.Minute
			maxEvictRetries = 1

			klog.V(2).Infof(
				"Force delete/drain has been triggerred for machine %q due to Label:%t, timeout:%t",
				machine.Name,
				forceDeleteLabelPresent,
				timeOutOccurred,
			)
		} else {
			klog.V(2).Infof(
				"Normal delete/drain has been triggerred for machine %q with drain-timeout:%v & maxEvictRetries:%d",
				machine.Name,
				timeOutDuration,
				maxEvictRetries,
			)
		}

		// If machine was created on the cloud provider
		machineID, _ := driver.GetExisting()

		// update node with the machine's state prior to termination
		if nodeName != "" && machineID != "" {
			if err = c.UpdateNodeTerminationCondition(ctx, machine); err != nil {
				if forceDeleteMachine {
					klog.Warningf("failed to update node conditions: %v. However, since it's a force deletion shall continue deletion of VM.", err)
				} else {
					klog.Error(err)
					return err
				}
			}
		}

		if machineID != "" && nodeName != "" {
			// Begin drain logic only when the nodeName & providerID exist's for the machine
			buf := bytes.NewBuffer([]byte{})
			errBuf := bytes.NewBuffer([]byte{})

			drainOptions := NewDrainOptions(
				c.targetCoreClient,
				timeOutDuration,
				maxEvictRetries,
				pvDetachTimeOut,
				nodeName,
				5*time.Minute,
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
			err = drainOptions.RunDrain(ctx)
			if err == nil {
				// Drain successful
				klog.V(2).Infof("Drain successful for machine %q. \nBuf:%v \nErrBuf:%v", machine.Name, buf, errBuf)

			} else if err != nil && forceDeleteMachine {
				// Drain failed on force deletion
				klog.Warningf("Drain failed for machine %q. However, since it's a force deletion shall continue deletion of VM. \nBuf:%v \nErrBuf:%v \nErr-Message:%v", machine.Name, buf, errBuf, err)

			} else {
				// Drain failed on normal (non-force) deletion, return error for retry
				lastOperation := v1alpha1.LastOperation{
					Description:    "Drain failed - " + err.Error(),
					State:          v1alpha1.MachineStateFailed,
					Type:           v1alpha1.MachineOperationDelete,
					LastUpdateTime: metav1.Now(),
				}
				c.updateMachineStatus(ctx, machine, lastOperation, machine.Status.CurrentStatus)

				klog.Warningf("Drain failed for machine %q. \nBuf:%v \nErrBuf:%v \nErr-Message:%v", machine.Name, buf, errBuf, err)
				return err
			}

			err = driver.Delete(machineID)
		} else {
			klog.V(2).Infof("Machine %q on deletion doesn't have a providerID attached to it. Checking for any VM linked to this machine object.", machine.Name)
			// As MachineID is missing, we should check once if actual VM was created but MachineID was not updated on machine-object.
			// We list VMs and check if any one them map with the given machine-object.
			var VMList map[string]string
			VMList, err = driver.GetVMs("")
			if err == nil {
				for providerID, machineName := range VMList {
					if machineName == machine.Name {
						klog.V(2).Infof("Deleting the VM %s backing the machine-object %s.", providerID, machine.Name)
						err = driver.Delete(providerID)
						if err != nil {
							klog.Errorf("Error deleting the VM %s backing the machine-object %s due to error %v", providerID, machine.Name, err)
							// Not returning error so that status on the machine object can be updated in the next step if errored.
						} else {
							klog.V(2).Infof("VM %s backing the machine-object %s is deleted successfully", providerID, machine.Name)
						}
					}
				}
			} else {
				klog.Errorf("Failed to list VMs while deleting the machine %q %v", machine.Name, err)
			}
		}

		if err != nil {
			// When machine deletion fails
			klog.Errorf("Deletion failed for machine %q: %s", machine.Name, err)

			lastOperation := v1alpha1.LastOperation{
				Description:    "Cloud provider message - " + err.Error(),
				State:          v1alpha1.MachineStateFailed,
				Type:           v1alpha1.MachineOperationDelete,
				LastUpdateTime: metav1.Now(),
			}
			currentStatus := v1alpha1.CurrentStatus{
				Phase:          v1alpha1.MachineTerminating,
				TimeoutActive:  false,
				LastUpdateTime: metav1.Now(),
			}
			c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)
			return err
		}

		if nodeName != "" {
			// Delete node object
			err = c.targetCoreClient.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				// If its an error, and anyother error than object not found
				message := fmt.Sprintf("Deletion of Node Object %q failed due to error: %s", nodeName, err)
				lastOperation := v1alpha1.LastOperation{
					Description:    message,
					State:          v1alpha1.MachineStateFailed,
					Type:           v1alpha1.MachineOperationDelete,
					LastUpdateTime: metav1.Now(),
				}
				c.updateMachineStatus(ctx, machine, lastOperation, machine.Status.CurrentStatus)
				klog.Errorf(message)
				return err
			}
		}

		// Remove finalizers from machine object
		c.deleteMachineFinalizers(ctx, machine)

		// Delete machine object
		err = c.controlMachineClient.Machines(machine.Namespace).Delete(ctx, machine.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// If its an error, and anyother error than object not found
			klog.Errorf("Deletion of Machine Object %q failed due to error: %s", machine.Name, err)
			return err
		}

		klog.V(2).Infof("Machine %q deleted successfully", machine.Name)
	}
	return nil
}

/*
	SECTION
	Update machine object
*/

func (c *controller) updateMachineStatus(
	ctx context.Context,
	machine *v1alpha1.Machine,
	lastOperation v1alpha1.LastOperation,
	currentStatus v1alpha1.CurrentStatus,
) (*v1alpha1.Machine, error) {
	// Get the latest version of the machine so that we can avoid conflicts
	latestMachine, err := c.controlMachineClient.Machines(machine.Namespace).Get(ctx, machine.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	clone := latestMachine.DeepCopy()

	clone.Status.LastOperation = lastOperation
	clone.Status.CurrentStatus = currentStatus
	if isMachineStatusEqual(clone.Status, machine.Status) {
		klog.V(3).Infof("Not updating the status of the machine object %q , as it is already same", clone.Name)
		return machine, nil
	}

	clone, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		// Keep retrying until update goes through
		klog.V(3).Infof("Warning: Updated failed, retrying, error: %q", err)
		return c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)
	}
	return clone, nil
}

// isMachineStatusEqual checks if the status of 2 machines is similar or not.
func isMachineStatusEqual(s1, s2 v1alpha1.MachineStatus) bool {
	tolerateTimeDiff := 30 * time.Minute
	s1Copy, s2Copy := s1.DeepCopy(), s2.DeepCopy()
	s1Copy.LastOperation.Description, s2Copy.LastOperation.Description = "", ""

	if (s1Copy.LastOperation.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) || (s2Copy.LastOperation.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) {
		return false
	}
	s1Copy.LastOperation.LastUpdateTime, s2Copy.LastOperation.LastUpdateTime = metav1.Time{}, metav1.Time{}

	if (s1Copy.CurrentStatus.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) || (s2Copy.CurrentStatus.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) {
		return false
	}
	s1Copy.CurrentStatus.LastUpdateTime, s2Copy.CurrentStatus.LastUpdateTime = metav1.Time{}, metav1.Time{}

	return apiequality.Semantic.DeepEqual(s1Copy.LastOperation, s2Copy.LastOperation) && apiequality.Semantic.DeepEqual(s1Copy.CurrentStatus, s2Copy.CurrentStatus)
}

func (c *controller) updateMachineConditions(ctx context.Context, machine *v1alpha1.Machine, conditions []v1.NodeCondition) (*v1alpha1.Machine, error) {

	var (
		msg                  string
		lastOperationType    v1alpha1.MachineOperationType
		objectRequiresUpdate bool
	)

	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(ctx, machine.Name, metav1.GetOptions{})
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
		msg = fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown. Node conditions: %+v", clone.Name, clone.Status.Conditions)
		klog.Warning(msg)

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
			// Delete the bootstrap token
			err = c.deleteBootstrapToken(ctx, clone.Name)
			if err != nil {
				klog.Warning(err)
			}

			lastOperationType = v1alpha1.MachineOperationCreate
		} else {
			// Machine rejoined the cluster after a healthcheck
			msg = fmt.Sprintf("Machine %s successfully re-joined the cluster", clone.Name)
			lastOperationType = v1alpha1.MachineOperationHealthCheck
		}
		klog.V(2).Infof(msg)

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
		clone, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			// Keep retrying until update goes through
			klog.Warningf("Updated failed, retrying, error: %q", err)
			return c.updateMachineConditions(ctx, machine, conditions)
		}

		return clone, nil
	}

	return machine, nil
}

func (c *controller) updateMachineFinalizers(ctx context.Context, machine *v1alpha1.Machine, finalizers []string) {
	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(ctx, machine.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := machine.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.Machines(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		// Keep retrying until update goes through
		klog.Warningf("Warning: Updated failed, retrying, error: %q", err)
		c.updateMachineFinalizers(ctx, machine, finalizers)
	}
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addMachineFinalizers(ctx context.Context, machine *v1alpha1.Machine) {
	clone := machine.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateMachineFinalizers(ctx, clone, finalizers.List())
	}
}

func (c *controller) deleteMachineFinalizers(ctx context.Context, machine *v1alpha1.Machine) {
	clone := machine.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateMachineFinalizers(ctx, clone, finalizers.List())
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
		conditions := strings.Split(*c.getEffectiveNodeConditions(machine), ",")
		for _, c := range conditions {
			if string(condition.Type) == c && condition.Status != v1.ConditionFalse {
				return false
			}
		}
	}
	return true
}

func (c *controller) getSecret(ref *v1.SecretReference, machineClassName string) (*v1.Secret, error) {
	secretRef, err := c.secretLister.Secrets(ref.Namespace).Get(ref.Name)
	if apierrors.IsNotFound(err) {
		klog.V(3).Infof("No secret %q: found for MachineClass %q", ref, machineClassName)
		return nil, err
	}
	if err != nil {
		klog.Errorf("Unable get secret %q for MachineClass %q: %v", ref, machineClassName, err)
		return nil, err
	}
	return secretRef, err
}

func (c *controller) checkMachineTimeout(ctx context.Context, machine *v1alpha1.Machine) {

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
			timeOutDuration = c.getEffectiveCreationTimeout(machine).Duration
		} else {
			timeOutDuration = c.getEffectiveHealthTimeout(machine).Duration
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
			klog.Error(description)

			// Update the machine status to reflect the changes
			c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)

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

// decodeMachineID is a generic way of decoding the Spec.ProviderID field of node-objects.
func decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}

// getEffectiveDrainTimeout returns the drainTimeout set on the machine-object, otherwise returns the timeout set using the global-flag.
func (c *controller) getEffectiveDrainTimeout(machine *v1alpha1.Machine) *metav1.Duration {
	var effectiveDrainTimeout *metav1.Duration
	if machine.Spec.MachineConfiguration != nil && machine.Spec.MachineConfiguration.MachineDrainTimeout != nil {
		effectiveDrainTimeout = machine.Spec.MachineConfiguration.MachineDrainTimeout
	} else {
		effectiveDrainTimeout = &c.safetyOptions.MachineDrainTimeout
	}
	return effectiveDrainTimeout
}

// getEffectiveMaxEvictRetries returns the maxEvictRetries set on the machine-object, otherwise returns the evict retries set using the global-flag.
func (c *controller) getEffectiveMaxEvictRetries(machine *v1alpha1.Machine) *int32 {
	var maxEvictRetries *int32
	if machine.Spec.MachineConfiguration != nil && machine.Spec.MachineConfiguration.MaxEvictRetries != nil {
		maxEvictRetries = machine.Spec.MachineConfiguration.MaxEvictRetries
	} else {
		maxEvictRetries = &c.safetyOptions.MaxEvictRetries
	}
	return maxEvictRetries
}

// getEffectiveHealthTimeout returns the healthTimeout set on the machine-object, otherwise returns the timeout set using the global-flag.
func (c *controller) getEffectiveHealthTimeout(machine *v1alpha1.Machine) *metav1.Duration {
	var effectiveHealthTimeout *metav1.Duration
	if machine.Spec.MachineConfiguration != nil && machine.Spec.MachineConfiguration.MachineHealthTimeout != nil {
		effectiveHealthTimeout = machine.Spec.MachineConfiguration.MachineHealthTimeout
	} else {
		effectiveHealthTimeout = &c.safetyOptions.MachineHealthTimeout
	}
	return effectiveHealthTimeout
}

// getEffectiveHealthTimeout returns the creationTimeout set on the machine-object, otherwise returns the timeout set using the global-flag.
func (c *controller) getEffectiveCreationTimeout(machine *v1alpha1.Machine) *metav1.Duration {
	var effectiveCreationTimeout *metav1.Duration
	if machine.Spec.MachineConfiguration != nil && machine.Spec.MachineConfiguration.MachineCreationTimeout != nil {
		effectiveCreationTimeout = machine.Spec.MachineConfiguration.MachineCreationTimeout
	} else {
		effectiveCreationTimeout = &c.safetyOptions.MachineCreationTimeout
	}
	return effectiveCreationTimeout
}

// getEffectiveNodeConditions returns the nodeConditions set on the machine-object, otherwise returns the conditions set using the global-flag.
func (c *controller) getEffectiveNodeConditions(machine *v1alpha1.Machine) *string {
	var effectiveNodeConditions *string
	if machine.Spec.MachineConfiguration != nil && machine.Spec.MachineConfiguration.NodeConditions != nil {
		effectiveNodeConditions = machine.Spec.MachineConfiguration.NodeConditions
	} else {
		effectiveNodeConditions = &c.nodeConditions
	}
	return effectiveNodeConditions
}
