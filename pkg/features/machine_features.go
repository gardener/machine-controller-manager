/*
Copyright 2017 The Gardener Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package features is reserved for future purposes
package features

import (
	utilfeature "k8s.io/apiserver/pkg/util/feature"
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
	MachineTestFeature utilfeature.Feature = "MachineTestFeature"
)

func init() {
	utilfeature.DefaultFeatureGate.Add(defaultKubernetesFeatureGates)
}

var defaultKubernetesFeatureGates = map[utilfeature.Feature]utilfeature.FeatureSpec{
	MachineTestFeature: {Default: true, PreRelease: utilfeature.Beta},
}
