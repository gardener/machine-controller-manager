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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

/*
SECTION
Machine controller - Machine add, update, delete watches
*/
func (c *controller) addMachine(obj interface{}) {
	c.enqueueMachine(obj, "handling machine obj ADD event")
}

func (c *controller) updateMachine(oldObj, newObj interface{}) {
	oldMachine := oldObj.(*v1alpha1.Machine)
	newMachine := newObj.(*v1alpha1.Machine)

	if oldMachine == nil || newMachine == nil {
		klog.Errorf("couldn't convert to machine resource from object")
		return
	}

	if oldMachine.Generation == newMachine.Generation {
		klog.V(3).Infof("Skipping non-spec updates for machine %s", oldMachine.Name)
		return
	}

	c.enqueueMachine(newObj, "handling machine object UPDATE event")
}

func (c *controller) deleteMachine(obj interface{}) {
	c.enqueueMachine(obj, "handling machine object DELETE event")
}

// getKeyForObj returns key for object, else returns false
func (c *controller) getKeyForObj(obj interface{}) (string, bool) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return "", false
	}
	return key, true
}

func (c *controller) enqueueMachine(obj interface{}, reason string) {
	if key, ok := c.getKeyForObj(obj); ok {
		klog.V(3).Infof("Adding machine object to queue %q, reason: %s", key, reason)
		c.machineQueue.Add(key)
	}
}

func (c *controller) enqueueMachineAfter(obj interface{}, after time.Duration, reason string) {
	if key, ok := c.getKeyForObj(obj); ok {
		klog.V(3).Infof("Adding machine object to queue %q after %s, reason: %s", key, after, reason)
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

	var reEnqueReason = "periodic reconcile"
	if err != nil {
		reEnqueReason = err.Error()
	}
	c.enqueueMachineAfter(machine, time.Duration(retryPeriod), reEnqueReason)

	return nil
}

func (c *controller) reconcileClusterMachine(ctx context.Context, machine *v1alpha1.Machine) (machineutils.RetryPeriod, error) {
	klog.V(2).Infof("reconcileClusterMachine: Start for %q with phase:%q, description:%q", machine.Name, machine.Status.CurrentStatus.Phase, machine.Status.LastOperation.Description)
	defer klog.V(2).Infof("reconcileClusterMachine: Stop for %q", machine.Name)

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
		err := fmt.Errorf("validation of Machine failed %s", validationerr.ToAggregate().Error())
		klog.Error(err)
		return machineutils.LongRetry, err
	}

	// Validate MachineClass
	machineClass, secretData, retry, err := c.ValidateMachineClass(ctx, &machine.Spec.Class)
	if err != nil {
		klog.Errorf("cannot reconcile machine %s: %s", machine.Name, err)
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
	if machine.Spec.ProviderID == "" || machine.Status.CurrentStatus.Phase == "" || machine.Status.CurrentStatus.Phase == v1alpha1.MachineCrashLoopBackOff {
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
	errMultipleMachineMatch = errors.New("multiple machines matching node")
	errNoMachineMatch       = errors.New("no machines matching node found")
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

	isMachineCrashLooping := machine.Status.CurrentStatus.Phase == v1alpha1.MachineCrashLoopBackOff
	isMachineTerminating := machine.Status.CurrentStatus.Phase == v1alpha1.MachineTerminating
	_, _, nodeConditionsHaveChanged := nodeConditionsHaveChanged(machine.Status.Conditions, node.Status.Conditions)

	if !isMachineCrashLooping && !isMachineTerminating && nodeConditionsHaveChanged {
		c.enqueueMachine(machine, fmt.Sprintf("handling node UPDATE event. conditions of backing node %q have changed", getNodeName(machine)))
	}
}

func (c *controller) updateNodeToMachine(oldObj, newObj interface{}) {
	oldNode := oldObj.(*corev1.Node)
	node := newObj.(*corev1.Node)
	if node == nil {
		klog.Errorf("Couldn't convert to node from object")
		return
	}

	machine, err := c.getMachineFromNode(node.Name)
	if err != nil {
		klog.Errorf("Unable to handle update event for node %s, couldn't fetch machine %s, Error: %s", machine.Name, err)
		return
	}

	// check for the TriggerDeletionByMCM annotation on the node object
	// if it is present then mark the machine object for deletion
	if value, ok := node.Annotations[machineutils.TriggerDeletionByMCM]; ok && value == "true" {
		if machine.DeletionTimestamp == nil {
			klog.Infof("Node %s for machine %s is annotated to trigger deletion by MCM.", node.Name, machine.Name)
			if err := c.controlMachineClient.Machines(c.namespace).Delete(context.Background(), machine.Name, metav1.DeleteOptions{}); err != nil {
				klog.Errorf("Machine object %s backing the node %s could not be marked for deletion.", machine.Name, node.Name)
				return
			}
			klog.Infof("Machine object %s backing the node %s marked for deletion.", machine.Name, node.Name)
		}
	}

	// to reconcile on addition/removal of essential taints in machine lifecycle, example - critical component taint
	if addedOrRemovedEssentialTaints(oldNode, node, machineutils.EssentialTaints) {
		c.enqueueMachine(machine, fmt.Sprintf("handling node UPDATE event. atleast one of essential taints on backing node %q has changed", getNodeName(machine)))
		return
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

	// donot respond if machine obj is already in termination flow
	if machine.DeletionTimestamp == nil {
		c.enqueueMachine(machine, fmt.Sprintf("backing node obj %q got deleted", key))
	}
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

func addedOrRemovedEssentialTaints(oldNode, node *corev1.Node, taintKeys []string) bool {
	mapOldNodeTaintKeys := make(map[string]bool)
	mapNodeTaintKeys := make(map[string]bool)

	for _, t := range oldNode.Spec.Taints {
		mapOldNodeTaintKeys[t.Key] = true
	}

	for _, t := range node.Spec.Taints {
		mapNodeTaintKeys[t.Key] = true
	}

	for _, tk := range taintKeys {
		_, oldNodeHasTaint := mapOldNodeTaintKeys[tk]
		_, newNodeHasTaint := mapNodeTaintKeys[tk]
		if oldNodeHasTaint != newNodeHasTaint {
			klog.V(2).Infof("Taint with key %q has been added/removed from the node %q", tk, node.Name)
			return true
		}
	}
	return false
}

/*
	SECTION
	Machine operations - Create, Delete
*/

func (c *controller) triggerCreationFlow(ctx context.Context, createMachineRequest *driver.CreateMachineRequest) (machineutils.RetryPeriod, error) {
	var (
		// Declarations
		nodeName, providerID string

		// Initializations
		machine                            = createMachineRequest.Machine
		machineName                        = createMachineRequest.Machine.Name
		newlyCreatedOrUninitializedMachine = false
	)

	// we should avoid mutating Secret, since it goes all the way into the Informer's store
	secretCopy := createMachineRequest.Secret.DeepCopy()
	err := c.addBootstrapTokenToUserData(ctx, machine, secretCopy)
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
		klog.Warningf("For machine %q, obtained VM error status as: %s", machineErr)

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
				newlyCreatedOrUninitializedMachine = true

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

					// machine obj marked Failed for double security
					updateRetryPeriod, updateErr := c.machineStatusUpdate(
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

					if updateErr != nil {
						return updateRetryPeriod, updateErr
					}

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
			updateRetryPeriod, updateErr := c.machineStatusUpdate(
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
			if updateErr != nil {
				return updateRetryPeriod, updateErr
			}

			return machineutils.ShortRetry, err

		case codes.Uninitialized:
			newlyCreatedOrUninitializedMachine = true
			klog.Infof("VM instance associated with machine %s was created but not initialized.", machine.Name)
		default:
			updateRetryPeriod, updateErr := c.machineStatusUpdate(
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
			if updateErr != nil {
				return updateRetryPeriod, updateErr
			}

			return machineutils.MediumRetry, err

		}
	} else {
		if machine.Labels[v1alpha1.NodeLabelKey] == "" || machine.Spec.ProviderID == "" {
			klog.V(2).Infof("Found VM with required machine name. Adopting existing machine: %q with ProviderID: %s", machineName, getMachineStatusResponse.ProviderID)
		}
		nodeName = getMachineStatusResponse.NodeName
		providerID = getMachineStatusResponse.ProviderID
	}

	if newlyCreatedOrUninitializedMachine {
		retryPeriod, err := c.initializeMachine(ctx, createMachineRequest.Machine, createMachineRequest.MachineClass, createMachineRequest.Secret)
		if err != nil {
			return retryPeriod, err
		}
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

	if machine.Status.CurrentStatus.Phase == "" || machine.Status.CurrentStatus.Phase == v1alpha1.MachineCrashLoopBackOff {
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

func (c *controller) initializeMachine(ctx context.Context, machine *v1alpha1.Machine, machineClass *v1alpha1.MachineClass, secret *corev1.Secret) (machineutils.RetryPeriod, error) {
	req := &driver.InitializeMachineRequest{
		Machine:      machine,
		MachineClass: machineClass,
		Secret:       secret,
	}
	klog.V(3).Infof("Initializing VM instance for Machine %q", machine.Name)
	resp, err := c.driver.InitializeMachine(ctx, req)
	if err != nil {
		errStatus, ok := status.FromError(err)
		if !ok {
			klog.Errorf("Error occurred while decoding machine error for machine %q: %s", machine.Name, err)
			return machineutils.MediumRetry, err
		}
		klog.Errorf("Error occurred while initializing VM instance for machine %q: %s", machine.Name, err)
		switch errStatus.Code() {
		case codes.Unimplemented:
			klog.Warningf("Provider does not support Driver.InitializeMachine - skipping VM instance initialization for %q.", machine.Name)
			return 0, nil
		case codes.NotFound:
			klog.Warningf("No VM instance found for machine %q. Skipping VM instance initialization.", machine.Name)
			return 0, nil
		case codes.Canceled, codes.Aborted:
			klog.Warningf("VM instance initialization for machine %q was aborted/cancelled.", machine.Name)
			return 0, nil
		}
		retryPeriod, _ := c.machineStatusUpdate(
			ctx,
			machine,
			v1alpha1.LastOperation{
				Description:    fmt.Sprintf("Provider error: %s. %s", err.Error(), machineutils.InstanceInitialization),
				ErrorCode:      errStatus.Code().String(),
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
		return retryPeriod, err
	}

	klog.V(3).Infof("VM instance %q for machine %q was initialized with last known state: %q", resp.ProviderID, machine.Name, resp.LastKnownState)
	return 0, nil
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

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.DelVolumesAttachments):
		return c.deleteNodeVolAttachments(ctx, deleteMachineRequest)

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
