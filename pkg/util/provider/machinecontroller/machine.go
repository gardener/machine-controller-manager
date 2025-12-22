// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"errors"
	"fmt"
	"github.com/gardener/machine-controller-manager/pkg/util/nodeops"
	clientretry "k8s.io/client-go/util/retry"
	"maps"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
func (c *controller) addMachine(obj any) {
	machine, ok := obj.(*v1alpha1.Machine)
	if !ok {
		klog.Errorf("couldn't convert to machine resource from object")
		return
	}
	// On restart of the controller process, a machine that was marked for
	// deletion would be processed as part of an `add` event. This check
	// ensures that its enqueued in the correct queue.
	if c.shouldMachineBeMovedToTerminatingQueue(machine) {
		c.enqueueMachineTermination(machine, "handling terminating machine object ADD event")
	} else {
		c.enqueueMachine(obj, "handling machine obj ADD event")
	}
}

func (c *controller) updateMachine(oldObj, newObj any) {
	oldMachine := oldObj.(*v1alpha1.Machine)
	newMachine := newObj.(*v1alpha1.Machine)

	if oldMachine == nil || newMachine == nil {
		klog.Errorf("couldn't convert to machine resource from object")
		return
	}
	{ // TODO@thiyyakat: remove after testing
		if newMachine.Labels["test-failed"] != oldMachine.Labels["test-failed"] {
			c.enqueueMachine(newObj, "TEST: handling machine failure simulation UPDATE event")
		}
	}
	if c.handlePreserveAnnotationsChange(oldMachine.Annotations, newMachine.Annotations, newMachine) {
		c.enqueueMachine(newObj, "handling machine object preservation related UPDATE event")
		return
	}

	if _, exists := newMachine.Annotations[machineutils.PreserveMachineAnnotationKey]; exists && newMachine.Status.CurrentStatus.Phase == v1alpha1.MachineFailed && oldMachine.Status.CurrentStatus.Phase != newMachine.Status.CurrentStatus.Phase {
		c.enqueueMachine(newObj, "handling preserved machine phase update")
	}

	if oldMachine.Generation == newMachine.Generation {
		klog.V(3).Infof("Skipping other non-spec updates for machine %q", oldMachine.Name)
		return
	}

	if c.shouldMachineBeMovedToTerminatingQueue(newMachine) {
		c.enqueueMachineTermination(newMachine, "handling terminating machine object UPDATE event")
	} else {
		c.enqueueMachine(newObj, "handling machine object UPDATE event")
	}
}

func (c *controller) deleteMachine(obj any) {
	machine, ok := obj.(*v1alpha1.Machine)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		machine, ok = tombstone.Obj.(*v1alpha1.Machine)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Machine %#v", obj))
			return
		}
	}
	c.enqueueMachineTermination(machine, "handling terminating machine object DELETE event")
}

// getKeyForObj returns key for object, else returns false
func (c *controller) getKeyForObj(obj any) (string, bool) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return "", false
	}
	return key, true
}

func (c *controller) enqueueMachine(obj any, reason string) {
	if key, ok := c.getKeyForObj(obj); ok {
		klog.V(3).Infof("Adding machine object to queue %q, reason: %s", key, reason)
		c.machineQueue.Add(key)
	}
}

func (c *controller) enqueueMachineAfter(obj any, after time.Duration, reason string) {
	if key, ok := c.getKeyForObj(obj); ok {
		klog.V(3).Infof("Adding machine object to queue %q after %s, reason: %s", key, after, reason)
		c.machineQueue.AddAfter(key, after)
	}
}

func (c *controller) enqueueMachineTermination(machine *v1alpha1.Machine, reason string) {

	if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(machine); err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", machine, err))
		return
	} else {
		klog.V(3).Infof("Adding machine object to termination queue %q, reason: %s", key, reason)
		c.machineTerminationQueue.Add(key)
	}
}

func (c *controller) enqueueMachineTerminationAfter(machine *v1alpha1.Machine, after time.Duration, reason string) {
	if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(machine); err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", machine, err))
		return
	} else {
		klog.V(3).Infof("Adding machine object to termination queue %q after %s, reason: %s", key, after, reason)
		c.machineTerminationQueue.AddAfter(key, after)
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

	// Add finalizers if not present on machine object
	_, err = c.addMachineFinalizers(ctx, machine)
	if err != nil {
		return err
	}

	if c.shouldMachineBeMovedToTerminatingQueue(machine) {
		klog.Errorf("Machine %q should be in machine termination queue", machine.Name)
		c.enqueueMachineTermination(machine, "handling terminating machine object")
		return nil
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

	{ //TODO@thiyyakat: remove after testing
		if machine.Labels["test-failed"] == "true" {
			node, err := c.nodeLister.Get(getNodeName(machine))
			if err != nil {
				klog.V(3).Infof("TEST:Machine %q: Failed to get node %q: %v", machine.Name, machine.Name, err)
				return machineutils.ShortRetry, err
			}
			if cond := nodeops.GetCondition(node, corev1.NodeNetworkUnavailable); cond.Status != corev1.ConditionTrue {
				newCond := corev1.NodeCondition{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue}
				err = nodeops.AddOrUpdateConditionsOnNode(ctx, c.targetCoreClient, getNodeName(machine), newCond)
				if err != nil {
					klog.V(2).Infof("TEST:Machine %q: Failed to change node condition %q: %v", machine.Name, machine.Name, err)
					return machineutils.ShortRetry, err
				}
				klog.V(2).Infof("TEST: marked nodenetwork as unavailable for machine %s", machine.Name)
			}
		} else if machine.Labels["test-failed"] == "false" {
			node, err := c.nodeLister.Get(getNodeName(machine))
			if err != nil {
				klog.V(3).Infof("TEST:Machine %q: Failed to get node %q: %v", machine.Name, machine.Name, err)
				return machineutils.ShortRetry, err
			}
			if cond := nodeops.GetCondition(node, corev1.NodeNetworkUnavailable); cond.Status != corev1.ConditionFalse {
				newCond := corev1.NodeCondition{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}
				err = nodeops.AddOrUpdateConditionsOnNode(ctx, c.targetCoreClient, getNodeName(machine), newCond)
				if err != nil {
					klog.V(2).Infof("TEST:Machine %q: Failed to change node condition %q: %v", machine.Name, machine.Name, err)
					return machineutils.ShortRetry, err
				}
				klog.V(2).Infof("TEST: marked nodenetwork as available %s", machine.Name)
			}
		}
	}

	retry, err = c.manageMachinePreservation(ctx, machine)
	if err != nil {
		return retry, err
	}
	if machine.Labels[v1alpha1.NodeLabelKey] != "" && machine.Status.CurrentStatus.Phase != "" {
		// If reference to node object exists execute the below
		retry, err := c.reconcileMachineHealth(ctx, machine)
		if err != nil {
			return retry, err
		}

		retry, err = c.syncMachineNameToNode(ctx, machine)
		if err != nil {
			return retry, err
		}

		retry, err = c.syncNodeTemplates(ctx, machine, machineClass)
		if err != nil {
			return retry, err
		}

		retry, err = c.updateNodeConditionBasedOnLabel(ctx, machine)
		if err != nil {
			return retry, err
		}

		retry, err = c.inPlaceUpdate(ctx, machine)
		if err != nil {
			return retry, err
		}
	}

	retry, err = c.manageMachinePreservation(ctx, machine)
	if err != nil {
		return retry, err
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

func (c *controller) reconcileClusterMachineTermination(key string) error {
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

	klog.V(2).Infof("reconcileClusterMachineTermination: Start for %q with phase:%q, description:%q",
		machine.Name, machine.Status.CurrentStatus.Phase, machine.Status.LastOperation.Description)
	defer klog.V(2).Infof("reconcileClusterMachineTermination: Stop for %q", machine.Name)

	machineClass, secretData, retry, err := c.ValidateMachineClass(ctx, &machine.Spec.Class)
	if err != nil {
		klog.Errorf("cannot reconcile machine %q: %s", machine.Name, err)
		c.enqueueMachineTerminationAfter(machine, time.Duration(retry), err.Error())
		return nil
	}

	// Process a delete event
	retryPeriod, err := c.triggerDeletionFlow(
		ctx,
		&driver.DeleteMachineRequest{
			Machine:      machine,
			MachineClass: machineClass,
			Secret:       &corev1.Secret{Data: secretData},
		},
	)

	if err != nil {
		c.enqueueMachineTerminationAfter(machine, time.Duration(retryPeriod), err.Error())
	} else {
		// If the informer loses connection to the API server it may need to resync.
		// If a resource is deleted while the watch is down, the informer won’t get
		// delete event because the object is already gone. To avoid this edge-case,
		// a requeue is scheduled post machine deletion as well.
		c.enqueueMachineTerminationAfter(machine, time.Duration(retryPeriod), "post-deletion reconcile")
	}
	return nil
}

// handlePreserveAnnotationsChange returns true if there is a change in preserve annotations
// it also handles the special case where the annotation is changed from 'now' to 'when-failed'
// in which case it stops the preservation if expiry time is already set
func (c *controller) handlePreserveAnnotationsChange(oldAnnotations, newAnnotations map[string]string, machine *v1alpha1.Machine) bool {
	valueNew, existsInNew := newAnnotations[machineutils.PreserveMachineAnnotationKey]
	valueOld, existsInOld := oldAnnotations[machineutils.PreserveMachineAnnotationKey]
	if valueNew != machineutils.PreserveMachineAnnotationValueWhenFailed || valueOld != machineutils.PreserveMachineAnnotationValueNow {
		return existsInOld != existsInNew || valueOld != valueNew
	}
	// Special case: annotation changed from 'now' to 'when-failed'
	isPreserved := machineutils.IsPreserveExpiryTimeSet(machine)
	if !isPreserved {
		return true
	}
	ctx := context.Background()
	err := clientretry.RetryOnConflict(nodeops.Backoff, func() error {
		klog.V(3).Infof("Stopping preservation for machine %q as preserve annotation changed from 'now' to 'when-failed'.", machine.Name)
		return c.stopMachinePreservation(ctx, machine)
	})
	if err != nil {
		klog.Errorf("error while stopping preservation for machine %q: %v. Use preserve=false to stop preservation.", machine.Name, err)
	}
	return true
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
		machine              = createMachineRequest.Machine
		machineName          = createMachineRequest.Machine.Name
		uninitializedMachine = false
		addresses            = sets.New[corev1.NodeAddress]()
	)
	c.markCreationProcessing(machine)
	defer func() {
		c.unmarkCreationProcessing(machine)
	}()

	// This field is only modified during the creation flow. We have to assume that every Address that was added to the
	// status in the past remains valid, otherwise we have no way of keeping the addresses that might be only returned
	// by the `CreateMachine` or `InitializeMachine` calls. Before persisting, the addresses are deduplicated.
	addresses.Insert(createMachineRequest.Machine.Status.Addresses...)

	// we should avoid mutating Secret, since it goes all the way into the Informer's store
	secretCopy := createMachineRequest.Secret.DeepCopy()
	if err := c.addBootstrapTokenToUserData(ctx, machine, secretCopy); err != nil {
		return machineutils.ShortRetry, err
	}
	if err := c.addMachineNameToUserData(machine, secretCopy); err != nil {
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
		klog.Warningf("For machine %q, obtained VM error status as: %s", machineName, machineErr)
		// Decoding machine error code
		switch machineErr.Code() {
		case codes.NotFound, codes.Unimplemented:
			// Either VM is not found
			// or GetMachineStatus() call is not implemented
			// In this case, invoke a CreateMachine() call
			if machine.Spec.ProviderID == "" {
				// If no providerID is present
				klog.V(2).Infof("Creating a VM for machine %q, please wait!", machine.Name)
				klog.V(2).Infof("The machine creation is triggered with timeout of %s", c.getEffectiveCreationTimeout(createMachineRequest.Machine).Duration)
				createMachineResponse, err := c.driver.CreateMachine(ctx, createMachineRequest)
				if err != nil {
					// Create call returned an error
					klog.Errorf("Error while creating machine %s: %s", machine.Name, err.Error())
					return c.machineCreateErrorHandler(ctx, machine, createMachineResponse, err)
				}
				nodeName = createMachineResponse.NodeName
				providerID = createMachineResponse.ProviderID
				addresses.Insert(createMachineResponse.Addresses...)
				// Creation was successful
				klog.V(2).Infof("Created new VM for machine: %q with ProviderID: %q and backing node: %q", machine.Name, providerID, nodeName)

				if c.nodeLister != nil {
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
				}
				uninitializedMachine = true
			} else {
				// if .spec.ProviderID is present that means there must be a backing VM, without need of GetMachineStatus() call
				nodeName = machine.Labels[v1alpha1.NodeLabelKey]
				providerID = machine.Spec.ProviderID
			}

		case codes.Uninitialized:
			uninitializedMachine = true
			klog.Infof("VM instance associated with machine %s was created but not initialized.", machine.Name)
			//clean me up. I'm dirty.
			//TODO@thiyyakat add a pointer to a boolean variable indicating whether initialization has happened successfully.
			nodeName = getMachineStatusResponse.NodeName
			providerID = getMachineStatusResponse.ProviderID

		default:
			return c.machineCreateErrorHandler(ctx, machine, nil, err)
		}
	} else {
		if machine.Labels[v1alpha1.NodeLabelKey] == "" || machine.Spec.ProviderID == "" {
			klog.V(2).Infof("Found VM with required machine name. Adopting existing machine: %q with ProviderID: %s", machineName, getMachineStatusResponse.ProviderID)
		}
		nodeName = getMachineStatusResponse.NodeName
		providerID = getMachineStatusResponse.ProviderID
		addresses.Insert(getMachineStatusResponse.Addresses...)
	}

	if (c.targetCoreClient != nil && nodeName == "") || providerID == "" {
		err := status.Error(codes.Internal, fmt.Sprintf("machine %q: nodeName (%q) or providerID (%q) is empty after creation flow", machine.Name, nodeName, providerID))
		klog.Error(err)
		return machineutils.MediumRetry, err
	}

	//Update labels, providerID
	var clone *v1alpha1.Machine
	clone, err = c.updateLabels(ctx, createMachineRequest.Machine, nodeName, providerID)
	if err != nil {
		klog.Errorf("failed to update labels and providerID for machine %q. err=%q", machine.Name, err.Error())
	}
	//initialize VM if not initialized
	if uninitializedMachine {
		var retryPeriod machineutils.RetryPeriod
		var initResponse *driver.InitializeMachineResponse
		initResponse, retryPeriod, err = c.initializeMachine(ctx, clone, createMachineRequest.MachineClass, createMachineRequest.Secret)
		if err != nil {
			return retryPeriod, err
		}

		if c.targetCoreClient == nil {
			// persist addresses from the InitializeMachine and CreateMachine responses
			clone := clone.DeepCopy()
			if initResponse != nil {
				addresses.Insert(initResponse.Addresses...)
			}
			clone.Status.Addresses = buildAddressStatus(addresses, nodeName)
			if _, err := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{}); err != nil {
				return machineutils.ShortRetry, fmt.Errorf("failed to persist status addresses after initialization was successful: %w", err)
			}
		}

		// Return error even when machine object is updated
		err = fmt.Errorf("machine creation in process. Machine initialization (if required) is successful")
		return machineutils.ShortRetry, err
	}
	if err != nil {
		return machineutils.ShortRetry, err
	}

	if machine.Status.CurrentStatus.Phase == "" || machine.Status.CurrentStatus.Phase == v1alpha1.MachineCrashLoopBackOff {
		clone := clone.DeepCopy()
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

		// If running without a target cluster, set the Machine to Available immediately after a successful VM creation and report its addresses.
		// Skip waiting for the Node object to get registered.
		if c.targetCoreClient == nil {
			clone.Status.LastOperation.Description = "Created machine on cloud provider"
			clone.Status.LastOperation.State = v1alpha1.MachineStateSuccessful
			clone.Status.CurrentStatus.Phase = v1alpha1.MachineAvailable
			clone.Status.CurrentStatus.TimeoutActive = false
			clone.Status.Addresses = buildAddressStatus(addresses, nodeName)
		}

		_, err := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine/status UPDATE failed for %q. Retrying, error: %s", machine.Name, err)
		} else {
			klog.V(2).Infof("Machine/status UPDATE for %q during creation", machine.Name)
			// Return error even when machine object is updated
			err = fmt.Errorf("machine creation in process. Machine/Status UPDATE successful")
		}
		return machineutils.ShortRetry, err
	}
	return machineutils.LongRetry, nil
}

func (c *controller) updateLabels(ctx context.Context, machine *v1alpha1.Machine, nodeName, providerID string) (clone *v1alpha1.Machine, err error) {
	machineNodeLabelMissing := c.targetCoreClient != nil && !metav1.HasLabel(machine.ObjectMeta, v1alpha1.NodeLabelKey)
	machinePriorityAnnotationPresent := metav1.HasAnnotation(machine.ObjectMeta, machineutils.MachinePriority)
	clone = machine.DeepCopy()
	if machineNodeLabelMissing || !machinePriorityAnnotationPresent || machine.Spec.ProviderID == "" {
		if c.targetCoreClient != nil {
			// If running without a target cluster, don't add the node label. This disables all interaction with the
			// Node object and related objects in the target cluster.
			metav1.SetMetaDataLabel(&clone.ObjectMeta, v1alpha1.NodeLabelKey, nodeName)
		}
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		if clone.Annotations[machineutils.MachinePriority] == "" {
			clone.Annotations[machineutils.MachinePriority] = "3"
		}
		clone.Spec.ProviderID = providerID
		var updatedMachine *v1alpha1.Machine
		updatedMachine, err = c.controlMachineClient.Machines(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Machine labels/annotations UPDATE failed for %q. Will retry after VM initialization (if required), error: %s", machine.Name, err)
			clone = machine.DeepCopy()
		} else {
			clone = updatedMachine
			klog.V(2).Infof("Machine labels/annotations UPDATE for %q", clone.Name)
			err = fmt.Errorf("machine creation in process. Machine labels/annotations update is successful")
		}
	}
	return clone, err
}

func (c *controller) initializeMachine(ctx context.Context, machine *v1alpha1.Machine, machineClass *v1alpha1.MachineClass, secret *corev1.Secret) (resp *driver.InitializeMachineResponse, retry machineutils.RetryPeriod, err error) {
	req := &driver.InitializeMachineRequest{
		Machine:      machine,
		MachineClass: machineClass,
		Secret:       secret,
	}
	klog.V(3).Infof("Initializing VM instance for Machine %q", machine.Name)
	resp, err = c.driver.InitializeMachine(ctx, req)
	if err != nil {
		errStatus, ok := status.FromError(err)
		if !ok {
			klog.Errorf("Cannot decode Driver error for machine %q: %s. Unexpected behaviour as Driver errors are expected to be of type status.Status", machine.Name, err)
			return nil, machineutils.LongRetry, err
		}
		if errStatus.Code() == codes.Unimplemented {
			klog.V(3).Infof("Provider does not support Driver.InitializeMachine - skipping VM instance initialization for %q.", machine.Name)
			return nil, 0, nil
		}
		klog.Errorf("Error occurred while initializing VM instance for machine %q: %s", machine.Name, err)
		updateRetryPeriod, updateErr := c.machineStatusUpdate(
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
		if updateErr != nil {
			return nil, updateRetryPeriod, updateErr
		}
		return nil, machineutils.ShortRetry, err
	}
	klog.V(3).Infof("VM instance %q for machine %q was initialized", resp.ProviderID, machine.Name)
	return resp, 0, nil
}

func (c *controller) triggerDeletionFlow(ctx context.Context, deleteMachineRequest *driver.DeleteMachineRequest) (machineutils.RetryPeriod, error) {
	var (
		machine    = deleteMachineRequest.Machine
		finalizers = sets.NewString(machine.Finalizers...)
	)

	switch {
	case c.isCreationProcessing(machine):
		err := fmt.Errorf("machine %q is in creation flow. Deletion cannot proceed", machine.Name)
		return machineutils.MediumRetry, err

	case !finalizers.Has(MCMFinalizerName):
		// If Finalizers are not present on machine
		err := fmt.Errorf("Machine %q is missing finalizers. Deletion cannot proceed", machine.Name)
		return machineutils.LongRetry, err

	case machine.Status.CurrentStatus.Phase != v1alpha1.MachineTerminating:
		return c.setMachineTerminationStatus(ctx, deleteMachineRequest)

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.GetVMStatus):
		return c.updateMachineStatusAndNodeLabel(
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

	case strings.Contains(machine.Status.LastOperation.Description, machineutils.RemoveNodeFinalizers):
		return c.deleteNodeFinalizers(ctx, machine)

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

// buildAddressStatus adds the nodeName as a HostName address, if it is not empty, and returns a sorted and deduplicated
// slice.
func buildAddressStatus(addresses sets.Set[corev1.NodeAddress], nodeName string) []corev1.NodeAddress {
	if nodeName != "" {
		addresses.Insert(corev1.NodeAddress{
			Type:    corev1.NodeHostName,
			Address: nodeName,
		})
	}
	res := slices.Collect(maps.Keys(addresses))
	slices.SortStableFunc(res, func(a, b corev1.NodeAddress) int {
		return strings.Compare(string(a.Type)+a.Address, string(b.Type)+b.Address)
	})
	return res
}

func getMachineKey(machine *v1alpha1.Machine) string {
	machineNamespacedName, err := cache.MetaNamespaceKeyFunc(machine)
	if err != nil {
		machineNamespacedName = fmt.Sprintf("%s/%s", machine.Namespace, machine.Name)
		klog.Errorf("couldn't get key for machine %q, using %q instead: %v", machine.Name, machineNamespacedName, err)
	}
	return machineNamespacedName
}

func (c *controller) shouldMachineBeMovedToTerminatingQueue(machine *v1alpha1.Machine) bool {
	if machine.DeletionTimestamp != nil && c.isCreationProcessing(machine) {
		klog.Warningf("Cannot delete machine %q, its deletionTimestamp is set but it is currently being processed by the creation flow\n", getMachineKey(machine))
	}

	return !c.isCreationProcessing(machine) && machine.DeletionTimestamp != nil
}

func (c *controller) markCreationProcessing(machine *v1alpha1.Machine) {
	c.pendingMachineCreationMap.Store(getMachineKey(machine), "")
}
func (c *controller) unmarkCreationProcessing(machine *v1alpha1.Machine) {
	c.pendingMachineCreationMap.Delete(getMachineKey(machine))
}
func (c *controller) isCreationProcessing(machine *v1alpha1.Machine) bool {
	_, isMachineInCreationFlow := c.pendingMachineCreationMap.Load(getMachineKey(machine))
	return isMachineInCreationFlow
}

// manageMachinePreservation checks if any preservation-related operations need to be performed on the machine and node objects
func (c *controller) manageMachinePreservation(ctx context.Context, machine *v1alpha1.Machine) (retry machineutils.RetryPeriod, err error) {
	defer func() {
		if err != nil {
			if apierrors.IsConflict(err) {
				retry = machineutils.ConflictRetry
			}
			retry = machineutils.ShortRetry
		}
		retry = machineutils.LongRetry
	}()

	preserveValue, exists, err := c.computeEffectivePreserveAnnotationValue(machine)
	if err != nil {
		return
	}
	if !exists {
		return
	}
	// if preserve value differs from machine's preserve value, overwrite the value in the machine
	clone := machine.DeepCopy()
	if machine.Annotations == nil || machine.Annotations[machineutils.PreserveMachineAnnotationKey] != preserveValue {
		clone, err = c.writePreserveAnnotationValueOnMachine(ctx, clone, preserveValue)
		if err != nil {
			return
		}
	}
	if !isPreserveAnnotationValueValid(preserveValue) {
		klog.Warningf("Preserve annotation value %q on machine %s is invalid", preserveValue, machine.Name)
		return
	} else if preserveValue == machineutils.PreserveMachineAnnotationValueFalse || hasMachinePreservationTimedOut(clone) {
		err = c.stopMachinePreservation(ctx, clone)
		return
	} else if preserveValue == machineutils.PreserveMachineAnnotationValueWhenFailed {
		machineFailed := machineutils.IsMachineFailed(clone)
		if machineFailed {
			err = c.preserveMachine(ctx, clone, preserveValue)
		}
		// Here, if the preserve value is when-failed, but the machine is in running, there could be 2 possibilities:
		// 1. The machine was initially annotated with preserve=now and has been preserved, but later the annotation was changed to when-failed. In this case,
		//    we want to stop preservation. This case is already being handled in updateMachine and updateNodeToMachine functions.
		// 2. The machine was initially annotated with preserve=when-failed and has recovered from Failed to Running. In this case,
		//    we want to continue preservation until the annotation is changed to false or the preservation times out, so that CA does not
		//    scale down the node before pods are assigned to it.
		return
	} else if preserveValue == machineutils.PreserveMachineAnnotationValueNow || preserveValue == machineutils.PreserveMachineAnnotationValuePreservedByMCM {
		err = c.preserveMachine(ctx, clone, preserveValue)
		return
	}
	return
}

func (c *controller) getNodePreserveAnnotationValue(machine *v1alpha1.Machine) (nAnnotationValue string, nExists bool, err error) {
	nodeName := machine.Labels[v1alpha1.NodeLabelKey]
	if nodeName == "" {
		return
	}
	node, err := c.nodeLister.Get(nodeName)
	if err != nil {
		klog.Errorf("error trying to get node %q: %v", nodeName, err)
		return
	}
	if node.Annotations != nil {
		nAnnotationValue, nExists = node.Annotations[machineutils.PreserveMachineAnnotationKey]
	}
	return
}

// computeEffectivePreserveAnnotationValue returns the effective preservation value based on node's and machine's annotations.
// if the backing node is annotated with preserve annotation, the node's preserve value will be honoured
// if there is no backing node, or the node has no preserve annotation, then the machine's preserve value is honoured
// if both machine and node objects have conflicting preserve annotation values, the node's value will be honoured
func (c *controller) computeEffectivePreserveAnnotationValue(machine *v1alpha1.Machine) (preserveValue string, exists bool, err error) {
	mAnnotationValue, mExists := machine.Annotations[machineutils.PreserveMachineAnnotationKey]
	nAnnotationValue, nExists, err := c.getNodePreserveAnnotationValue(machine)
	if err != nil {
		return
	}
	exists = mExists || nExists
	if !exists {
		return
	}
	if nExists {
		preserveValue = nAnnotationValue
	} else {
		preserveValue = mAnnotationValue
	}
	return
}

// writePreserveAnnotationValueOnMachine syncs the effective preserve value on the machine objects
func (c *controller) writePreserveAnnotationValueOnMachine(ctx context.Context, machine *v1alpha1.Machine, preserveValue string) (*v1alpha1.Machine, error) {
	if machine.Annotations == nil {
		machine.Annotations = make(map[string]string)
	}
	machine.Annotations[machineutils.PreserveMachineAnnotationKey] = preserveValue
	updatedMachine, err := c.controlMachineClient.Machines(c.namespace).Update(ctx, machine, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("error updating machine %q with preserve annotation %q: %v", machine.Name, preserveValue, err)
		return machine, err
	}
	return updatedMachine, nil
}

// isPreserveAnnotationValueValid checks if the preserve annotation value is valid
func isPreserveAnnotationValueValid(preserveValue string) bool {
	_, exists := machineutils.AllowedPreserveAnnotationValues[preserveValue]
	return exists
}

// isPreservedNodeConditionStatusTrue check if all the steps in the preservation logic have been completed for the machine
// if the machine has no backing node, only PreserveExpiryTime needs to be set
// if the machine has a backing node, the NodePreserved condition on the node needs to be true
func (c *controller) isPreservedNodeConditionStatusTrue(cond *corev1.NodeCondition) bool {
	if cond == nil {
		return false
	}
	if cond.Status == corev1.ConditionTrue {
		return true
	}
	return false
}
