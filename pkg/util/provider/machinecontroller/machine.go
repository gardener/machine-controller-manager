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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
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

// isToBeEnqueued returns true if the key is to be managed by this controller
func (c *controller) isToBeEnqueued(obj interface{}) (bool, string) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return false, ""
	}

	return true, key
}

func (c *controller) enqueueMachine(obj interface{}) {
	if toBeEnqueued, key := c.isToBeEnqueued(obj); toBeEnqueued {
		klog.V(4).Infof("Adding machine object to the queue %q", key)
		c.machineQueue.Add(key)
	}
}

func (c *controller) enqueueMachineAfter(obj interface{}, after time.Duration) {
	if toBeEnqueued, key := c.isToBeEnqueued(obj); toBeEnqueued {
		klog.V(4).Infof("Adding machine object to the queue %q after %s", key, after)
		c.machineQueue.AddAfter(key, after)
	}
}

func (c *controller) reconcileClusterMachineKey(key string) error {
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
		klog.Errorf("Machine %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	retryPeriod, err := c.reconcileClusterMachine(machine)
	klog.V(4).Info(err, retryPeriod)

	c.enqueueMachineAfter(machine, time.Duration(retryPeriod))

	return nil
}

func (c *controller) reconcileClusterMachine(machine *v1alpha1.Machine) (machineutils.RetryPeriod, error) {
	klog.V(4).Infof("Start Reconciling machine %q", machine.Name)
	defer klog.V(4).Infof("Stop Reconciling machine %q", machine.Name)

	if c.safetyOptions.MachineControllerFrozen && machine.DeletionTimestamp == nil {
		// If Machine controller is frozen and
		// machine is not set for termination don't process it
		err := fmt.Errorf("Machine controller has frozen. Retrying reconcile after resync period")
		klog.Error(err)
		return machineutils.LongRetry, err
	}

	internalMachine := &machineapi.Machine{}
	if err := c.internalExternalScheme.Convert(machine, internalMachine, nil); err != nil {
		klog.Error(err)
		return machineutils.LongRetry, err
	}

	validationerr := validation.ValidateMachine(internalMachine)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		err := fmt.Errorf("Validation of Machine failed %s", validationerr.ToAggregate().Error())
		klog.Error(err)
		return machineutils.LongRetry, err
	}

	machineClass, secretData, retry, err := c.ValidateMachineClass(&machine.Spec.Class)
	if err != nil {
		klog.Error(err)
		return retry, err
	}

	if machine.DeletionTimestamp != nil {
		// Process a delete event
		return c.triggerDeletionFlow(&driver.DeleteMachineRequest{
			Machine:      machine,
			MachineClass: machineClass,
			Secret:       &corev1.Secret{Data: secretData},
		})
	}

	if machine.Status.Node != "" {
		// If reference to node object exists execute the below
		retry, err := c.reconcileMachineHealth(machine)
		if err != nil {
			return retry, err
		}

		retry, err = c.syncMachineNodeTemplates(machine)
		if err != nil {
			return retry, err
		}
	}
	if machine.Spec.ProviderID == "" || machine.Status.CurrentStatus.Phase == "" || machine.Status.Node == "" {
		return c.triggerCreationFlow(&driver.CreateMachineRequest{
			Machine:      machine,
			MachineClass: machineClass,
			Secret:       &corev1.Secret{Data: secretData},
		})
	}

	return machineutils.LongRetry, nil
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

/*
	Move to update method?
	clone := machine.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}

	if _, ok := clone.Labels["node"]; !ok {
		clone.Labels["node"] = machine.Status.Node
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			klog.Warningf("Machine update failed. Retrying, error: %s", err)
			return machine, err
		}
	}
*/

/*
	SECTION
	Machine operations - Create, Update, Delete
*/

func (c *controller) triggerCreationFlow(createMachineRequest *driver.CreateMachineRequest) (machineutils.RetryPeriod, error) {
	var (
		machine     = createMachineRequest.Machine
		machineName = createMachineRequest.Machine.Name
		nodeName    = ""
		providerID  = ""
	)

	// Add finalizers if not present
	retry, err := c.addMachineFinalizers(createMachineRequest.Machine)
	if err != nil {
		return retry, err
	}

	// we should avoid mutating Secret, since it goes all the way into the Informer's store
	secretCopy := createMachineRequest.Secret.DeepCopy()
	err = c.addBootstrapTokenToUserData(machine.Name, secretCopy)
	if err != nil {
		return machineutils.ShortRetry, err
	}
	createMachineRequest.Secret = secretCopy

	// Find out if VM exists on provider for this machine object
	getMachineStatusResponse, err := c.driver.GetMachineStatus(context.TODO(), &driver.GetMachineStatusRequest{
		Machine:      machine,
		MachineClass: createMachineRequest.MachineClass,
		Secret:       createMachineRequest.Secret,
	})
	if err == nil {
		// Found VM with required machine name
		klog.V(2).Infof("Found VM with required machine name. Adopting existing machine: %q with ProviderID: %s", machineName, getMachineStatusResponse.ProviderID)
		nodeName = getMachineStatusResponse.NodeName
		providerID = getMachineStatusResponse.ProviderID
	} else {
		// VM with required name is not found.

		machineErr, ok := status.FromError(err)
		if !ok {
			// Error occurred with decoding machine error status, abort with retry.
			klog.Errorf("Error occurred while decoding machine error for machine %q: %s", machine.Name, err)
			return machineutils.MediumRetry, err
		}

		// Decoding machine error code
		switch machineErr.Code() {
		case codes.NotFound, codes.Unimplemented:
			// Either VM is not found
			// or GetMachineStatus() call is not implemented
			// In this case, invoke a CreateMachine() call
			klog.V(2).Infof("Creating a VM for machine %q, please wait!", machine.Name)
			if _, present := machine.Labels["node"]; !present {
				// If node label is not present
				createMachineResponse, err := c.driver.CreateMachine(context.TODO(), createMachineRequest)
				if err != nil {
					// Create call returned an error.
					klog.Errorf("Error while creating machine %s: %s", machine.Name, err.Error())
					return c.machineCreateErrorHandler(machine, createMachineResponse, err)
				}
				nodeName = createMachineResponse.NodeName
				providerID = createMachineResponse.ProviderID
			} else {
				nodeName = machine.Labels["node"]
			}

			// Creation was successful
			klog.V(2).Infof("Created new VM for machine: %q with ProviderID: %s", machine.Name, providerID)
			break

		case codes.Unknown, codes.DeadlineExceeded, codes.Aborted, codes.Unavailable:
			// GetMachineStatus() returned with one of the above error codes.
			// Retry operation.
			c.machineStatusUpdate(
				machine,
				v1alpha1.LastOperation{
					Description:    "Cloud provider message - " + err.Error(),
					State:          v1alpha1.MachineStateFailed,
					Type:           v1alpha1.MachineOperationCreate,
					LastUpdateTime: metav1.Now(),
				},
				v1alpha1.CurrentStatus{
					Phase:          c.getCreateFailurePhase(machine),
					LastUpdateTime: metav1.Now(),
				},
				machine.Status.LastKnownState,
			)

			return machineutils.ShortRetry, err

		default:
			c.machineStatusUpdate(
				machine,
				v1alpha1.LastOperation{
					Description:    "Cloud provider message - " + err.Error(),
					State:          v1alpha1.MachineStateFailed,
					Type:           v1alpha1.MachineOperationCreate,
					LastUpdateTime: metav1.Now(),
				},
				v1alpha1.CurrentStatus{
					Phase:          c.getCreateFailurePhase(machine),
					LastUpdateTime: metav1.Now(),
				},
				machine.Status.LastKnownState,
			)

			return machineutils.MediumRetry, err
		}
	}
	_, machineNodeLabelPresent := createMachineRequest.Machine.Labels["node"]
	_, machinePriorityAnnotationPresent := createMachineRequest.Machine.Annotations[machineutils.MachinePriority]

	if !machineNodeLabelPresent || !machinePriorityAnnotationPresent || machine.Spec.ProviderID == "" {
		clone := machine.DeepCopy()
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}
		clone.Labels["node"] = nodeName
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		if clone.Annotations[machineutils.MachinePriority] == "" {
			clone.Annotations[machineutils.MachinePriority] = "3"
		}

		clone.Spec.ProviderID = providerID
		_, err := c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			klog.Warningf("Machine UPDATE failed for %q. Retrying, error: %s", machine.Name, err)
		} else {
			klog.V(2).Infof("Machine labels/annotations UPDATE for %q", machine.Name)

			// Return error even when machine object is updated
			err = fmt.Errorf("Machine creation in process. Machine UPDATE successful")
		}
		return machineutils.ShortRetry, err
	}

	if machine.Status.Node != nodeName || machine.Status.CurrentStatus.Phase == "" {
		clone := machine.DeepCopy()

		clone.Status.Node = nodeName
		clone.Status.LastOperation = v1alpha1.LastOperation{
			Description:    "Creating machine on cloud provider",
			State:          v1alpha1.MachineStateProcessing,
			Type:           v1alpha1.MachineOperationCreate,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
			Phase:          v1alpha1.MachinePending,
			TimeoutActive:  true,
			LastUpdateTime: metav1.Now(),
		}

		_, err := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
		if err != nil {
			klog.Warningf("Machine/status UPDATE failed for %q. Retrying, error: %s", machine.Name, err)
		} else {
			klog.V(2).Infof("Machine/status UPDATE for %q during creation", machine.Name)

			// Return error even when machine object is updated
			err = fmt.Errorf("Machine creation in process. Machine/Status UPDATE successful")
		}

		return machineutils.ShortRetry, err
	}

	return machineutils.LongRetry, nil
}

func (c *controller) triggerUpdationFlow(machine *v1alpha1.Machine, actualProviderID string) (machineutils.RetryPeriod, error) {
	klog.V(2).Infof("Setting ProviderID of %s to %s", machine.Name, actualProviderID)

	for {
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Machine GET failed. Retrying, error: %s", err)
			continue
		}

		clone := machine.DeepCopy()
		clone.Spec.ProviderID = actualProviderID
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			klog.Warningf("Machine UPDATE failed. Retrying, error: %s", err)
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
			klog.Warningf("Machine/status UPDATE failed. Retrying, error: %s", err)
			continue
		}
		// Update went through, exit out of infinite loop
		break
	}

	return machineutils.LongRetry, nil
}

func (c *controller) triggerDeletionFlow(deleteMachineRequest *driver.DeleteMachineRequest) (machineutils.RetryPeriod, error) {
	var (
		machine    = deleteMachineRequest.Machine
		finalizers = sets.NewString(machine.Finalizers...)
	)

	switch {
	case !finalizers.Has(MCMFinalizerName):
		// If Finalizers are not present on machine
		err := fmt.Errorf("Machine %q is missing finalizers. Deletion cannot proceed", machine.Name)
		return machineutils.LongRetry, err

	case machine.Status.CurrentStatus.Phase != v1alpha1.MachineTerminating:
		return c.setMachineTerminationStatus(deleteMachineRequest)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.GetVMStatus):
		return c.getVMStatus(&driver.GetMachineStatusRequest{
			Machine:      deleteMachineRequest.Machine,
			MachineClass: deleteMachineRequest.MachineClass,
			Secret:       deleteMachineRequest.Secret,
		})

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateDrain):
		return c.drainNode(deleteMachineRequest)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateVMDeletion):
		return c.deleteVM(deleteMachineRequest)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateNodeDeletion):
		return c.deleteNodeObject(machine)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateFinalizerRemoval):
		_, err := c.deleteMachineFinalizers(machine)
		if err != nil {
			// Keep retrying until update goes through
			klog.Errorf("Machine finalizer REMOVAL failed for machine %q. Retrying, error: %s", machine.Name, err)
			return machineutils.ShortRetry, err
		}

	default:
		err := fmt.Errorf("Unable to decode deletion flow state for machine %q. Re-initiate termination", machine.Name)
		klog.Warning(err)
		return c.setMachineTerminationStatus(deleteMachineRequest)
	}

	/*
		// Delete machine object
		err := c.controlMachineClient.Machines(machine.Namespace).Delete(machine.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// If its an error, and anyother error than object not found
			klog.Errorf("Deletion of Machine Object %q failed due to error: %s", machine.Name, err)
			return machineutils.ShortRetry, err
		}
	*/

	klog.V(2).Infof("Machine %q deleted successfully", machine.Name)
	return machineutils.LongRetry, nil
}
