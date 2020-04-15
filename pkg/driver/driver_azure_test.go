/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

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

package driver

import (
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Driver Azure", func() {

	Context("GenerateDataDisks Driver Azure Spec", func() {

		It("should convert multiple dataDisks successfully", func() {
			azureDriver := &AzureDriver{}
			vmName := "vm"
			lun1 := int32(1)
			lun2 := int32(2)
			size1 := int32(10)
			size2 := int32(100)
			expectedName1 := "vm-sdb-1-data-disk"
			expectedName2 := "vm-sdc-2-data-disk"
			disks := []v1alpha1.AzureDataDisk{
				{
					Name:               "sdb",
					Caching:            "None",
					StorageAccountType: "Premium_LRS",
					DiskSizeGB:         size1,
					Lun:                &lun1,
				},
				{
					Name:               "sdc",
					Caching:            "None",
					StorageAccountType: "Standard_LRS",
					DiskSizeGB:         size2,
					Lun:                &lun2,
				},
			}

			disksGenerated := azureDriver.generateDataDisks(vmName, disks)
			expectedDisks := []compute.DataDisk{
				{
					Lun:     &lun1,
					Name:    &expectedName1,
					Caching: compute.CachingTypes("None"),
					ManagedDisk: &compute.ManagedDiskParameters{
						StorageAccountType: compute.StorageAccountTypes("Premium_LRS"),
					},
					DiskSizeGB:   &size1,
					CreateOption: compute.DiskCreateOptionTypes("Empty"),
				},
				{
					Lun:     &lun2,
					Name:    &expectedName2,
					Caching: compute.CachingTypes("None"),
					ManagedDisk: &compute.ManagedDiskParameters{
						StorageAccountType: compute.StorageAccountTypes("Standard_LRS"),
					},
					DiskSizeGB:   &size2,
					CreateOption: compute.DiskCreateOptionTypes("Empty"),
				},
			}

			Expect(disksGenerated).To(Equal(expectedDisks))
		})

		It("should convert multiple dataDisks successfully with default caching and luns", func() {
			azureDriver := &AzureDriver{}
			vmName := "vm"
			lun1 := int32(0)
			lun2 := int32(1)
			lun3 := int32(42)
			size1 := int32(10)
			size2 := int32(100)
			expectedName1 := "vm-sdb-0-data-disk"
			expectedName2 := "vm-1-data-disk"
			expectedName3 := "vm-sdc-42-data-disk"
			disks := []v1alpha1.AzureDataDisk{
				{
					Name:               "sdb",
					StorageAccountType: "Premium_LRS",
					DiskSizeGB:         size1,
				},
				{
					StorageAccountType: "Standard_LRS",
					DiskSizeGB:         size2,
				},
				{
					Lun:                &lun3,
					Name:               "sdc",
					StorageAccountType: "Standard_LRS",
					DiskSizeGB:         size2,
				},
			}

			disksGenerated := azureDriver.generateDataDisks(vmName, disks)
			expectedDisks := []compute.DataDisk{
				{
					Lun:     &lun1,
					Name:    &expectedName1,
					Caching: compute.CachingTypes("None"),
					ManagedDisk: &compute.ManagedDiskParameters{
						StorageAccountType: compute.StorageAccountTypes("Premium_LRS"),
					},
					DiskSizeGB:   &size1,
					CreateOption: compute.DiskCreateOptionTypes("Empty"),
				},
				{
					Lun:     &lun2,
					Name:    &expectedName2,
					Caching: compute.CachingTypes("None"),
					ManagedDisk: &compute.ManagedDiskParameters{
						StorageAccountType: compute.StorageAccountTypes("Standard_LRS"),
					},
					DiskSizeGB:   &size2,
					CreateOption: compute.DiskCreateOptionTypes("Empty"),
				},
				{
					Lun:     &lun3,
					Name:    &expectedName3,
					Caching: compute.CachingTypes("None"),
					ManagedDisk: &compute.ManagedDiskParameters{
						StorageAccountType: compute.StorageAccountTypes("Standard_LRS"),
					},
					DiskSizeGB:   &size2,
					CreateOption: compute.DiskCreateOptionTypes("Empty"),
				},
			}

			Expect(disksGenerated).To(Equal(expectedDisks))
		})

	})
})
