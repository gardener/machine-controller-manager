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
	// DefaultCreationTimeoutGrowthFactor is the default value for growth of the effective-creation-timeout.
	DefaultCreationTimeoutGrowthFactor = 2
	// DefaultCreationTimeoutMax is the max limit upto which the effective-creation-timeout can be adjusted
	DefaultCreationTimeoutMax = 90 * time.Minute
	// DefaultMachineReplaceCycleCountThreshold is the default value of the threshold for Machine replace cycles caused
	// by failures following which the effective-creation-timeout is grown.
	DefaultMachineReplaceCycleCountThreshold = 2
	// DefaultMaxJoinDurationLookback is the lookback window used to compute the maximum observed machine join
	// duration when reducing the effective-creation-timeout after machines join successfully.
	DefaultMaxJoinDurationLookback = 24 * time.Hour
)
