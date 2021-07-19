package helpers

import (
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

//ResourcesTrackerInterface provides an interface to check for orphan resources.
//The implementation should handle probing for resources while contructing or calling New method
//And reporting orphan resources whenever IsOrphanedResourcesAvailable is invoked
type ResourcesTrackerInterface interface {
	IsOrphanedResourcesAvailable() bool
	InitializeResourcesTracker(
		machineClass *v1alpha1.MachineClass,
		secretData map[string][]byte,
		clusterName string,
	) error
}
