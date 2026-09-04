// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package machineutils contains the consts and global vaariables for machine operation
package machineutils

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1alpha1client "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	// GetVMStatus sets machine status to terminating and specifies next step as getting VMs
	GetVMStatus = "Set machine status to termination. Now, getting VM Status"

	// InstanceInitialization is a step that represents initialization of a VM instance (post-creation).
	InstanceInitialization = "Initialize VM Instance"

	// InitiateDrain specifies next step as initiate node drain
	InitiateDrain = "Initiate node drain"

	// NodeReadyForUpdate specifies next step as node ready for update.
	NodeReadyForUpdate = "Node drain successful. Node is ready for update"

	// DelVolumesAttachments specifies next step as deleting volume attachments
	DelVolumesAttachments = "Delete Volume Attachments"

	// InitiateVMDeletion specifies next step as initiate VM deletion
	InitiateVMDeletion = "Initiate VM deletion"

	// InitiateNodeDeletion specifies next step as node object deletion
	InitiateNodeDeletion = "Initiate node object deletion"

	// InitiateFinalizerRemoval specifies next step as machine finalizer removal
	InitiateFinalizerRemoval = "Initiate machine object finalizer removal"

	// LastAppliedALTAnnotation contains the last configuration of annotations, labels & taints applied on the node object
	LastAppliedALTAnnotation = "node.machine.sapcloud.io/last-applied-anno-labels-taints"

	// LastAppliedVirtualCapacityAnnotation contains the last configuration of MachineClass.NodeTemplate.VirtualCapacity applied on the node object
	LastAppliedVirtualCapacityAnnotation = "node.machine.sapcloud.io/last-applied-virtual-capacity"

	// MachinePriority is the annotation used to specify priority
	// associated with a machine while deleting it. The less its
	// priority the more likely it is to be deleted first
	// Default priority for a machine is set to 3
	MachinePriority = "machinepriority.machine.sapcloud.io"

	// MachineClassKind is used to identify the machineClassKind for generic machineClasses
	MachineClassKind = "MachineClass"

	// NotManagedByMCM annotation helps in identifying the nodes which are not handled by MCM
	NotManagedByMCM = "node.machine.sapcloud.io/not-managed-by-mcm"

	// TriggerDeletionByMCM is the annotation set on the MachineDeployment by the CA-MCM cloud provider. It contains the machine names
	// for which deletion should be triggered along with the time when CA decided to scale-down those machines.
	// Expected format for this annotation value is [M1~T1,M2~T2,...]
	TriggerDeletionByMCM = "node.machine.sapcloud.io/trigger-deletion-by-mcm"

	// MarkedForDeletionTime is the annotation used to specify the time when machine was marked for deletion.
	// This is used by MCS to delete the machines which were marked for deletion before the MCS saw the replica change.
	MarkedForDeletionTime = "machine.sapcloud.io/marked-for-deletion-time"

	// LastDeploymentReplicaChangeByScalerTime is the annotation used to specify the time when machineDeployment replica change was triggered by a scaler.
	LastDeploymentReplicaChangeByScalerTime = "machine.sapcloud.io/last-deployment-replica-change-by-scaler-time"

	// NodeUnhealthy is a node termination reason for failed machines
	NodeUnhealthy = "Unhealthy"

	// NodeScaledDown is a node termination reason for healthy deleted machines
	NodeScaledDown = "ScaleDown"

	// NodeTerminationCondition describes nodes that are terminating
	NodeTerminationCondition corev1.NodeConditionType = "Terminating"

	// TaintNodeCriticalComponentsNotReady is the name of a gardener taint
	// indicating that a node is not yet ready to have user workload scheduled
	TaintNodeCriticalComponentsNotReady = "node.gardener.cloud/critical-components-not-ready"

	// MachineLabelKey defines the labels which contains the name of the machine of a node
	MachineLabelKey = "node.gardener.cloud/machine-name"

	// LabelKeyMachineSetScaleUpDisabled is the label key that indicates scaling up of the machine set is disabled.
	LabelKeyMachineSetScaleUpDisabled = "node.machine.sapcloud.io/scale-up-disabled"

	// PreserveMachineAnnotationKey is the annotation used to explicitly request that a Machine be preserved
	PreserveMachineAnnotationKey = "node.machine.sapcloud.io/preserve"

	// LastAppliedNodePreserveValueAnnotationKey is the annotation used to store the last preserve value applied by MCM
	LastAppliedNodePreserveValueAnnotationKey = "node.machine.sapcloud.io/last-applied-node-preserve-value"

	// PreserveMachineAnnotationValueNow is the annotation value used to explicitly request that
	// a Machine be preserved immediately, irrespective of its current phase, and its phase is not changed
	// on preservation
	PreserveMachineAnnotationValueNow = "now"

	// PreserveMachineAnnotationValueWhenFailed is the annotation value used to explicitly request that
	// a Machine be preserved if and when it enters Failed phase
	PreserveMachineAnnotationValueWhenFailed = "when-failed"

	// PreserveMachineAnnotationValueAutoPreserved is the annotation value used by the machineset controller to
	// indicate to the machine controller that the machine must be auto-preserved.
	// The AutoPreserveFailedMachineMax, set on the MCD, is enforced based on the number of machines annotated with this value.
	PreserveMachineAnnotationValueAutoPreserved = "auto-preserved"

	// PreserveMachineAnnotationValueFalse is the annotation value used to
	// 1) indicate to MCM that a machine must not be auto-preserved on failure
	// and, 2) to stop preservation of a machine that is already preserved by MCM.
	PreserveMachineAnnotationValueFalse = "false"

	// NodePreservedTaintKey is used to cordon a node when a Failed machine is preserved.
	// This taint is added to the node before draining it, and removed when the machine is unpreserved.
	NodePreservedTaintKey = "node.machine.sapcloud.io/preserved"
)

// AllowedPreserveAnnotationValues contains the allowed values for the preserve annotation
var AllowedPreserveAnnotationValues = sets.New(PreserveMachineAnnotationValueNow, PreserveMachineAnnotationValueWhenFailed, PreserveMachineAnnotationValueAutoPreserved, PreserveMachineAnnotationValueFalse)

// RetryPeriod is an alias for specifying the retry period
type RetryPeriod time.Duration

// These are the valid values for RetryPeriod
const (
	// ConflictRetry tells the controller to retry quickly - 200 milliseconds
	ConflictRetry RetryPeriod = RetryPeriod(200 * time.Millisecond)
	// ShortRetry tells the controller to retry after a short duration - 5 seconds
	ShortRetry RetryPeriod = RetryPeriod(5 * time.Second)
	// MediumRetry tells the controller to retry after a medium duration - 3 minutes
	MediumRetry RetryPeriod = RetryPeriod(3 * time.Minute)
	// LongRetry tells the controller to retry after a long duration - 10 minutes
	LongRetry RetryPeriod = RetryPeriod(10 * time.Minute)
)

// EssentialTaints are taints on node object which if added/removed, require an immediate reconcile by machine controller
// TODO: update this when taints for ALT updation and PostCreate operations is introduced.
var EssentialTaints = []string{TaintNodeCriticalComponentsNotReady}

// IsMachineFailedOrTerminating returns true if machine is Failed or already being Terminated.
func IsMachineFailedOrTerminating(machine *v1alpha1.Machine) bool {
	if !machine.GetDeletionTimestamp().IsZero() || machine.Status.CurrentStatus.Phase == v1alpha1.MachineFailed {
		return true
	}
	return false
}

// IsMachineActive checks if machine was active
func IsMachineActive(p *v1alpha1.Machine) bool {
	return p.Status.CurrentStatus.Phase != v1alpha1.MachineFailed && p.Status.CurrentStatus.Phase != v1alpha1.MachineTerminating
}

// IsMachineFailed checks if machine has failed
func IsMachineFailed(p *v1alpha1.Machine) bool {
	return p.Status.CurrentStatus.Phase == v1alpha1.MachineFailed
}

// IsMachineTriggeredForDeletion checks if machine was triggered for deletion
func IsMachineTriggeredForDeletion(m *v1alpha1.Machine) bool {
	return m.Annotations[MachinePriority] == "1"
}

// IsMachinePreservationExpired checks if the preserve expiry time has passed for a machine
func IsMachinePreservationExpired(m *v1alpha1.Machine) bool {
	t := m.Status.CurrentStatus.PreserveExpiryTime
	return t != nil && !t.After(time.Now())
}

// GetMachineDeploymentName gets the name of the MachineDeployment associated with this Machine
func GetMachineDeploymentName(machine *v1alpha1.Machine) string {
	return machine.Labels["name"]
}

// PatchMachine patches a machine using a strategic merge patch derived from mutateFn applied to the given machine object.
// If optimisticLock is true, the patch includes the current resourceVersion to detect concurrent updates.
func PatchMachine(ctx context.Context, machineClient v1alpha1client.MachineInterface, machine *v1alpha1.Machine, mutateFn func(*v1alpha1.Machine) error, optimisticLock bool, subresources ...string) (*v1alpha1.Machine, error) {
	base, err := json.Marshal(machine)
	if err != nil {
		return nil, err
	}
	modified := machine.DeepCopy()
	if err := mutateFn(modified); err != nil {
		return nil, err
	}
	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		return nil, err
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(base, modifiedJSON, v1alpha1.Machine{})
	if err != nil {
		return nil, err
	}
	if optimisticLock {
		var patchMap map[string]any
		if err := json.Unmarshal(patch, &patchMap); err != nil {
			return nil, err
		}
		meta, ok := patchMap["metadata"].(map[string]any)
		if !ok {
			meta = map[string]any{}
		}
		meta["resourceVersion"] = machine.ResourceVersion
		patchMap["metadata"] = meta
		patch, err = json.Marshal(patchMap)
		if err != nil {
			return nil, err
		}
	}
	return machineClient.Patch(ctx, machine.Name, types.MergePatchType, patch, metav1.PatchOptions{}, subresources...)
}

// GetPreserveAnnotationValue returns the preserve annotation value for the given node and machine
// and a boolean informing whether we need to do any work or skip.
// Invalid annotation values are treated as absent.
func GetPreserveAnnotationValue(node *corev1.Node, machine *v1alpha1.Machine) (annotationValue string, shouldHandlePreservation bool) {
	if node != nil {
		if val, ok :=
			node.Annotations[PreserveMachineAnnotationKey]; ok &&
			AllowedPreserveAnnotationValues.Has(val) {
			return val, true
		}
		if _, ok :=
			machine.Annotations[LastAppliedNodePreserveValueAnnotationKey]; ok {
			return "", true
		}
	}
	if val, ok :=
		machine.Annotations[PreserveMachineAnnotationKey]; ok &&
		AllowedPreserveAnnotationValues.Has(val) {
		return val, true
	}
	if machine.Status.CurrentStatus.PreserveExpiryTime != nil {
		return "", true
	}
	return "", false
}
