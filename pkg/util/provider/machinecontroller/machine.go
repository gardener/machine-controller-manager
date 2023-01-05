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
	"k8s.io/klog/v2"
)

/*
SECTION
Machine controller - Machine add, update, delete watches
*/
func (c *controller) addMachine(obj interface{}) {
	klog.V(5).Infof("Adding machine object")
	c.enqueueMachine(obj)
}

func (c *controller) updateMachine(oldObj, newObj interface{}) {
	klog.V(5).Info("Updating machine object")
	c.enqueueMachine(newObj)
}

func (c *controller) deleteMachine(obj interface{}) {
	klog.V(5).Info("Deleting machine object")
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
		klog.V(5).Infof("Adding machine object to the queue %q", key)
		c.machineQueue.Add(key)
	}
}

func (c *controller) enqueueMachineAfter(obj interface{}, after time.Duration) {
	if toBeEnqueued, key := c.isToBeEnqueued(obj); toBeEnqueued {
		klog.V(5).Infof("Adding machine object to the queue %q after %s", key, after)
		c.machineQueue.AddAfter(key, after)
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
		klog.Errorf("Machine %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	retryPeriod, err := c.reconcileClusterMachine(ctx, machine)
	klog.V(5).Info(err, retryPeriod)

	c.enqueueMachineAfter(machine, time.Duration(retryPeriod))

	return nil
}

func (c *controller) reconcileClusterMachine(ctx context.Context, machine *v1alpha1.Machine) (machineutils.RetryPeriod, error) {
	klog.V(5).Infof("Start Reconciling machine: %q , nodeName: %q ,providerID: %q", machine.Name, getNodeName(machine), getProviderID(machine))
	defer klog.V(5).Infof("Stop Reconciling machine %q, nodeName: %q ,providerID: %q", machine.Name, getNodeName(machine), getProviderID(machine))

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

	// Validate MachineClass
	machineClass, secretData, retry, err := c.ValidateMachineClass(ctx, &machine.Spec.Class)
	if err != nil {
		klog.Error(err)
		return retry, err
	}

	if machine.DeletionTimestamp != nil {
		// Process a delete event
		return c.triggerDeletionFlow(
			ctx,
			&driver.DeleteMachineRequest{
				Machine:      machine,
				MachineClass: machineClass,
				Secret:       &corev1.Secret{Data: secretData},
			},
		)
	}

	// Add finalizers if not present on machine object
	retry, err = c.addMachineFinalizers(ctx, machine)
	if err != nil {
		return retry, err
	}

	if machine.Labels[v1alpha1.NodeLabelKey] != "" && machine.Status.CurrentStatus.Phase != "" {
		// If reference to node object exists execute the below
		retry, err := c.reconcileMachineHealth(ctx, machine)
		if err != nil {
			return retry, err
		}

		retry, err = c.syncMachineNodeTemplates(ctx, machine)
		if err != nil {
			return retry, err
		}
	}
	if machine.Spec.ProviderID == "" || machine.Status.CurrentStatus.Phase == "" {
		return c.triggerCreationFlow(
			ctx,
			&driver.CreateMachineRequest{
				Machine:      machine,
				MachineClass: machineClass,
				Secret:       &corev1.Secret{Data: secretData},
			},
		)
	}

	return machineutils.LongRetry, nil
}

/*
SECTION
Machine controller - nodeToMachine
*/
var (
	errMultipleMachineMatch = errors.New("Multiple machines matching node")
	errNoMachineMatch       = errors.New("No machines matching node found")
)

func (c *controller) addNodeToMachine(obj interface{}) {
	node := obj.(*corev1.Node)
	if node == nil {
		klog.Errorf("Couldn't convert to node from object")
		return
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		return
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	machine, err := c.getMachineFromNode(key)
	if err != nil {
		if err == errNoMachineMatch {
			// errNoMachineMatch could mean that VM is still in creation hence ignoring it
			return
		}

		klog.Errorf("Couldn't fetch machine %s, Error: %s", key, err)
		return
	}

	if machine.Status.CurrentStatus.Phase != v1alpha1.MachineCrashLoopBackOff && nodeConditionsHaveChanged(machine.Status.Conditions, node.Status.Conditions) {
		klog.V(5).Infof("Enqueue machine object %q as conditions of backing node %q have changed", machine.Name, getNodeName(machine))
		c.enqueueMachine(machine)
	}
}

func (c *controller) updateNodeToMachine(oldObj, newObj interface{}) {
	node := newObj.(*corev1.Node)
	if node == nil {
		klog.Errorf("Couldn't convert to node from object")
		return
	}

	// check for the TriggerDeletionByMCM annotation on the node object
	// if it is present then mark the machine object for deletion
	if value, ok := node.Annotations[machineutils.TriggerDeletionByMCM]; ok && value == "true" {

		machine, err := c.getMachineFromNode(node.Name)
		if err != nil {
			klog.Errorf("Couldn't fetch machine %s, Error: %s", machine.Name, err)
			return
		}

		if machine.DeletionTimestamp == nil {
			klog.Infof("Node %s is annotated to trigger deletion by MCM.", node.Name)
			if err := c.controlMachineClient.Machines(c.namespace).Delete(context.Background(), machine.Name, metav1.DeleteOptions{}); err != nil {
				klog.Errorf("Machine object %s backing the node %s could not be marked for deletion.", machine.Name, node.Name)
				return
			}
			klog.Infof("Machine object %s backing the node %s marked for deletion.", machine.Name, node.Name)
		}
	}

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
		return nil, errMultipleMachineMatch
	} else if len(machines) < 1 {
		return nil, errNoMachineMatch
	}

	return machines[0], nil
}

/*
	SECTION
	Machine operations - Create, Update, Delete
*/

func (c *controller) triggerCreationFlow(ctx context.Context, createMachineRequest *driver.CreateMachineRequest) (machineutils.RetryPeriod, error) {
	var (
		// Declarations
		nodeName, providerID string

		// Initializations
		machine     = createMachineRequest.Machine
		machineName = createMachineRequest.Machine.Name
	)

	// we should avoid mutating Secret, since it goes all the way into the Informer's store
	secretCopy := createMachineRequest.Secret.DeepCopy()
	err := c.addBootstrapTokenToUserData(ctx, machine.Name, secretCopy)
	if err != nil {
		return machineutils.ShortRetry, err
	}
	createMachineRequest.Secret = secretCopy

	// Find out if VM exists on provider for this machine object
	getMachineStatusResponse, err := c.driver.GetMachineStatus(
		ctx,
		&driver.GetMachineStatusRequest{
			Machine:      machine,
			MachineClass: createMachineRequest.MachineClass,
			Secret:       createMachineRequest.Secret,
		},
	)

	if err != nil {
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
			if _, present := machine.Labels[v1alpha1.NodeLabelKey]; !present {
				// If node label is not present
				klog.V(2).Infof("Creating a VM for machine %q, please wait!", machine.Name)
				klog.V(2).Infof("The machine creation is triggered with timeout of %s", c.getEffectiveCreationTimeout(createMachineRequest.Machine).Duration)
				createMachineResponse, err := c.driver.CreateMachine(ctx, createMachineRequest)
				if err != nil {
					// Create call returned an error.
					klog.Errorf("Error while creating machine %s: %s", machine.Name, err.Error())
					return c.machineCreateErrorHandler(ctx, machine, createMachineResponse, err)
				}

				nodeName = createMachineResponse.NodeName
				providerID = createMachineResponse.ProviderID

				// Creation was successful
				klog.V(2).Infof("Created new VM for machine: %q with ProviderID: %q and backing node: %q", machine.Name, providerID, getNodeName(machine))

				// If a node obj already exists by the same nodeName, treat it as a stale node and trigger machine deletion.
				// TODO: there is a case with Azure where the VM may join the cluster before the CreateMachine call is completed,
				// and that would make an otherwise healthy node, be marked as stale.
				// To avoid this scenario, check if the name of the node is equal to the machine name before marking them as stale.
				// Ideally, the check should compare that the providerID of the machine and the node are matching, but since this is
				// not enforced  for MCM extensions the current best option is to compare the names.
				if _, err := c.nodeLister.Get(nodeName); err == nil && nodeName != machineName {
					// mark the machine obj as `Failed`
					klog.Errorf("Stale node obj with name %q for machine %q has been found. Hence marking the created VM for deletion to trigger a new machine creation.", nodeName, machine.Name)

					deleteMachineRequest := &driver.DeleteMachineRequest{
						Machine: &v1alpha1.Machine{
							ObjectMeta: machine.ObjectMeta,
							Spec: v1alpha1.MachineSpec{
								ProviderID: providerID,
							},
						},
						MachineClass: createMachineRequest.MachineClass,
						Secret:       secretCopy,
					}

					_, err := c.driver.DeleteMachine(ctx, deleteMachineRequest)

					if err != nil {
						klog.V(2).Infof("VM deletion in context of stale node obj failed for machine %q, will be retried. err=%q", machine.Name, err.Error())
					} else {
						klog.V(2).Infof("VM successfully deleted in context of stale node obj for machine %q", machine.Name)
					}

					// machine obj marked Failed for double surity
					c.machineStatusUpdate(
						ctx,
						machine,
						v1alpha1.LastOperation{
							Description:    "VM using old node obj",
							State:          v1alpha1.MachineStateFailed,
							Type:           v1alpha1.MachineOperationCreate,
							LastUpdateTime: metav1.Now(),
						},
						v1alpha1.CurrentStatus{
							Phase:          v1alpha1.MachineFailed,
							LastUpdateTime: metav1.Now(),
						},
						machine.Status.LastKnownState,
					)

					klog.V(2).Infof("Machine %q marked Failed as VM was referring to a stale node object", machine.Name)
					return machineutils.ShortRetry, err
				}
			} else {
				// if node label present that means there must be a backing VM ,without need of GetMachineStatus() call
				nodeName = machine.Labels[v1alpha1.NodeLabelKey]
			}

		case codes.Unknown, codes.DeadlineExceeded, codes.Aborted, codes.Unavailable:
			// GetMachineStatus() returned with one of the above error codes.
			// Retry operation.
			c.machineStatusUpdate(
				ctx,
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
				ctx,
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
	} else {
		if machine.Labels[v1alpha1.NodeLabelKey] == "" || machine.Spec.ProviderID == "" {
			klog.V(2).Infof("Found VM with required machine name. Adopting existing machine: %q with ProviderID: %s", machineName, getMachineStatusResponse.ProviderID)
		}
		nodeName = getMachineStatusResponse.NodeName
		providerID = getMachineStatusResponse.ProviderID
	}

	_, machineNodeLabelPresent := createMachineRequest.Machine.Labels[v1alpha1.NodeLabelKey]
	_, machinePriorityAnnotationPresent := createMachineRequest.Machine.Annotations[machineutils.MachinePriority]

	if !machineNodeLabelPresent || !machinePriorityAnnotationPresent || machine.Spec.ProviderID == "" {
		clone := machine.DeepCopy()
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}
		clone.Labels[v1alpha1.NodeLabelKey] = nodeName
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		if clone.Annotations[machineutils.MachinePriority] == "" {
			clone.Annotations[machineutils.MachinePriority] = "3"
		}

		clone.Spec.ProviderID = providerID
		_, err := c.controlMachineClient.Machines(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine UPDATE failed for %q. Retrying, error: %s", machine.Name, err)
		} else {
			klog.V(2).Infof("Machine labels/annotations UPDATE for %q", machine.Name)

			// Return error even when machine object is updated
			err = fmt.Errorf("Machine creation in process. Machine UPDATE successful")
		}
		return machineutils.ShortRetry, err
	}

	if machine.Status.CurrentStatus.Phase == "" {
		clone := machine.DeepCopy()
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

		_, err := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
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

func (c *controller) triggerUpdationFlow(ctx context.Context, machine *v1alpha1.Machine, actualProviderID string) (machineutils.RetryPeriod, error) {
	klog.V(2).Infof("Setting ProviderID of machine %s with backing node %s to %s", machine.Name, getNodeName(machine), actualProviderID)

	for {
		machine, err := c.controlMachineClient.Machines(machine.Namespace).Get(ctx, machine.Name, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Machine GET failed. Retrying, error: %s", err)
			continue
		}

		clone := machine.DeepCopy()
		clone.Spec.ProviderID = actualProviderID
		machine, err = c.controlMachineClient.Machines(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
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
		_, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine/status UPDATE failed. Retrying, error: %s", err)
			continue
		}
		// Update went through, exit out of infinite loop
		break
	}

	return machineutils.LongRetry, nil
}

func (c *controller) triggerDeletionFlow(ctx context.Context, deleteMachineRequest *driver.DeleteMachineRequest) (machineutils.RetryPeriod, error) {
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
		return c.setMachineTerminationStatus(ctx, deleteMachineRequest)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.GetVMStatus):
		return c.getVMStatus(
			ctx,
			&driver.GetMachineStatusRequest{
				Machine:      deleteMachineRequest.Machine,
				MachineClass: deleteMachineRequest.MachineClass,
				Secret:       deleteMachineRequest.Secret,
			})

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateDrain):
		return c.drainNode(ctx, deleteMachineRequest)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateVMDeletion):
		return c.deleteVM(ctx, deleteMachineRequest)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateNodeDeletion):
		return c.deleteNodeObject(ctx, machine)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.InitiateFinalizerRemoval):
		_, err := c.deleteMachineFinalizers(ctx, machine)
		if err != nil {
			// Keep retrying until update goes through
			klog.Errorf("Machine finalizer REMOVAL failed for machine %q. Retrying, error: %s", machine.Name, err)
			return machineutils.ShortRetry, err
		}

	default:
		err := fmt.Errorf("Unable to decode deletion flow state for machine %q. Re-initiate termination", machine.Name)
		klog.Warning(err)
		return c.setMachineTerminationStatus(ctx, deleteMachineRequest)
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

	klog.V(2).Infof("Machine %q with providerID %q and nodeName %q deleted successfully", machine.Name, getProviderID(machine), getNodeName(machine))
	return machineutils.LongRetry, nil
}
