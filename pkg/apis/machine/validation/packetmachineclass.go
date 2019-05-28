/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package validation is used to validate all the machine CRD objects
package validation

import (
	"regexp"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
)

const packetNameFmt string = `[-a-z0-9]+`
const packetNameMaxLength int = 63

var packetNameRegexp = regexp.MustCompile("^" + packetNameFmt + "$")

// packetValidateName is the validation function for object names.
func packetValidateName(value string, prefix bool) []string {
	var errs []string
	if len(value) > nameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(packetNameMaxLength))
	}
	if !packetNameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(packetNameFmt, "name-40d-0983-1b89"))
	}

	return errs
}

// ValidatePacketMachineClass validates a PacketMachineClass and returns a list of errors.
func ValidatePacketMachineClass(PacketMachineClass *machine.PacketMachineClass) field.ErrorList {
	return internalValidatePacketMachineClass(PacketMachineClass)
}

func internalValidatePacketMachineClass(PacketMachineClass *machine.PacketMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&PacketMachineClass.ObjectMeta, true, /*namespace*/
		packetValidateName,
		field.NewPath("metadata"))...)

	allErrs = append(allErrs, validatePacketMachineClassSpec(&PacketMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePacketMachineClassSpec(spec *machine.PacketMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.OS {
		allErrs = append(allErrs, field.Required(fldPath.Child("os"), "OS is required"))
	}
	if "" == spec.MachineType {
		allErrs = append(allErrs, field.Required(fldPath.Child("machineType"), "Machine Type is required"))
	}
	if "" == spec.ProjectID {
		allErrs = append(allErrs, field.Required(fldPath.Child("projectID"), "Project ID is required"))
	}
	if len(spec.Facility) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("facility"), "At least one Facility specification is required"))
	}

	allErrs = append(allErrs, validatePacketClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)
	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)

	return allErrs
}

func validatePacketClassSpecTags(tags []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	for _, key := range tags {
		if strings.Contains(key, "kubernetes.io/cluster/") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes.io/role/") {
			nodeRole = key
		}
	}

	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io/cluster/"), "Tag required of the form kubernetes.io/cluster/****"))
	}
	if nodeRole == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io/role/"), "Tag required of the form kubernetes.io/role/****"))
	}

	return allErrs
}
