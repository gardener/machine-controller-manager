// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package features is reserved for future purposes
package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
)

const (
	// Every feature gate should add method here following this template:
	//
	// // owner: @username
	// // alpha: v1.X
	// MyFeature utilfeature.Feature = "MyFeature"

	// MachineTestFeature is a feature gate
	// owner: @i068969
	// beta: v1.4
	MachineTestFeature featuregate.Feature = "MachineTestFeature"
)

func init() {
	runtime.Must(utilfeature.DefaultFeatureGate.DeepCopy().Add(defaultKubernetesFeatureGates))
}

var defaultKubernetesFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	MachineTestFeature: {Default: true, PreRelease: featuregate.Beta},
}
