// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cidr

import (
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CIDR contains CIDR and Path information
type CIDR interface {
	// GetCIDR returns the provided CIDR
	GetCIDR() string
	// GetFieldPath returns the fieldpath
	GetFieldPath() *field.Path
	// GetIPNet optionally returns the IPNet of the CIDR
	GetIPNet() *net.IPNet
	// Parse checks if CIDR parses
	Parse() bool
	// ValidateNotSubset returns errors if subsets is a subset.
	ValidateNotSubset(subsets ...CIDR) field.ErrorList
	// ValidateParse returns errors CIDR can't be parsed.
	ValidateParse() field.ErrorList
	// ValidateSubset returns errors if subsets is not a subset.
	ValidateSubset(subsets ...CIDR) field.ErrorList
}

type cidrPath struct {
	cidr       string
	fieldPath  *field.Path
	net        *net.IPNet
	ParseError error
}

// NewCIDR creates a new instance of cidrPath
func NewCIDR(c string, f *field.Path) CIDR {
	_, ipNet, err := net.ParseCIDR(string(c))
	return &cidrPath{c, f, ipNet, err}
}

func (c *cidrPath) ValidateSubset(subsets ...CIDR) field.ErrorList {
	allErrs := field.ErrorList{}
	if c.ParseError != nil {
		return allErrs
	}
	for _, subset := range subsets {
		if subset == nil || c == subset || !subset.Parse() {
			continue
		}
		if !c.net.Contains(subset.GetIPNet().IP) {
			allErrs = append(allErrs, field.Invalid(subset.GetFieldPath(), subset.GetCIDR(), fmt.Sprintf("must be a subset of %q (%q)", c.fieldPath.String(), c.cidr)))
		}
	}
	return allErrs
}

func (c *cidrPath) ValidateNotSubset(subsets ...CIDR) field.ErrorList {
	allErrs := field.ErrorList{}
	if c.ParseError != nil {
		return allErrs
	}
	for _, subset := range subsets {
		if subset == nil || c == subset || !subset.Parse() {
			continue
		}
		if c.net.Contains(subset.GetIPNet().IP) {
			allErrs = append(allErrs, field.Invalid(subset.GetFieldPath(), subset.GetCIDR(), fmt.Sprintf("must not be a subset of %q (%q)", c.fieldPath.String(), c.cidr)))
		}
	}
	return allErrs
}

func (c *cidrPath) ValidateParse() field.ErrorList {
	allErrs := field.ErrorList{}

	if c.ParseError != nil {
		allErrs = append(allErrs, field.Invalid(c.fieldPath, c.cidr, c.ParseError.Error()))
	}

	return allErrs
}

func (c *cidrPath) Parse() (success bool) {
	return c.ParseError == nil
}

func (c *cidrPath) GetIPNet() *net.IPNet {
	return c.net
}

func (c *cidrPath) GetFieldPath() *field.Path {
	return c.fieldPath
}

func (c *cidrPath) GetCIDR() string {
	return c.cidr
}
