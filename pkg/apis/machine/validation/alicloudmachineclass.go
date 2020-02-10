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
	"fmt"
	"strings"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateAlicloudMachineClass is to validate Alicoud machine class
func ValidateAlicloudMachineClass(AlicloudMachineClass *machine.AlicloudMachineClass) field.ErrorList {
	return internalValidateAlicloudMachineClass(AlicloudMachineClass)
}

func internalValidateAlicloudMachineClass(AlicloudMachineClass *machine.AlicloudMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&AlicloudMachineClass.ObjectMeta, true,
		validateName,
		field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateAlicloudMachineClassSpec(&AlicloudMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateAlicloudMachineClassSpec(spec *machine.AlicloudMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.ImageID {
		allErrs = append(allErrs, field.Required(fldPath.Child("imageID"), "ImageID is required"))
	}
	if "" == spec.Region {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "Region is required"))
	}
	if "" == spec.ZoneID {
		allErrs = append(allErrs, field.Required(fldPath.Child("zoneID"), "ZoneID is required"))
	}
	if "" == spec.InstanceType {
		allErrs = append(allErrs, field.Required(fldPath.Child("instanceType"), "InstanceType is required"))
	}
	if "" == spec.VSwitchID {
		allErrs = append(allErrs, field.Required(fldPath.Child("vSwitchID"), "VSwitchID is required"))
	}
	if "" == spec.KeyPairName {
		allErrs = append(allErrs, field.Required(fldPath.Child("keyPairName"), "KeyPairName is required"))
	}

	if spec.DataDisks != nil {
		names := map[string]int{}
		for i, dataDisk := range spec.DataDisks {
			idxPath := fldPath.Child("dataDisks").Index(i)
			if _, keyExist := names[dataDisk.Name]; keyExist {
				names[dataDisk.Name]++
			} else {
				names[dataDisk.Name] = 1
			}

			if dataDisk.Name == "" {
				allErrs = append(allErrs, field.Required(idxPath.Child("name"), "Data Disk name is required"))
			}
			if dataDisk.Size <= 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("size"), "Data Disk size must be positive"))
			}
			if dataDisk.Category == "" {
				allErrs = append(allErrs, field.Required(idxPath.Child("category"), "Data Disk category is required"))
			}
		}

		for name, number := range names {
			if number > 1 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("dataDisks"), name, fmt.Sprintf("Data Disk Name '%s' duplicated %d times, Name must be unique", name, number)))
			}
		}
	}

	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)
	allErrs = append(allErrs, validateAlicloudClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)

	return allErrs
}

func validateAlicloudClassSpecTags(tags map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	if tags == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("tags"), "Tags required for Alicloud machines"))
	}

	for key := range tags {
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
