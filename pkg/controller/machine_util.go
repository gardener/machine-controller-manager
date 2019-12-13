/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/util/pod_util.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	v1alpha1client "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	v1alpha1listers "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
	cmiclient "github.com/gardener/machine-controller-manager/pkg/cmiclient"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

const (
	// LastAppliedALTAnnotation contains the last configuration of annotations, labels & taints applied on the node object
	LastAppliedALTAnnotation = "node.machine.sapcloud.io/last-applied-anno-labels-taints"
	// RetryOp tells the controller to retry
	RetryOp = true
	// DoNotRetryOp tells the controller to not retry for now. Resync after re-sync period
	DoNotRetryOp             = false
	getVMStatus              = "Set machine status to termination. Now, getting VM Status"
	initiateDrain            = "Initiate node drain"
	initiateVMDeletion       = "Initiate VM deletion"
	initiateNodeDeletion     = "Initiate node object deletion"
	initiateFinalizerRemoval = "Initiate machine object finalizer removal"
)

var (
	// emptyMap is a dummy emptyMap to compare with
	emptyMap = make(map[string]string)
)

// TODO: use client library instead when it starts to support update retries
//       see https://github.com/kubernetes/kubernetes/issues/21479
type updateMachineFunc func(machine *v1alpha1.Machine) error

// UpdateMachineWithRetries updates a machine with given applyUpdate function. Note that machine not found error is ignored.
// The returned bool value can be used to tell if the machine is actually updated.
func UpdateMachineWithRetries(machineClient v1alpha1client.MachineInterface, machineLister v1alpha1listers.MachineLister, namespace, name string, applyUpdate updateMachineFunc) (*v1alpha1.Machine, error) {
	var machine *v1alpha1.Machine

	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error
		machine, err = machineLister.Machines(namespace).Get(name)
		if err != nil {
			return err
		}
		machine = machine.DeepCopy()
		// Apply the update, then attempt to push it to the apiserver.
		if applyErr := applyUpdate(machine); applyErr != nil {
			return applyErr
		}
		machine, err = machineClient.Update(machine)
		return err
	})

	// Ignore the precondition violated error, this machine is already updated
	// with the desired label.
	if retryErr == errorsutil.ErrPreconditionViolated {
		glog.V(4).Infof("Machine %s precondition doesn't hold, skip updating it.", name)
		retryErr = nil
	}

	return machine, retryErr
}

// ValidateMachineClass validates the machine object
// and returns a list of errors.
func (c *controller) ValidateMachineClass(classSpec *v1alpha1.ClassSpec) (interface{}, *v1.Secret, error) {
	var (
		MachineClass interface{}
		secretRef    *v1.Secret
	)

	switch classSpec.Kind {

	case "MachineClass":
		CommonMachineClass, err := c.machineClassLister.MachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			glog.V(2).Infof("MachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}

		MachineClass = CommonMachineClass

		internalMachineClass := &machineapi.MachineClass{}
		err = c.internalExternalScheme.Convert(MachineClass, internalMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateMachineClass(internalMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of MachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		secretRef, err = c.getSecret(CommonMachineClass.SecretRef, CommonMachineClass.Name)
		if err != nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}

	default:
		glog.V(2).Infof("ClassKind %q not found", classSpec.Kind)
	}

	return MachineClass, secretRef, nil
}

// getSecret retrives the kubernetes secret if found
func (c *controller) getSecret(ref *v1.SecretReference, MachineClassName string) (*v1.Secret, error) {
	if ref == nil {
		// If no secretRef, return nil
		return nil, nil
	}

	secretRef, err := c.secretLister.Secrets(ref.Namespace).Get(ref.Name)
	if err != nil && apierrors.IsNotFound(err) {
		glog.V(3).Infof("No secret %q: found for MachineClass %q", ref, MachineClassName)
		return nil, nil
	} else if err != nil {
		glog.Errorf("Unable get secret %q for MachineClass %q: %v", MachineClassName, ref, err)
		return nil, err
	}
	return secretRef, err
}

// nodeConditionsHaveChanged compares two node statuses to see if any of the statuses have changed
func nodeConditionsHaveChanged(machineConditions []v1.NodeCondition, nodeConditions []v1.NodeCondition) bool {

	if len(machineConditions) != len(nodeConditions) {
		return true
	}

	for i := range nodeConditions {
		if nodeConditions[i].Status != machineConditions[i].Status {
			return true
		}
	}

	return false
}

// syncMachineNodeTemplate syncs nodeTemplates between machine and corresponding node-object.
// It ensures, that any nodeTemplate element available on Machine should be available on node-object.
// Although there could be more elements already available on node-object which will not be touched.
func (c *controller) syncMachineNodeTemplates(machine *v1alpha1.Machine) (bool, error) {
	var (
		initializedNodeAnnotation   bool
		currentlyAppliedALTJSONByte []byte
		lastAppliedALT              v1alpha1.NodeTemplateSpec
	)

	node, err := c.nodeLister.Get(machine.Status.Node)
	if err != nil && apierrors.IsNotFound(err) {
		// Dont return error so that other steps can be executed.
		return DoNotRetryOp, nil
	}
	if err != nil {
		glog.Errorf("Error occurred while trying to fetch node object - err: %s", err)
		return DoNotRetryOp, err
	}

	nodeCopy := node.DeepCopy()

	// Initialize node annotations if empty
	if nodeCopy.Annotations == nil {
		nodeCopy.Annotations = make(map[string]string)
		initializedNodeAnnotation = true
	}

	// Extracts the last applied annotations to lastAppliedLabels
	lastAppliedALTJSONString, exists := node.Annotations[LastAppliedALTAnnotation]
	if exists {
		err = json.Unmarshal([]byte(lastAppliedALTJSONString), &lastAppliedALT)
		if err != nil {
			glog.Errorf("Error occurred while syncing node annotations, labels & taints: %s", err)
			return RetryOp, err
		}
	}

	annotationsChanged := SyncMachineAnnotations(machine, nodeCopy, lastAppliedALT.Annotations)
	labelsChanged := SyncMachineLabels(machine, nodeCopy, lastAppliedALT.Labels)
	taintsChanged := SyncMachineTaints(machine, nodeCopy, lastAppliedALT.Spec.Taints)

	// Update node-object with latest nodeTemplate elements if elements have changed.
	if initializedNodeAnnotation || labelsChanged || annotationsChanged || taintsChanged {

		glog.V(2).Infof(
			"Updating machine annotations:%v, labels:%v, taints:%v for machine: %q",
			annotationsChanged,
			labelsChanged,
			taintsChanged,
			machine.Name,
		)

		// Update the LastAppliedALTAnnotation
		lastAppliedALT = machine.Spec.NodeTemplateSpec
		currentlyAppliedALTJSONByte, err = json.Marshal(lastAppliedALT)
		if err != nil {
			glog.Errorf("Error occurred while syncing node annotations, labels & taints: %s", err)
			return RetryOp, err
		}
		nodeCopy.Annotations[LastAppliedALTAnnotation] = string(currentlyAppliedALTJSONByte)

		_, err := c.targetCoreClient.Core().Nodes().Update(nodeCopy)
		if err != nil {
			// Keep retrying until update goes through
			glog.Errorf("Updated failed for node object of machine %q. Retrying, error: %q", machine.Name, err)
		} else {
			// Return error even when machine object is updated
			err = fmt.Errorf("Machine ALTs have been reconciled")
		}

		return RetryOp, err
	}

	return DoNotRetryOp, nil
}

// SyncMachineAnnotations syncs the annotations of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineAnnotations(
	machine *v1alpha1.Machine,
	node *v1.Node,
	lastAppliedAnnotations map[string]string,
) bool {
	toBeUpdated := false
	mAnnotations, nAnnotations := machine.Spec.NodeTemplateSpec.Annotations, node.Annotations

	// Initialize node annotations if nil
	if nAnnotations == nil {
		nAnnotations = make(map[string]string)
		node.Annotations = nAnnotations
	}
	// Intialize machine annotations to empty map if nil
	if mAnnotations == nil {
		mAnnotations = emptyMap
	}

	// Delete any annotation that existed in the past but has been deleted now
	for lastAppliedAnnotationKey := range lastAppliedAnnotations {
		if _, exists := mAnnotations[lastAppliedAnnotationKey]; !exists {
			delete(nAnnotations, lastAppliedAnnotationKey)
			toBeUpdated = true
		}
	}

	// Add/Update any key that doesn't exist or whose value as changed
	for mKey, mValue := range mAnnotations {
		if nValue, exists := nAnnotations[mKey]; !exists || mValue != nValue {
			nAnnotations[mKey] = mValue
			toBeUpdated = true
		}
	}

	return toBeUpdated
}

// SyncMachineLabels syncs the labels of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineLabels(
	machine *v1alpha1.Machine,
	node *v1.Node,
	lastAppliedLabels map[string]string,
) bool {
	toBeUpdated := false
	mLabels, nLabels := machine.Spec.NodeTemplateSpec.Labels, node.Labels

	// Initialize node labels if nil
	if nLabels == nil {
		nLabels = make(map[string]string)
		node.Labels = nLabels
	}
	// Intialize machine labels to empty map if nil
	if mLabels == nil {
		mLabels = emptyMap
	}

	// Delete any labels that existed in the past but has been deleted now
	for lastAppliedLabelKey := range lastAppliedLabels {
		if _, exists := mLabels[lastAppliedLabelKey]; !exists {
			delete(nLabels, lastAppliedLabelKey)
			toBeUpdated = true
		}
	}

	// Add/Update any key that doesn't exist or whose value as changed
	for mKey, mValue := range mLabels {
		if nValue, exists := nLabels[mKey]; !exists || mValue != nValue {
			nLabels[mKey] = mValue
			toBeUpdated = true
		}
	}

	return toBeUpdated
}

type taintKeyEffect struct {
	// Required. The taint key to be applied to a node.
	Key string
	// Valid effects are NoSchedule, PreferNoSchedule and NoExecute.
	Effect v1.TaintEffect
}

// SyncMachineTaints syncs the taints of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineTaints(
	machine *v1alpha1.Machine,
	node *v1.Node,
	lastAppliedTaints []v1.Taint,
) bool {
	toBeUpdated := false
	mTaints, nTaints := machine.Spec.NodeTemplateSpec.Spec.Taints, node.Spec.Taints
	mTaintsMap := make(map[taintKeyEffect]*v1.Taint, 0)
	nTaintsMap := make(map[taintKeyEffect]*v1.Taint, 0)

	// Convert the slice of taints to map of taint [key, effect] = Taint
	// Helps with indexed searching
	for i := range mTaints {
		mTaint := &mTaints[i]
		taintKE := taintKeyEffect{
			Key:    mTaint.Key,
			Effect: mTaint.Effect,
		}
		mTaintsMap[taintKE] = mTaint
	}
	for i := range nTaints {
		nTaint := &nTaints[i]
		taintKE := taintKeyEffect{
			Key:    nTaint.Key,
			Effect: nTaint.Effect,
		}
		nTaintsMap[taintKE] = nTaint
	}

	// Delete taints that existed on the machine object in the last update but deleted now
	for _, lastAppliedTaint := range lastAppliedTaints {

		lastAppliedKE := taintKeyEffect{
			Key:    lastAppliedTaint.Key,
			Effect: lastAppliedTaint.Effect,
		}

		if _, exists := mTaintsMap[lastAppliedKE]; !exists {
			delete(nTaintsMap, lastAppliedKE)
			toBeUpdated = true
		}
	}

	// Add any taints that exists in the machine object but not on the node object
	for mKE, mV := range mTaintsMap {
		if nV, exists := nTaintsMap[mKE]; !exists || *nV != *mV {
			nTaintsMap[mKE] = mV
			toBeUpdated = true
		}
	}

	if toBeUpdated {
		// Convert the map of taints to slice of taints
		nTaints = make([]v1.Taint, len(nTaintsMap))
		i := 0
		for _, nV := range nTaintsMap {
			nTaints[i] = *nV
			i++
		}
		node.Spec.Taints = nTaints
	}

	return toBeUpdated
}

// machineCreateErrorHandler TODO
func (c *controller) machineCreateErrorHandler(machine *v1alpha1.Machine, lastKnownState string, err error) (bool, error) {
	var retryRequired = DoNotRetryOp

	if grpcErr, ok := status.FromError(err); ok {
		switch grpcErr.Code() {
		case codes.Unknown, codes.DeadlineExceeded, codes.Aborted, codes.Unavailable:
			retryRequired = RetryOp
		}
	}

	clone := machine.DeepCopy()
	clone.Status.LastOperation = v1alpha1.LastOperation{
		Description:    "Cloud provider message - " + err.Error(),
		State:          v1alpha1.MachineStateFailed,
		Type:           v1alpha1.MachineOperationCreate,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
		Phase: v1alpha1.MachineFailed,
		//TimeoutActive:  false,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.LastKnownState = lastKnownState

	_, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.Errorf("Machine/status UPDATE failed for machine %q. Retrying, error: %s", machine.Name, err)
	} else {
		glog.V(2).Infof("Machine/status UPDATE for %q during CREATE error", machine.Name)
	}

	return retryRequired, err
}

// reconcileMachineHealth updates the machine object with
// any change in node conditions or health
func (c *controller) reconcileMachineHealth(machine *v1alpha1.Machine) (bool, error) {
	var (
		objectRequiresUpdate = false
		clone                = machine.DeepCopy()
		description          string
		lastOperationType    v1alpha1.MachineOperationType
	)

	node, err := c.nodeLister.Get(machine.Status.Node)
	if err == nil {
		if nodeConditionsHaveChanged(machine.Status.Conditions, node.Status.Conditions) {
			clone.Status.Conditions = node.Status.Conditions
			glog.V(3).Infof("Machine %q conditions are changing", machine.Name)
			objectRequiresUpdate = true
		}

		if !c.isHealthy(clone) && clone.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
			// If machine is not healthy, and current state is running,
			// change the machinePhase to unknown and activate health check timeout
			description = fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", clone.Name)
			glog.Warning(description)

			clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
				Phase: v1alpha1.MachineUnknown,
				//TimeoutActive:  true,
				LastUpdateTime: metav1.Now(),
			}
			clone.Status.LastOperation = v1alpha1.LastOperation{
				Description:    description,
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
				description = fmt.Sprintf("Machine %s successfully joined the cluster", clone.Name)
				lastOperationType = v1alpha1.MachineOperationCreate
			} else {
				// Machine rejoined the cluster after a healthcheck
				description = fmt.Sprintf("Machine %s successfully re-joined the cluster", clone.Name)
				lastOperationType = v1alpha1.MachineOperationHealthCheck
			}
			glog.V(2).Info(description)

			// Machine is ready and has joined/re-joined the cluster
			clone.Status.LastOperation = v1alpha1.LastOperation{
				Description:    description,
				State:          v1alpha1.MachineStateSuccessful,
				Type:           lastOperationType,
				LastUpdateTime: metav1.Now(),
			}
			clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
				Phase: v1alpha1.MachineRunning,
				//TimeoutActive:  false,
				LastUpdateTime: metav1.Now(),
			}
			objectRequiresUpdate = true
		}

	} else if err != nil && apierrors.IsNotFound(err) {
		// Node object is not found

		if len(machine.Status.Conditions) > 0 &&
			machine.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
			// If machine has conditions on it,
			// and corresponding node object went missing
			// and if machine object still reports healthy
			description = fmt.Sprintf(
				"Node object went missing. Machine %s is unhealthy - changing MachineState to Unknown",
				machine.Name,
			)
			glog.Warning(description)

			clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
				Phase: v1alpha1.MachineUnknown,
				//TimeoutActive:  true,
				LastUpdateTime: metav1.Now(),
			}
			clone.Status.LastOperation = v1alpha1.LastOperation{
				Description:    description,
				State:          v1alpha1.MachineStateProcessing,
				Type:           v1alpha1.MachineOperationHealthCheck,
				LastUpdateTime: metav1.Now(),
			}
			objectRequiresUpdate = true
		}

	} else {
		// Any other types of errors while fetching node object
		glog.Errorf("Could not fetch node object for machine %q", machine.Name)
		return RetryOp, err
	}

	if !objectRequiresUpdate &&
		(machine.Status.CurrentStatus.Phase == v1alpha1.MachinePending ||
			machine.Status.CurrentStatus.Phase == v1alpha1.MachineUnknown) {
		var (
			description     string
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

			// Log the error message for machine failure
			glog.Error(description)

			clone.Status.LastOperation = v1alpha1.LastOperation{
				Description:    description,
				State:          v1alpha1.MachineStateFailed,
				Type:           machine.Status.LastOperation.Type,
				LastUpdateTime: metav1.Now(),
			}
			clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
				Phase: v1alpha1.MachineFailed,
				//TimeoutActive:  false,
				LastUpdateTime: metav1.Now(),
			}
			objectRequiresUpdate = true
		} else {
			// If timeout has not occurred, re-enqueue the machine
			// after a specified sleep time
			c.enqueueMachineAfter(machine, sleepTime)
		}
	}

	if objectRequiresUpdate {
		_, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
		if err != nil {
			// Keep retrying until update goes through
			glog.Errorf("Update failed for machine %q. Retrying, error: %q", machine.Name, err)
		} else {
			glog.V(2).Infof("Machine State has been updated for %q", machine.Name)
			// Return error for continuing in next iteration
			err = fmt.Errorf("Machine creation is successful. Machine State has been UPDATED")
		}

		return RetryOp, err
	}

	return DoNotRetryOp, nil
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addMachineFinalizers(machine *v1alpha1.Machine) (bool, error) {
	if finalizers := sets.NewString(machine.Finalizers...); !finalizers.Has(DeleteFinalizerName) {

		finalizers.Insert(DeleteFinalizerName)
		clone := machine.DeepCopy()
		clone.Finalizers = finalizers.List()
		_, err := c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			// Keep retrying until update goes through
			glog.Errorf("Failed to add finalizers for machine %q: %s", machine.Name, err)
		} else {
			// Return error even when machine object is updated
			glog.V(2).Infof("Added finalizer to machine %q", machine.Name)
			err = fmt.Errorf("Machine creation in process. Machine finalizers are UPDATED")
		}

		return RetryOp, err
	}

	return DoNotRetryOp, nil
}

func (c *controller) deleteMachineFinalizers(machine *v1alpha1.Machine) (bool, error) {
	if finalizers := sets.NewString(machine.Finalizers...); finalizers.Has(DeleteFinalizerName) {

		finalizers.Delete(DeleteFinalizerName)
		clone := machine.DeepCopy()
		clone.Finalizers = finalizers.List()
		_, err := c.controlMachineClient.Machines(clone.Namespace).Update(clone)
		if err != nil {
			// Keep retrying until update goes through
			glog.Errorf("Failed to delete finalizers for machine %q: %s", machine.Name, err)
			return RetryOp, err
		}

		glog.V(2).Infof("Removed finalizer to machine %q", machine.Name)
		return DoNotRetryOp, nil
	}

	return DoNotRetryOp, nil
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

/*
	SECTION
	Delete machine
*/

// setMachineTerminationStatus set's the machine status to terminating
func (c *controller) setMachineTerminationStatus(machine *v1alpha1.Machine, driver cmiclient.CMIClient) (bool, error) {
	clone := machine.DeepCopy()
	clone.Status.LastOperation = v1alpha1.LastOperation{
		Description:    getVMStatus,
		State:          v1alpha1.MachineStateProcessing,
		Type:           v1alpha1.MachineOperationDelete,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
		Phase: v1alpha1.MachineTerminating,
		//TimeoutActive:  false,
		LastUpdateTime: metav1.Now(),
	}

	_, err := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.Errorf("Machine/status UPDATE failed for machine %q. Retrying, error: %s", machine.Name, err)
	} else {
		glog.V(2).Infof("Machine %q status updated to terminating ", machine.Name)
		// Return error even when machine object is updated to ensure reconcilation is restarted
		err = fmt.Errorf("Machine deletion in process. Phase set to termination")
	}
	return RetryOp, err
}

// getVMStatus tries to retrive VM status backed by machine
func (c *controller) getVMStatus(machine *v1alpha1.Machine, driver cmiclient.CMIClient) (bool, error) {
	var (
		retry       bool
		description string
		state       v1alpha1.MachineState
		phase       v1alpha1.MachinePhase
	)

	_, _, _, err := driver.GetMachineStatus()
	if err == nil {
		// VM Found
		description = initiateDrain
		state = v1alpha1.MachineStateProcessing
		retry = RetryOp
		phase = v1alpha1.MachineTerminating
		// Return error even when machine object is updated to ensure reconcilation is restarted
		err = fmt.Errorf("Machine deletion in process. VM with matching ID found")

	} else {
		if grpcErr, ok := status.FromError(err); !ok {
			// Error occurred with decoding gRPC error status, aborting without retry.
			description = "Error occurred with decoding gRPC error status while getting VM status, aborting without retry. " + getVMStatus
			state = v1alpha1.MachineStateFailed
			phase = v1alpha1.MachineFailed
			retry = DoNotRetryOp

			err = fmt.Errorf("Machine deletion has failed. " + description)
		} else {
			// Decoding gRPC error code
			switch grpcErr.Code() {

			case codes.Unimplemented:
				// GetMachineStatus() call is not implemented
				// In this case, try to drain and delete
				description = initiateDrain
				state = v1alpha1.MachineStateProcessing
				phase = v1alpha1.MachineTerminating
				retry = RetryOp

			case codes.NotFound:
				// VM was not found at provder
				description = "VM was not found at provider. " + initiateNodeDeletion
				state = v1alpha1.MachineStateProcessing
				phase = v1alpha1.MachineTerminating
				retry = RetryOp

			case codes.Unknown, codes.DeadlineExceeded, codes.Aborted, codes.Unavailable:
				description = "Error occurred with decoding gRPC error status while getting VM status, aborting with retry. " + getVMStatus
				state = v1alpha1.MachineStateFailed
				phase = v1alpha1.MachineTerminating
				retry = RetryOp

			default:
				// Error occurred with decoding gRPC error status, abort with retry.
				description = "Error occurred with decoding gRPC error status while getting VM status, aborting without retry. gRPC code: " + grpcErr.Message() + " " + getVMStatus
				state = v1alpha1.MachineStateFailed
				phase = v1alpha1.MachineTerminating
				retry = DoNotRetryOp
			}
		}

	}

	clone := machine.DeepCopy()
	clone.Status.LastOperation = v1alpha1.LastOperation{
		Description:    description,
		State:          state,
		Type:           v1alpha1.MachineOperationDelete,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
		Phase:          phase,
		LastUpdateTime: metav1.Now(),
	}

	_, updateErr := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
	if updateErr != nil {
		// Keep retrying until update goes through
		glog.Errorf("Machine/status UPDATE failed for machine %q. Retrying, error: %s", machine.Name, updateErr)
	}

	return retry, err
}

// drainNode attempts to drain the node backed by the machine object
func (c *controller) drainNode(machine *v1alpha1.Machine, driver cmiclient.CMIClient) (bool, error) {
	var (
		forceDeletePods         = false
		forceDeleteMachine      = false
		timeOutOccurred         = false
		description             = ""
		state                   v1alpha1.MachineState
		phase                   v1alpha1.MachinePhase
		maxEvictRetries         = c.safetyOptions.MaxEvictRetries
		pvDetachTimeOut         = c.safetyOptions.PvDetachTimeout.Duration
		timeOutDuration         = c.safetyOptions.MachineDrainTimeout.Duration
		forceDeleteLabelPresent = machine.Labels["force-deletion"] == "True"
		nodeName                = machine.Labels["node"]
	)

	// Timeout value obtained by subtracting last operation with expected time out period
	timeOut := metav1.Now().Add(-timeOutDuration).Sub(machine.Status.CurrentStatus.LastUpdateTime.Time)
	timeOutOccurred = timeOut > 0

	if forceDeleteLabelPresent || timeOutOccurred {
		// To perform forceful machine drain/delete either one of the below conditions must be satified
		// 1. force-deletion: "True" label must be present
		// 2. Deletion operation is more than drain-timeout minutes old
		// 3. Last machine drain had failed
		forceDeleteMachine = true
		forceDeletePods = true
		timeOutDuration = 1 * time.Minute
		maxEvictRetries = 1

		glog.V(2).Infof(
			"Force deletion has been triggerred for machine %q due to Label:%t, timeout:%t",
			machine.Name,
			forceDeleteLabelPresent,
			timeOutOccurred,
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
	err := drainOptions.RunDrain()
	if err == nil {
		// Drain successful
		glog.V(2).Infof("Drain successful for machine %q. \nBuf:%v \nErrBuf:%v", machine.Name, buf, errBuf)

		description = fmt.Sprintf("Drain successful. %s", initiateVMDeletion)
		state = v1alpha1.MachineStateProcessing
		phase = v1alpha1.MachineTerminating

		// Return error even when machine object is updated
		err = fmt.Errorf("Machine deletion in process. " + description)
	} else if err != nil && forceDeleteMachine {
		// Drain failed on force deletion
		glog.Warningf("Drain failed for machine %q. However, since it's a force deletion shall continue deletion of VM. \nBuf:%v \nErrBuf:%v \nErr-Message:%v", machine.Name, buf, errBuf, err)

		description = fmt.Sprintf("Drain failed due to - %s. However, since it's a force deletion shall continue deletion of VM. %s", err.Error(), initiateVMDeletion)
		state = v1alpha1.MachineStateProcessing
		phase = v1alpha1.MachineTerminating
	} else {
		glog.Warningf("Drain failed for machine %q. \nBuf:%v \nErrBuf:%v \nErr-Message:%v", machine.Name, buf, errBuf, err)

		description = fmt.Sprintf("Drain failed due to - %s. Will retry in next sync. %s", err.Error(), initiateDrain)
		state = v1alpha1.MachineStateFailed
		phase = v1alpha1.MachineTerminating
	}

	clone := machine.DeepCopy()
	clone.Status.LastOperation = v1alpha1.LastOperation{
		Description:    description,
		State:          state,
		Type:           v1alpha1.MachineOperationDelete,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
		Phase:          phase,
		LastUpdateTime: metav1.Now(),
	}

	_, updateErr := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
	if updateErr != nil {
		// Keep retrying until update goes through
		glog.Errorf("Machine/status UPDATE failed for machine %q. Retrying, error: %s", machine.Name, updateErr)
	}

	return RetryOp, err
}

// deleteVM attempts to delete the VM backed by the machine object
func (c *controller) deleteVM(machine *v1alpha1.Machine, driver cmiclient.CMIClient) (bool, error) {
	var (
		retryRequired bool
		description   string
		state         v1alpha1.MachineState
		phase         v1alpha1.MachinePhase
	)

	lastKnownState, err := driver.DeleteMachine()
	if err != nil {

		glog.Errorf("Error while deleting machine %s: %s", machine.Name, err)

		if grpcErr, ok := status.FromError(err); ok {
			switch grpcErr.Code() {
			case codes.Unknown, codes.DeadlineExceeded, codes.Aborted, codes.Unavailable:
				retryRequired = RetryOp
				description = fmt.Sprintf("VM deletion failed due to - %s. However, will re-try in the next resync. %s", err.Error(), initiateVMDeletion)
				state = v1alpha1.MachineStateFailed
				phase = v1alpha1.MachineTerminating
			case codes.NotFound:
				retryRequired = RetryOp
				description = fmt.Sprintf("VM not found. Continuing deletion flow. %s", initiateNodeDeletion)
				state = v1alpha1.MachineStateProcessing
				phase = v1alpha1.MachineTerminating
			default:
				retryRequired = DoNotRetryOp
				description = fmt.Sprintf("VM deletion failed due to - %s. Aborting operation. %s", err.Error(), initiateVMDeletion)
				state = v1alpha1.MachineStateFailed
				phase = v1alpha1.MachineTerminating
			}
		} else {
			retryRequired = DoNotRetryOp
			description = fmt.Sprintf("Error occurred while decoding gRPC error: %s. %s", err.Error(), initiateVMDeletion)
			state = v1alpha1.MachineStateFailed
			phase = v1alpha1.MachineFailed
		}

	} else {
		retryRequired = RetryOp
		description = fmt.Sprintf("VM deletion was successful. %s", initiateNodeDeletion)
		state = v1alpha1.MachineStateProcessing
		phase = v1alpha1.MachineTerminating

		err = fmt.Errorf("Machine deletion in process. " + description)
	}

	clone := machine.DeepCopy()
	clone.Status.LastOperation = v1alpha1.LastOperation{
		Description:    description,
		State:          state,
		Type:           v1alpha1.MachineOperationDelete,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
		Phase:          phase,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.LastKnownState = lastKnownState

	_, updateErr := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
	if updateErr != nil {
		// Keep retrying until update goes through
		glog.Errorf("Machine/status UPDATE failed for machine %q. Retrying, error: %s", machine.Name, updateErr)
	}

	return retryRequired, err
}

// deleteNodeObject attempts to delete the node object backed by the machine object
func (c *controller) deleteNodeObject(machine *v1alpha1.Machine) (bool, error) {
	var (
		err         error
		description string
		state       v1alpha1.MachineState
	)

	nodeName := machine.Labels["node"]

	if nodeName != "" {
		// Delete node object
		err = c.targetCoreClient.CoreV1().Nodes().Delete(nodeName, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// If its an error, and anyother error than object not found
			description = fmt.Sprintf("Deletion of Node Object %q failed due to error: %s. %s", nodeName, err, initiateNodeDeletion)
			state = v1alpha1.MachineStateFailed
		} else if err == nil {
			description = fmt.Sprintf("Deletion of Node Object %q is successful. %s", nodeName, initiateFinalizerRemoval)
			state = v1alpha1.MachineStateProcessing

			err = fmt.Errorf("Machine deletion in process. Deletion of node object was succesful")
		} else {
			description = fmt.Sprintf("No node object found for %q, continuing deletion flow. %s", nodeName, initiateFinalizerRemoval)
			state = v1alpha1.MachineStateProcessing
		}
	} else {
		description = fmt.Sprintf("No node object found for machine, continuing deletion flow. %s", initiateFinalizerRemoval)
		state = v1alpha1.MachineStateProcessing

		err = fmt.Errorf("Machine deletion in process. No node object found")
	}

	clone := machine.DeepCopy()
	clone.Status.LastOperation = v1alpha1.LastOperation{
		Description:    description,
		State:          state,
		Type:           v1alpha1.MachineOperationDelete,
		LastUpdateTime: metav1.Now(),
	}
	clone.Status.CurrentStatus = v1alpha1.CurrentStatus{
		Phase:          v1alpha1.MachineTerminating,
		LastUpdateTime: metav1.Now(),
	}

	_, updateErr := c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(clone)
	if updateErr != nil {
		// Keep retrying until update goes through
		glog.Errorf("Machine/status UPDATE failed for machine %q. Retrying, error: %s", machine.Name, updateErr)
	}

	return RetryOp, err
}
