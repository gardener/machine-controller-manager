/*
Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package k8sutils is used to provider helper consts and functions for k8s operations
package k8sutils

import (
	"github.com/Masterminds/semver"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	// ConstraintK8sGreaterEqual121 is a version constraint for versions >= 1.21.
	ConstraintK8sGreaterEqual121 *semver.Constraints
)

func init() {
	var err error

	ConstraintK8sGreaterEqual121, err = semver.NewConstraint(">= 1.21-0")
	utilruntime.Must(err)
}

// CheckVersionMeetsConstraint returns true if the <version> meets the <constraint>.
func CheckVersionMeetsConstraint(version, constraint string) (bool, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, err
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return false, err
	}

	return c.Check(v), nil
}
