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
	/*"strconv"
	"strings"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	*/
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
)

// ValidateAzureMachineClass validates a AzureMachineClass and returns a list of errors.
func ValidateAzureMachineClass(AzureMachineClass *machine.AzureMachineClass) field.ErrorList {
	return internalValidateAzureMachineClass(AzureMachineClass)
}

func internalValidateAzureMachineClass(AzureMachineClass *machine.AzureMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateAzureMachineClassSpec(&AzureMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateAzureMachineClassSpec(spec *machine.AzureMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.Location {
		allErrs = append(allErrs, field.Required(fldPath.Child("location"), "Location is required"))
	}
	if "" == spec.ResourceGroup {
		allErrs = append(allErrs, field.Required(fldPath.Child("resourceGroup"), "ResourceGroup is required"))
	}
	if "" == spec.SubnetInfo.SubnetName {
		allErrs = append(allErrs, field.Required(fldPath.Child("subnetInfo.subnetName"), "SubnetName is required"))
	}
	if "" == spec.SubnetInfo.VnetName {
		allErrs = append(allErrs, field.Required(fldPath.Child("subnetInfo.vnetName"), "VNetName Name is required"))
	}

	allErrs = append(allErrs, validateAzureProperties(spec.Properties, field.NewPath("spec.properties"))...)
	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)
	allErrs = append(allErrs, validateAzureClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)

	return allErrs
}

func validateAzureClassSpecTags(tags map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	for key := range tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			nodeRole = key
		}
	}

	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io-cluster-"), "Tag required of the form kubernetes.io-cluster-****"))
	}
	if nodeRole == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io-role-"), "Tag required of the form kubernetes.io-role-****"))
	}

	return allErrs
}

func validateAzureProperties(properties machine.AzureVirtualMachineProperties, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if properties.HardwareProfile.VMSize == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("hardwareProfile.vmSize"), "VMSize is required"))
	}

	if properties.StorageProfile.ImageReference.URN == nil || *properties.StorageProfile.ImageReference.URN == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("storageProfile.imageReference.urn"), "Empty urn"))
	} else {
		splits := strings.Split(*properties.StorageProfile.ImageReference.URN, ":")
		if len(splits) != 4 {
			allErrs = append(allErrs, field.Required(fldPath.Child("storageProfile.imageReference.urn"), "Invalid urn format"))
		} else {
			for _, s := range splits {
				if len(s) == 0 {
					allErrs = append(allErrs, field.Required(fldPath.Child("storageProfile.imageReference.urn"), "Invalid urn format, empty field"))
				}
			}
		}
	}

	if properties.StorageProfile.OsDisk.Caching == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("storageProfile.osDisk.caching"), "OSDisk caching is required"))
	}
	if properties.StorageProfile.OsDisk.DiskSizeGB <= 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("storageProfile.osDisk.diskSizeGB"), "OSDisk size must be positive"))
	}
	if properties.StorageProfile.OsDisk.CreateOption == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("storageProfile.osDisk.createOption"), "OSDisk create option is required"))
	}

	if properties.OsProfile.AdminUsername == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("osProfile.adminUsername"), "AdminUsername is required"))
	}

	if properties.Zone == nil && properties.AvailabilitySet == nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("zone|.availabilitySet"), "Machine need to be assigned to a zone or an AvailabilitySet"))
	}
	if properties.Zone != nil && properties.AvailabilitySet != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("zone|.availabilitySet"), "Machine cannot be assigned to a zone and an AvailabilitySet in parallel"))
	}

	/*
		if properties.OsProfile.LinuxConfiguration.SSH.PublicKeys.Path == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("osProfile.linuxConfiguration.ssh.publicKeys.path"), "PublicKey path is required"))
		}
		if properties.OsProfile.LinuxConfiguration.SSH.PublicKeys.KeyData == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("osProfile.linuxConfiguration.ssh.publicKeys.keyData"), "PublicKey data is required"))
		}*/

	return allErrs
}
