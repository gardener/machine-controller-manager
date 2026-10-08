// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package constants

import "time"

const (
	// TargetKubeconfigDisabledValue is the value for the --target-kubeconfig file that disables all interaction with
	// the target cluster.
	TargetKubeconfigDisabledValue = "none"
	// DefaultMachineCreationTimeout is the default value for the machine creation timeout if un-specified.
	DefaultMachineCreationTimeout = 20 * time.Minute
	// DefaultCreationTimeoutGrowthPercent is the default percentage by which the effective-creation-timeout is grown on each
	// growth step, i.e. an increase of 50% corresponds to a factor of 1.5.
	DefaultCreationTimeoutGrowthPercent int32 = 50
	// DefaultMaxCreationTimeoutGrowthCount is the maximum number of times the effective-creation-timeout may be grown
	// by the growth factor before it is capped.
	DefaultMaxCreationTimeoutGrowthCount = 4
	// DefaultMachineReplaceCycleCountThreshold is the default value of the threshold for Machine replace cycles caused
	// by failures following which the effective-creation-timeout is grown.
	DefaultMachineReplaceCycleCountThreshold = 2
	// DefaultSuccessJoinCountThreshold is the minimum number of machines that must have joined within the current
	// effective-creation-timeout window before the timeout is shrunk to the average join duration of those machines.
	DefaultSuccessJoinCountThreshold = 2
)
