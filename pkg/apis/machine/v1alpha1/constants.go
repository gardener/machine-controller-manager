// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// AnnotationKeyMachineUpdateFailedReason is the annotation key that indicates the reason for a machine update failure.
	AnnotationKeyMachineUpdateFailedReason = "node.machine.sapcloud.io/update-failed-reason"
	// AnnotationKeyMachineEffectiveCreationTimeout is the annotation key set on the MachineDeployment that indicates
	// the effective-creation-timeout for all Machine's belonging to this MachineDeployment. If specified, the value for this
	// annotation takes precedence over the MachineDeployment.Spec.Template.Spec.MachineCreationTimeout.
	AnnotationKeyMachineEffectiveCreationTimeout = "node.machine.sapcloud.io/effective-creation-timeout"
	// AnnotationKeyMachineEffectiveCreationTimeoutLastAdjustedAt is the annotation key set on the MachineDeployment that indicates
	// the time that the effective-creation-timeout annotation was last adjusted for this MachineDeployment.
	AnnotationKeyMachineEffectiveCreationTimeoutLastAdjustedAt = "node.machine.sapcloud.io/effective-creation-timeout-last-adjusted-at"
	// AnnotationKeyMachineReplaceCycleCount is the annotation key set on the MachineDeployment that indicates
	// the number of times Machine's in this MachineDeployment have undergone a replace-cycle caused by failure within
	// the current effective-creation-timeout for this machine deployment.
	AnnotationKeyMachineReplaceCycleCount = "node.machine.sapcloud.io/replace-cycle-count"
	// AnnotationKeyMachineReplaceCycleCountLastAdjustedAt is the annotation key set on the MachineDeployment that indicates
	// the time that the 'replace-cycle-count' was last adjusted for this MachineDeployment.
	AnnotationKeyMachineReplaceCycleCountLastAdjustedAt = "node.machine.sapcloud.io/replace-cycle-count-last-adjusted-at"
	// AnnotationKeyMachineMaxJoinDuration is the annotation key set on the MachineDeployment that indicates the maximum amount
	//  of time a Machine took to join the cluster within an effective-creation-timeout window. The value is a Go Duration string.
	AnnotationKeyMachineMaxJoinDuration = "node.machine.sapcloud.io/machine-max-join-duration"
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
