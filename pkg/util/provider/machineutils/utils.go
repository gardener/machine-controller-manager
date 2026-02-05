// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package machineutils contains the consts and global vaariables for machine operation
package machineutils

import (
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
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

	//RemoveNodeFinalizers specifies next step as removing node finalizers
	RemoveNodeFinalizers = "Remove node finalizers"

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

	// TriggerDeletionByMCM annotation on the machine would trigger the deletion of the corresponding node and machine object in the control cluster
	// This annotation can also be set on the MachineDeployment and contains the machine names for which deletion should be triggered.
	// The latter feature is leveraged by the CA-MCM cloud provider.
	TriggerDeletionByMCM = "node.machine.sapcloud.io/trigger-deletion-by-mcm"

	// NodeUnhealthy is a node termination reason for failed machines
	NodeUnhealthy = "Unhealthy"

	// NodeScaledDown is a node termination reason for healthy deleted machines
	NodeScaledDown = "ScaleDown"

	// NodeTerminationCondition describes nodes that are terminating
	NodeTerminationCondition v1.NodeConditionType = "Terminating"

	// TaintNodeCriticalComponentsNotReady is the name of a gardener taint
	// indicating that a node is not yet ready to have user workload scheduled
	TaintNodeCriticalComponentsNotReady = "node.gardener.cloud/critical-components-not-ready"

	// MachineLabelKey defines the labels which contains the name of the machine of a node
	MachineLabelKey = "node.gardener.cloud/machine-name"

	// LabelKeyMachineSetScaleUpDisabled is the label key that indicates scaling up of the machine set is disabled.
	LabelKeyMachineSetScaleUpDisabled = "node.machine.sapcloud.io/scale-up-disabled"
)

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
	return m.Annotations[MachinePriority] == "1" || m.Annotations[TriggerDeletionByMCM] == "true"
}
