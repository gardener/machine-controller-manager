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
package validation

import (
	"strconv"
	"strings"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node"
)

const nameFmt string = `[-a-z0-9]+`
const nameMaxLength int = 63

var nameRegexp = regexp.MustCompile("^" + nameFmt + "$")

// validateName is the validation function for ServicePlan names.
func validateName(value string, prefix bool) []string {
	var errs []string
	if len(value) > nameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(nameMaxLength))
	}
	if !nameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(nameFmt, "name-40d-0983-1b89"))
	}

	return errs
}

// ValidateAWSInstanceClass validates a AWSInstanceClass and returns a list of errors.
func ValidateAWSInstanceClass(AWSInstanceClass *node.AWSInstanceClass) field.ErrorList {
	return internalValidateAWSInstanceClass(AWSInstanceClass)
}

func internalValidateAWSInstanceClass(AWSInstanceClass *node.AWSInstanceClass) field.ErrorList {
	allErrs := field.ErrorList{}
	
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&AWSInstanceClass.ObjectMeta, false, /*namespace*/
		validateName,
		field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateAWSInstanceClassSpec(&AWSInstanceClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateAWSInstanceClassSpec(spec *node.AWSInstanceClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.AMI {
		allErrs = append(allErrs, field.Required(fldPath.Child("ami"), "AMI is required"))
	}
	if "" == spec.AvailabilityZone {
		allErrs = append(allErrs, field.Required(fldPath.Child("availabilityZone"), "AvailabilityZone is required"))
	}
	if "" == spec.InstanceType {
		allErrs = append(allErrs, field.Required(fldPath.Child("instanceType"), "InstanceType is required"))
	}
	if "" == spec.IAM.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("iam.name"), "IAM Name is required"))
	}
	if "" == spec.KeyName {
		allErrs = append(allErrs, field.Required(fldPath.Child("keyName"), "KeyName is required"))
	}
	
	allErrs = append(allErrs, validateBlockDevices(spec.BlockDevices, field.NewPath("spec.blockDevices"))...)
	allErrs = append(allErrs, validateNetworkInterfaces(spec.NetworkInterfaces, field.NewPath("spec.networkInterfaces"))...)
	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)

	return allErrs
}

func validateBlockDevices(blockDevices []node.AWSBlockDeviceMappingSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(blockDevices) > 1 {
		allErrs = append(allErrs, field.Required(fldPath.Child(""), "Can only specify one (root) block device"))
	} else if len(blockDevices) == 1 {
		if blockDevices[0].Ebs.VolumeSize == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("ebs.volumeSize"), "Please mention a valid ebs volume size"))
		}
		if blockDevices[0].Ebs.VolumeType == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("ebs.volumeType"), "Please mention a valid ebs volume type"))
		}
	}
	return allErrs
}

func validateNetworkInterfaces(networkInterfaces []node.AWSNetworkInterfaceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(networkInterfaces) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child(""), "Mention atleast one NetworkInterface"))
	} else {
		for i, _ := range networkInterfaces {
			if "" == networkInterfaces[i].SubnetID {
				allErrs = append(allErrs, field.Required(fldPath.Child("subnetID"), "SubnetID is required"))
			}
			
			if 0 == len(networkInterfaces[i].SecurityGroupID) {
				allErrs = append(allErrs, field.Required(fldPath.Child("securityGroupID"), "Mention atleast one securityGroupID"))
			} else {
				for j, _ := range networkInterfaces[i].SecurityGroupID {
					if "" == networkInterfaces[i].SecurityGroupID[j] {
						output := strings.Join([]string{"securityGroupID cannot be blank for networkInterface:", strconv.Itoa(i) ," securityGroupID:", strconv.Itoa(j)}, "")
						allErrs = append(allErrs, field.Required(fldPath.Child("securityGroupID"), output))	
					}
				}
			}
		}
	}
	return allErrs
}

func validateSecretRef(reference *corev1.SecretReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if "" == reference.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	}

	if "" == reference.Namespace {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "namespace is required"))
	}
	return allErrs
}


