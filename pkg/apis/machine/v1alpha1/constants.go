// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// AnnotationKeyMachineUpdateFailedReason is the annotation key that indicates the reason for a machine update failure.
	AnnotationKeyMachineUpdateFailedReason = "node.machine.sapcloud.io/update-failed-reason"
	// AnnotationKeyMachineEffectiveCreationTimeout is the annotation key set on the MachineDeployment that indicates
	// the effective creation timeout for all Machine's belonging to this MachineDeployment. If specified, the value for this
	// annotation takes precedence over the MachineDeployment.Spec.Template.Spec.MachineCreationTimeout.
	AnnotationKeyMachineEffectiveCreationTimeout = "node.machine.sapcloud.io/effective-creation-timeout"
	// AnnotationKeyMachineJoinDuration is the annotation key set on the Machine that indicates the amount of time Machine
	// took to join the cluster. The value is a Go Duration string.
	AnnotationKeyMachineJoinDuration = "node.machine.sapcloud.io/machine-join-duration"
	// LabelKeyNodeCandidateForUpdate is the label key that indicates a node is a candidate for update.
	LabelKeyNodeCandidateForUpdate = "node.machine.sapcloud.io/candidate-for-update"
	// LabelKeyNodeSelectedForUpdate is the label key that indicates a node has been selected for update.
	LabelKeyNodeSelectedForUpdate = "node.machine.sapcloud.io/selected-for-update"
	// LabelKeyNodeUpdateResult is the label key that indicates the result of the update on the node.
	LabelKeyNodeUpdateResult = "node.machine.sapcloud.io/update-result"

	// LabelValueNodeUpdateSuccessful is the label value that indicates the update on the node has succeeded.
	LabelValueNodeUpdateSuccessful = "successful"
	// LabelValueNodeUpdateFailed is the label value that indicates the update on the node has failed.
	LabelValueNodeUpdateFailed = "failed"
)
