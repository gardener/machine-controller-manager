// Copyright 2023 SAP SE or an SAP affiliate company
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

// ResourcesTrackerInterface provides an interface to check for orphan resources.
// The implementation should handle probing for resources while contructing or calling New method
// And reporting orphan resources whenever IsOrphanedResourcesAvailable is invoked
type ResourcesTrackerInterface interface {
	IsOrphanedResourcesAvailable() bool
	InitializeResourcesTracker(
		machineClass *v1alpha1.MachineClass,
		secretData map[string][]byte,
		clusterName string,
	) error
}
