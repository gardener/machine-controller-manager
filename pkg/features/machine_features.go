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
