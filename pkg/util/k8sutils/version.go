// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
