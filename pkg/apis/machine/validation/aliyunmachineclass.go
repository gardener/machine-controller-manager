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
	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateAliyunMachineClass(AliyunMachineClass *machine.AliyunMachineClass) field.ErrorList {
	return internalValidateAliyunMachineClass(AliyunMachineClass)
}

func internalValidateAliyunMachineClass(AliyunMachineClass *machine.AliyunMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&AliyunMachineClass.ObjectMeta, true,
		validateName,
		field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateAliyunMachineClassSpec(&AliyunMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateAliyunMachineClassSpec(spec *machine.AliyunMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.ImageId {
		allErrs = append(allErrs, field.Required(fldPath.Child("imageId"), "ImageId is required"))
	}
	if "" == spec.Region {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "Region is required"))
	}
	if "" == spec.InstanceType {
		allErrs = append(allErrs, field.Required(fldPath.Child("instanceType"), "InstanceType is required"))
	}
	if "" == spec.VSwitchId {
		allErrs = append(allErrs, field.Required(fldPath.Child("vSwitchId"), "VSwitchId is required"))
	}
	if "" == spec.KeyPairName {
		allErrs = append(allErrs, field.Required(fldPath.Child("keyPairName"), "KeyPairName is required"))
	}

	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)
	allErrs = append(allErrs, validateAliyunClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)

	return allErrs
}

func validateAliyunClassSpecTags(tags *machine.AliyunTags, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	if tags == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("tags"), "Tags required for Aliyun machines"))
	}

	clusterName = tags.Tag1Key
	nodeRole = tags.Tag2Key

	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("tags.Tag1Key"), "Tag1Key required of the form kubernetes.io/cluster/****"))
	}
	if nodeRole == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("tags.Tag2Key"), "Tag2Key required of the form kubernetes.io/role/****"))
	}

	return allErrs
}
