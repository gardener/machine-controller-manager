// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// to reconcile on change in annotations related to preservation
	if oldMachine.Annotations[machineutils.PreserveMachineAnnotationKey] != newMachine.Annotations[machineutils.PreserveMachineAnnotationKey] {
		c.enqueueMachine(newObj, "handling machine object preservation related UPDATE event")
		return
	}
	// this check is required to enqueue a machine, annotated with the preservation key, whose phase has changed.
	// if annotated with "when-failed", the machine should be preserved on Failure
	// if annotated with "now", the machine should be drained on Failure
	// if machine phase changes from Failed to Running, machine preservation should be stopped
	_, exists := newMachine.Annotations[machineutils.PreserveMachineAnnotationKey]
	if exists && oldMachine.Status.CurrentStatus.Phase != newMachine.Status.CurrentStatus.Phase {
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

/*
	SECTION
	Machine Preservation operations
*/

// manageMachinePreservation manages machine preservation based on the preserve annotation value.
func (c *controller) manageMachinePreservation(ctx context.Context, machine *v1alpha1.Machine) (retry machineutils.RetryPeriod, err error) {
	machineAnnotationValue := machine.Annotations[machineutils.PreserveMachineAnnotationKey]
	laNodePreserveValue := machine.Annotations[machineutils.LastAppliedNodePreserveValueAnnotationKey]
	clone := machine.DeepCopy()
	defer func() {
		if err == nil {
			err = c.updatePreserveAnnotations(ctx, clone, machineAnnotationValue, laNodePreserveValue)
			if err == nil {
				retry = machineutils.LongRetry
			}
		}
		if err != nil {
			if apierrors.IsConflict(err) {
				retry = machineutils.ConflictRetry
			} else {
				retry = machineutils.ShortRetry
			}
		} else {
			retry = machineutils.LongRetry
		}
	}()
	nodeName := clone.Labels[v1alpha1.NodeLabelKey]
	nodeAnnotationValue := ""
	if nodeName != "" {
		nodeAnnotationValue, err = c.getNodeAnnotationValue(nodeName)
		if err != nil {
			return
		}
	}
	effectivePreserveValue, laNodePreserveValue := c.computeEffectivePreserveValues(nodeAnnotationValue, laNodePreserveValue, machineAnnotationValue)
	// The annotation has either been deleted, set to empty or no preserve annotation exists.
	// in all these cases, machine preservation should not be done. If machine is preserved, stop preservation.
	if effectivePreserveValue == "" {
		err = c.stopMachinePreservationIfPreserved(ctx, clone)
		return
	}
	if !machineutils.AllowedPreserveAnnotationValues.Has(effectivePreserveValue) {
		if effectivePreserveValue == nodeAnnotationValue {
			klog.Warningf("Preserve annotation value %q on node %q is invalid", effectivePreserveValue, nodeName)
		} else {
			klog.Warningf("Preserve annotation value %q on machine %q is invalid", effectivePreserveValue, clone.Name)
		}
		return
	}
	// if preserve=false or if preservation has expired, stop preservation
	if effectivePreserveValue == machineutils.PreserveMachineAnnotationValueFalse || (clone.Status.CurrentStatus.PreserveExpiryTime != nil && !clone.Status.CurrentStatus.PreserveExpiryTime.After(time.Now())) {
		err = c.stopMachinePreservationIfPreserved(ctx, clone)
		return
	}
	if effectivePreserveValue == machineutils.PreserveMachineAnnotationValueWhenFailed || effectivePreserveValue == machineutils.PreserveMachineAnnotationValuePreservedByMCM {
		if machineutils.IsMachineFailed(clone) {
			err = c.preserveMachine(ctx, clone, effectivePreserveValue)
		} else {
			err = c.stopMachinePreservationIfPreserved(ctx, clone)
			if err != nil {
				return
			}
			// To prevent erroneous re-preservation of a recovered, previously auto-preserved machine on future failures
			// (since the autoPreserveFailedMachineCount maintained by the machineSetController, may have changed),
			// in addition to stopping preservation we also remove the preservation annotation on the machine.
			// If the machine fails again, it can be preserved again based on the autoPreserveFailedMachineCount value at that time.
			if effectivePreserveValue == machineutils.PreserveMachineAnnotationValuePreservedByMCM {
				machineAnnotationValue = ""
			}
		}
	} else if effectivePreserveValue == machineutils.PreserveMachineAnnotationValueNow {
		err = c.preserveMachine(ctx, clone, effectivePreserveValue)
		if err != nil {
			return
		}
	}
	// This is to handle the case where a preserved machine recovers from Failed to Running
	// in which case, pods should be allowed to be scheduled onto the node
	if machineutils.IsMachineActive(clone) && nodeName != "" {
		err = c.uncordonNodeIfCordoned(ctx, nodeName)
	}
	return
}

// computeEffectivePreserveValue returns the effective preservation value, and the updated lastAppliedNodeAnnotationValue
func (c *controller) computeEffectivePreserveValues(nodeAnnotationValue, lastAppliedNodeAnnotationValue, machineAnnotationValue string) (string, string) {
	if nodeAnnotationValue == "" && lastAppliedNodeAnnotationValue == "" {
		return machineAnnotationValue, ""
	}
	return nodeAnnotationValue, nodeAnnotationValue
}

func (c *controller) getNodeAnnotationValue(nodeName string) (string, error) {
	node, err := c.nodeLister.Get(nodeName)
	if err != nil {
		klog.Errorf("error fetching node %q: %v", nodeName, err)
		return "", err
	}
	return node.Annotations[machineutils.PreserveMachineAnnotationKey], nil
}

// updatePreserveAnnotations updates the last applied node annotation value and the preservation annotation values on the machine
func (c *controller) updatePreserveAnnotations(ctx context.Context, machine *v1alpha1.Machine, machineAnnotationValue, laNodePreserveValue string) error {
	update := false
	if machineAnnotationValue == "" {
		delete(machine.Annotations, machineutils.PreserveMachineAnnotationKey)
		update = true
	} else if machine.Annotations[machineutils.PreserveMachineAnnotationKey] != machineAnnotationValue {
		machine.Annotations[machineutils.PreserveMachineAnnotationKey] = machineAnnotationValue
		update = true
	}

	if machine.Annotations[machineutils.LastAppliedNodePreserveValueAnnotationKey] != laNodePreserveValue {
		machine.Annotations[machineutils.LastAppliedNodePreserveValueAnnotationKey] = laNodePreserveValue
		update = true
	}
	if !update {
		return nil
	}
	_, err := c.controlMachineClient.Machines(machine.Name).Update(ctx, machine, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("error updating preserve annotations values on machine %q.", machine.Name)
	}
	return err
}
