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

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
)

// ValidateGCPMachineClass validates a GCPMachineClass and returns a list of errors.
func ValidateGCPMachineClass(GCPMachineClass *machine.GCPMachineClass) field.ErrorList {
	return internalValidateGCPMachineClass(GCPMachineClass)
}

func internalValidateGCPMachineClass(GCPMachineClass *machine.GCPMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateGCPMachineClassSpec(&GCPMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateGCPMachineClassSpec(spec *machine.GCPMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateGCPDisks(spec.Disks, fldPath.Child("disks"))...)

	if "" == spec.MachineType {
		allErrs = append(allErrs, field.Required(fldPath.Child("machineType"), "machineType is required"))
	}
	if "" == spec.Region {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "region is required"))
	}
	if "" == spec.Zone {
		allErrs = append(allErrs, field.Required(fldPath.Child("zone"), "zone is required"))
	}

	allErrs = append(allErrs, validateGCPNetworkInterfaces(spec.NetworkInterfaces, fldPath.Child("networkInterfaces"))...)
	allErrs = append(allErrs, validateGCPMetadata(spec.Metadata, fldPath.Child("networkInterfaces"))...)
	allErrs = append(allErrs, validateGCPScheduling(spec.Scheduling, fldPath.Child("scheduling"))...)
	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, fldPath.Child("secretRef"))...)
	allErrs = append(allErrs, validateGCPServiceAccounts(spec.ServiceAccounts, fldPath.Child("serviceAccounts"))...)
	allErrs = append(allErrs, validateGCPClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)

	return allErrs
}

func validateGCPClassSpecTags(tags []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	for _, key := range tags {
		if strings.Contains(key, "kubernetes-io-cluster-") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes-io-role-") {
			nodeRole = key
		}
	}

	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes-io-cluster-"), "Tag required of the form kubernetes-io-cluster-****"))
	}
	if nodeRole == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes-io-role-"), "Tag required of the form kubernetes-io-role-****"))
	}

	return allErrs
}

func validateGCPDisks(disks []*machine.GCPDisk, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if 0 == len(disks) {
		allErrs = append(allErrs, field.Required(fldPath, "at least one disk is required"))
	}

	for i, disk := range disks {
		idxPath := fldPath.Index(i)
		if disk.SizeGb < 20 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("sizeGb"), disk.SizeGb, "disk size must be at least 20 GB"))
		}
		if disk.Type != "pd-standard" && disk.Type != "pd-ssd" && disk.Type != "SCRATCH" {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("type"), disk.Type, []string{"pd-standard", "pd-ssd", "SCRATCH"}))
		}
		if disk.Type == "SCRATCH" && (disk.Interface != "NVME" && disk.Interface != "SCSI") {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("interface"), disk.Interface, []string{"NVME", "SCSI"}))
		}
		if disk.Boot && "" == disk.Image {
			allErrs = append(allErrs, field.Required(idxPath.Child("image"), "image is required for boot disk"))
		}
	}

	return allErrs
}

func validateGCPNetworkInterfaces(interfaces []*machine.GCPNetworkInterface, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if 0 == len(interfaces) {
		allErrs = append(allErrs, field.Required(fldPath.Child("networkInterfaces"), "at least one network interface is required"))
	}

	for i, nic := range interfaces {
		idxPath := fldPath.Index(i)
		if "" == nic.Network && "" == nic.Subnetwork {
			allErrs = append(allErrs, field.Required(idxPath, "either network or subnetwork or both is required"))
		}
	}

	return allErrs
}

func validateGCPMetadata(metadata []*machine.GCPMetadata, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, item := range metadata {
		idxPath := fldPath.Index(i)
		if item.Key == "user-data" {
			allErrs = append(allErrs, field.Forbidden(idxPath.Child("key"), "user-data key is forbidden in metadata"))
		}
	}

	return allErrs
}

func validateGCPScheduling(scheduling machine.GCPScheduling, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "MIGRATE" != scheduling.OnHostMaintenance && "TERMINATE" != scheduling.OnHostMaintenance {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("onHostMaintenance"), scheduling.OnHostMaintenance, []string{"MIGRATE", "TERMINATE"}))
	}

	return allErrs
}

func validateGCPServiceAccounts(serviceAccounts []machine.GCPServiceAccount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if 0 == len(serviceAccounts) {
		allErrs = append(allErrs, field.Required(fldPath, "at least one service account is required"))
	}

	for i, account := range serviceAccounts {
		idxPath := fldPath.Index(i)
		if match, _ := regexp.MatchString(`^[^@]+@(?:[a-zA-Z-0-9]+\.)+[a-zA-Z]{2,}$`, account.Email); !match {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("email"), account.Email, "email address is of invalid format"))
		}
	}

	return allErrs
}
