// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// AnnotationKeyMachineUpdateFailedReason is the annotation key for the machine indicating the reason for the machine update failure.
	AnnotationKeyMachineUpdateFailedReason = "node.machine.sapcloud.io/update-failed-reason"

	// LabelKeyMachineCandidateForUpdate is the label key for the machine indicating the machine will undergo an update.
	LabelKeyMachineCandidateForUpdate = "node.machine.sapcloud.io/candidate-for-update"
	// LabelKeyMachineSelectedForUpdate is the label key for the machine indicating the machine is selected for update.
	LabelKeyMachineSelectedForUpdate = "node.machine.sapcloud.io/selected-for-update"
	// LabelKeyMachineReadyForUpdate is the label key for the machine indicating the machine is ready for update after the node drain.
	LabelKeyMachineReadyForUpdate = "node.machine.sapcloud.io/ready-for-update"
	// LabelKeyMachineDrainSuccessful is the label key for the machine indicating the node drain was successful.
	LabelKeyMachineDrainSuccessful = "node.machine.sapcloud.io/drain-successful"
	// LabelKeyMachineUpdateSuccessful is the label key for the machine indicating the machine update was successful.
	LabelKeyMachineUpdateSuccessful = "node.machine.sapcloud.io/update-successful"
	// LabelKeyMachineUpdateFailed is the label key for the machine indicating the machine update failed.
	LabelKeyMachineUpdateFailed = "node.machine.sapcloud.io/update-failed"
	// LabelKeyMachineSetSkipUpdate is the label key for the machine indicating the machine set update should be skipped.
	LabelKeyMachineSetSkipUpdate = "node.machine.sapcloud.io/machine-set-skip-update"
)
