// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package machineutils contains the consts and global vaariables for machine operation
package machineutils

import (
	"context"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1alpha1client "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	v1alpha1listers "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"time"
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

	// TriggerDeletionByMCM annotation on the node or machine would trigger the deletion of the corresponding node and machine object in the control cluster
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

	// PreserveMachineAnnotationKey is the annotation used to explicitly request that a Machine be preserved
	PreserveMachineAnnotationKey = "node.machine.sapcloud.io/preserve"

	// PreserveMachineAnnotationValueNow is the annotation value used to explicitly request that
	// a Machine be preserved immediately in its current phase
	PreserveMachineAnnotationValueNow = "now"

	// PreserveMachineAnnotationValueWhenFailed is the annotation value used to explicitly request that
	// a Machine be preserved if and when in it enters Failed phase
	PreserveMachineAnnotationValueWhenFailed = "when-failed"

	// PreserveMachineAnnotationValuePreservedByMCM is the annotation value used to explicitly request that
	// a Machine be preserved if and when in it enters Failed phase.
	// The AutoPreserveFailedMachineMax, set on the MCD, is enforced based on the number of machines annotated with this value.
	PreserveMachineAnnotationValuePreservedByMCM = "auto-preserved"

	//PreserveMachineAnnotationValueFalse is the annotation value used to explicitly request that
	// a Machine should not be preserved any longer, even if the expiry timeout has not been reached
	PreserveMachineAnnotationValueFalse = "false"
)

// AllowedPreserveAnnotationValues contains the allowed values for the preserve annotation
var AllowedPreserveAnnotationValues = sets.New(PreserveMachineAnnotationValueNow, PreserveMachineAnnotationValueWhenFailed, PreserveMachineAnnotationValuePreservedByMCM, PreserveMachineAnnotationValueFalse)

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

// PreserveAnnotationsChanged returns true if there is a change in preserve annotations
func PreserveAnnotationsChanged(oldAnnotations, newAnnotations map[string]string) bool {
	return newAnnotations[PreserveMachineAnnotationKey] != oldAnnotations[PreserveMachineAnnotationKey]
}

// AnnotateMachineWithPreserveValueWithRetries annotates the given machine with the preservation annotation value
func AnnotateMachineWithPreserveValueWithRetries(ctx context.Context, machineClient v1alpha1client.MachineInterface, machineLister v1alpha1listers.MachineLister, m *v1alpha1.Machine, preserveValue string) (*v1alpha1.Machine, error) {
	updatedMachine, err := UpdateMachineWithRetries(ctx, machineClient, machineLister, m.Namespace, m.Name, func(clone *v1alpha1.Machine) error {
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[PreserveMachineAnnotationKey] = preserveValue
		return nil
	})
	if err != nil {
		return nil, err
	}
	klog.V(2).Infof("Updated machine %q with %q=%q.", m.Name, PreserveMachineAnnotationKey, preserveValue)
	return updatedMachine, nil
}

// see https://github.com/kubernetes/kubernetes/issues/21479
type updateMachineFunc func(machine *v1alpha1.Machine) error

// UpdateMachineWithRetries updates a machine with given applyUpdate function. Note that machine not found error is ignored.
// The returned bool value can be used to tell if the machine is actually updated.
func UpdateMachineWithRetries(ctx context.Context, machineClient v1alpha1client.MachineInterface, machineLister v1alpha1listers.MachineLister, namespace, name string, applyUpdate updateMachineFunc) (*v1alpha1.Machine, error) {
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
		machine, err = machineClient.Update(ctx, machine, metav1.UpdateOptions{})
		return err
	})

	// Ignore the precondition violated error, this machine is already updated
	// with the desired label.
	if retryErr == errorsutil.ErrPreconditionViolated {
		klog.V(4).Infof("Machine %s precondition doesn't hold, skip updating it.", name)
		retryErr = nil
	}

	return machine, retryErr
}
