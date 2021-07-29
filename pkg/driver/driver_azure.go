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

// Package driver contains the cloud provider specific implementations to manage machines
package driver

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	backoff "github.com/gardener/machine-controller-manager/pkg/util/backoff"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/marketplaceordering/mgmt/2015-06-01/marketplaceordering"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-11-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog/v2"
)

const (
	// azureDiskDriverName is the name of the CSI driver for Azure Disk
	azureDiskDriverName = "disk.csi.azure.com"
)

// AzureDriver is the driver struct for holding azure machine information
type AzureDriver struct {
	AzureMachineClass *v1alpha1.AzureMachineClass
	CredentialsData   map[string][]byte
	UserData          string
	MachineID         string
	MachineName       string
}

func (d *AzureDriver) getNICParameters(vmName string, subnet *network.Subnet) network.Interface {

	var (
		nicName            = dependencyNameFromVMName(vmName, nicSuffix)
		location           = d.AzureMachineClass.Spec.Location
		enableIPForwarding = true
	)

	// Add tags to the machine resources
	tagList := map[string]*string{}
	for idx, element := range d.AzureMachineClass.Spec.Tags {
		tagList[idx] = to.StringPtr(element)
	}

	NICParameters := network.Interface{
		Name:     &nicName,
		Location: &location,
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			IPConfigurations: &[]network.InterfaceIPConfiguration{
				{
					Name: &nicName,
					InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: network.Dynamic,
						Subnet:                    subnet,
					},
				},
			},
			EnableIPForwarding:          &enableIPForwarding,
			EnableAcceleratedNetworking: d.AzureMachineClass.Spec.Properties.NetworkProfile.AcceleratedNetworking,
		},
		Tags: tagList,
	}

	return NICParameters
}

func (d *AzureDriver) getVMParameters(vmName string, image *compute.VirtualMachineImage, networkInterfaceReferenceID string) compute.VirtualMachine {
	var (
		diskName    = dependencyNameFromVMName(vmName, diskSuffix)
		UserDataEnc = base64.StdEncoding.EncodeToString([]byte(d.UserData))
		location    = d.AzureMachineClass.Spec.Location
	)

	// Add tags to the machine resources
	tagList := map[string]*string{}
	for idx, element := range d.AzureMachineClass.Spec.Tags {
		tagList[idx] = to.StringPtr(element)
	}

	imageReference := getImageReference(d)

	var plan *compute.Plan
	if image != nil && image.Plan != nil {
		// If image.Plan exists, create a plan object and attach it to the VM
		klog.V(2).Infof("Creating a plan object and attaching it to the VM - %q", vmName)
		plan = &compute.Plan{
			Name:      image.VirtualMachineImageProperties.Plan.Name,
			Product:   image.VirtualMachineImageProperties.Plan.Product,
			Publisher: image.VirtualMachineImageProperties.Plan.Publisher,
		}
	}

	VMParameters := compute.VirtualMachine{
		Name:     &vmName,
		Plan:     plan,
		Location: &location,
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			HardwareProfile: &compute.HardwareProfile{
				VMSize: compute.VirtualMachineSizeTypes(d.AzureMachineClass.Spec.Properties.HardwareProfile.VMSize),
			},
			StorageProfile: &compute.StorageProfile{
				ImageReference: &imageReference,
				OsDisk: &compute.OSDisk{
					Name:    &diskName,
					Caching: compute.CachingTypes(d.AzureMachineClass.Spec.Properties.StorageProfile.OsDisk.Caching),
					ManagedDisk: &compute.ManagedDiskParameters{
						StorageAccountType: compute.StorageAccountTypes(d.AzureMachineClass.Spec.Properties.StorageProfile.OsDisk.ManagedDisk.StorageAccountType),
					},
					DiskSizeGB:   &d.AzureMachineClass.Spec.Properties.StorageProfile.OsDisk.DiskSizeGB,
					CreateOption: compute.DiskCreateOptionTypes(d.AzureMachineClass.Spec.Properties.StorageProfile.OsDisk.CreateOption),
				},
			},
			OsProfile: &compute.OSProfile{
				ComputerName:  &vmName,
				AdminUsername: &d.AzureMachineClass.Spec.Properties.OsProfile.AdminUsername,
				CustomData:    &UserDataEnc,
				LinuxConfiguration: &compute.LinuxConfiguration{
					DisablePasswordAuthentication: &d.AzureMachineClass.Spec.Properties.OsProfile.LinuxConfiguration.DisablePasswordAuthentication,
					SSH: &compute.SSHConfiguration{
						PublicKeys: &[]compute.SSHPublicKey{
							{
								Path:    &d.AzureMachineClass.Spec.Properties.OsProfile.LinuxConfiguration.SSH.PublicKeys.Path,
								KeyData: &d.AzureMachineClass.Spec.Properties.OsProfile.LinuxConfiguration.SSH.PublicKeys.KeyData,
							},
						},
					},
				},
			},
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: &networkInterfaceReferenceID,
						NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
							Primary: to.BoolPtr(true),
						},
					},
				},
			},
		},
		Tags: tagList,
	}

	if d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks != nil && len(d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks) > 0 {
		dataDisks := d.generateDataDisks(vmName, d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks)
		VMParameters.StorageProfile.DataDisks = &dataDisks
	}

	if d.AzureMachineClass.Spec.Properties.Zone != nil {
		VMParameters.Zones = &[]string{strconv.Itoa(*d.AzureMachineClass.Spec.Properties.Zone)}
	}

	// DEPRECATED: This will be removed in future in favour of the machineSet field which has a type for AvailabilitySet.
	if d.AzureMachineClass.Spec.Properties.AvailabilitySet != nil {
		VMParameters.VirtualMachineProperties.AvailabilitySet = &compute.SubResource{
			ID: &d.AzureMachineClass.Spec.Properties.AvailabilitySet.ID,
		}
	}

	if d.AzureMachineClass.Spec.Properties.MachineSet != nil {
		switch d.AzureMachineClass.Spec.Properties.MachineSet.Kind {
		case machine.MachineSetKindVMO:
			VMParameters.VirtualMachineProperties.VirtualMachineScaleSet = &compute.SubResource{
				ID: &d.AzureMachineClass.Spec.Properties.MachineSet.ID,
			}
		case machine.MachineSetKindAvailabilitySet:
			VMParameters.VirtualMachineProperties.AvailabilitySet = &compute.SubResource{
				ID: &d.AzureMachineClass.Spec.Properties.MachineSet.ID,
			}
		}
	}

	if d.AzureMachineClass.Spec.Properties.IdentityID != nil && *d.AzureMachineClass.Spec.Properties.IdentityID != "" {
		VMParameters.Identity = &compute.VirtualMachineIdentity{
			Type: compute.ResourceIdentityTypeUserAssigned,
			UserAssignedIdentities: map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue{
				*d.AzureMachineClass.Spec.Properties.IdentityID: {},
			},
		}
	}

	return VMParameters
}

func getImageReference(d *AzureDriver) compute.ImageReference {
	imageRefClass := d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference
	if imageRefClass.ID != "" {
		return compute.ImageReference{
			ID: &imageRefClass.ID,
		}
	}

	splits := strings.Split(*imageRefClass.URN, ":")
	publisher := splits[0]
	offer := splits[1]
	sku := splits[2]
	version := splits[3]
	return compute.ImageReference{
		Publisher: &publisher,
		Offer:     &offer,
		Sku:       &sku,
		Version:   &version,
	}
}

func (d *AzureDriver) generateDataDisks(vmName string, azureDataDisks []v1alpha1.AzureDataDisk) []compute.DataDisk {
	var dataDisks []compute.DataDisk
	for i, azureDataDisk := range azureDataDisks {

		var dataDiskLun *int32
		if azureDataDisk.Lun != nil {
			dataDiskLun = azureDataDisk.Lun
		} else {
			lun := int32(i)
			dataDiskLun = &lun
		}

		dataDiskName := dependencyNameFromVMNameAndDependency(getAzureDataDiskPrefix(azureDataDisk.Name, dataDiskLun), vmName, dataDiskSuffix)

		var caching compute.CachingTypes
		if azureDataDisk.Caching != "" {
			caching = compute.CachingTypes(azureDataDisk.Caching)
		} else {
			caching = compute.CachingTypesNone
		}

		dataDiskSize := azureDataDisk.DiskSizeGB

		dataDisk := compute.DataDisk{
			Lun:     dataDiskLun,
			Name:    &dataDiskName,
			Caching: caching,
			ManagedDisk: &compute.ManagedDiskParameters{
				StorageAccountType: compute.StorageAccountTypes(azureDataDisk.StorageAccountType),
			},
			DiskSizeGB:   &dataDiskSize,
			CreateOption: compute.DiskCreateOptionTypesEmpty,
		}
		dataDisks = append(dataDisks, dataDisk)
	}
	return dataDisks
}

// Create method is used to create an azure machine
func (d *AzureDriver) Create() (string, string, error) {
	var (
		vmName   = strings.ToLower(d.MachineName)
		location = d.AzureMachineClass.Spec.Location
	)

	_, err := d.createVMNicDisk()
	if err != nil {
		return "Error", "Error", err
	}

	return encodeMachineID(location, vmName), vmName, nil
}

// Delete method is used to delete an azure machine
func (d *AzureDriver) Delete(machineID string) error {
	clients, err := d.setup()
	if err != nil {
		return err
	}

	var (
		ctx               = context.Background()
		vmName            = decodeMachineID(machineID)
		nicName           = dependencyNameFromVMName(vmName, nicSuffix)
		diskName          = dependencyNameFromVMName(vmName, diskSuffix)
		resourceGroupName = d.AzureMachineClass.Spec.ResourceGroup
	)

	// Check if the underlying resource group still exists, if not skip the deletion as all resources are gone.
	if _, err := clients.group.Get(ctx, resourceGroupName); err != nil {
		if notFound(err) {
			return nil
		}
		return err
	}

	var dataDiskNames []string
	if d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks != nil && len(d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks) > 0 {
		dataDiskNames = getAzureDataDiskNames(d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks, vmName, dataDiskSuffix)
	}

	if err := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames); err != nil {
		return err
	}

	orphanNicChecker := func() error {
		return retry(func() error {
			return clients.checkOrphanNics(ctx, resourceGroupName, vmName)
		}, 3, time.Second*30)
	}
	orphanDiskChecker := func() error {
		return retry(func() error {
			return clients.checkOrphanDisks(ctx, resourceGroupName, vmName)
		}, 3, time.Second*30)
	}
	orphanCheckers := []func() error{orphanNicChecker, orphanDiskChecker}
	return runInParallel(orphanCheckers)
}

// GetExisting method is used to fetch the machineID for an azure machine
func (d *AzureDriver) GetExisting() (string, error) {
	return d.MachineID, nil
}

// GetVMs returns a list of VM or VM resources
// Azure earlier treated all VM resources (instance, NIC, Disks)
// as a single resource and atomically created/deleted them in the driver interface.
// This caused issues when the controller crashes, during deletions. To fix this,
// now GetVMs interface checks for all resources instead of just VMs.
func (d *AzureDriver) GetVMs(machineID string) (result VMs, err error) {
	result = make(VMs)

	mergeIntoResult := func(source VMs) {
		for k, v := range source {
			result[k] = v
		}
	}

	clients, err := d.setup()
	if err != nil {
		return
	}

	var (
		ctx               = context.Background()
		resourceGroupName = d.AzureMachineClass.Spec.ResourceGroup
		location          = d.AzureMachineClass.Spec.Location
		tags              = d.AzureMachineClass.Spec.Tags
	)

	listOfVMs, err := clients.getRelevantVMs(ctx, machineID, resourceGroupName, location, tags)
	if err != nil {
		return
	}
	mergeIntoResult(listOfVMs)

	listOfVMsByNIC, err := clients.getRelevantNICs(ctx, machineID, resourceGroupName, location, tags)
	if err != nil {
		return
	}
	mergeIntoResult(listOfVMsByNIC)

	listOfVMsByDisk, err := clients.getRelevantDisks(ctx, machineID, resourceGroupName, location, tags)
	if err != nil {
		return
	}
	mergeIntoResult(listOfVMsByDisk)

	return
}

//GetUserData return the user data with which the VM will be booted
func (d *AzureDriver) GetUserData() string {
	return d.UserData
}

//SetUserData set the used data whit which the VM will be booted
func (d *AzureDriver) SetUserData(userData string) {
	d.UserData = userData
}

func (d *AzureDriver) setup() (*azureDriverClients, error) {
	var (
		subscriptionID = ExtractCredentialsFromData(d.CredentialsData, v1alpha1.AzureSubscriptionID, v1alpha1.AzureAlternativeSubscriptionID)
		tenantID       = ExtractCredentialsFromData(d.CredentialsData, v1alpha1.AzureTenantID, v1alpha1.AzureAlternativeTenantID)
		clientID       = ExtractCredentialsFromData(d.CredentialsData, v1alpha1.AzureClientID, v1alpha1.AzureAlternativeClientID)
		clientSecret   = ExtractCredentialsFromData(d.CredentialsData, v1alpha1.AzureClientSecret, v1alpha1.AzureAlternativeClientSecret)
		env            = azure.PublicCloud
	)
	return newClients(subscriptionID, tenantID, clientID, clientSecret, env)
}

type azureDriverClients struct {
	subnet      network.SubnetsClient
	nic         network.InterfacesClient
	vm          compute.VirtualMachinesClient
	disk        compute.DisksClient
	deployments resources.DeploymentsClient
	group       resources.GroupsClient
	images      compute.VirtualMachineImagesClient
	marketplace marketplaceordering.MarketplaceAgreementsClient
}

type azureTags map[string]string

func newClients(subscriptionID, tenantID, clientID, clientSecret string, env azure.Environment) (*azureDriverClients, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, err
	}

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, clientID, clientSecret, env.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	authorizer := autorest.NewBearerAuthorizer(spToken)

	subnetClient := network.NewSubnetsClient(subscriptionID)
	subnetClient.Authorizer = authorizer

	interfacesClient := network.NewInterfacesClient(subscriptionID)
	interfacesClient.Authorizer = authorizer

	vmClient := compute.NewVirtualMachinesClient(subscriptionID)
	vmClient.Authorizer = authorizer

	vmImagesClient := compute.NewVirtualMachineImagesClient(subscriptionID)
	vmImagesClient.Authorizer = authorizer

	diskClient := compute.NewDisksClient(subscriptionID)
	diskClient.Authorizer = authorizer

	groupClient := resources.NewGroupsClient(subscriptionID)
	groupClient.Authorizer = authorizer

	deploymentsClient := resources.NewDeploymentsClient(subscriptionID)
	deploymentsClient.Authorizer = authorizer

	marketplaceClient := marketplaceordering.NewMarketplaceAgreementsClient(subscriptionID)
	marketplaceClient.Authorizer = authorizer

	return &azureDriverClients{subnet: subnetClient, nic: interfacesClient, vm: vmClient, disk: diskClient, deployments: deploymentsClient, images: vmImagesClient, group: groupClient, marketplace: marketplaceClient}, nil
}

func (d *AzureDriver) createVMNicDisk() (*compute.VirtualMachine, error) {

	var (
		ctx               = context.Background()
		vmName            = strings.ToLower(d.MachineName)
		resourceGroupName = d.AzureMachineClass.Spec.ResourceGroup
		vnetName          = d.AzureMachineClass.Spec.SubnetInfo.VnetName
		vnetResourceGroup = resourceGroupName
		subnetName        = d.AzureMachineClass.Spec.SubnetInfo.SubnetName
		nicName           = dependencyNameFromVMName(vmName, nicSuffix)
		diskName          = dependencyNameFromVMName(vmName, diskSuffix)
		vmImageRef        *compute.VirtualMachineImage
	)

	clients, err := d.setup()
	if err != nil {
		return nil, err
	}

	// Check if the machine should assigned to a vnet in a different resource group.
	if d.AzureMachineClass.Spec.SubnetInfo.VnetResourceGroup != nil {
		vnetResourceGroup = *d.AzureMachineClass.Spec.SubnetInfo.VnetResourceGroup
	}

	var dataDiskNames []string
	if d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks != nil && len(d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks) > 0 {
		dataDiskNames = getAzureDataDiskNames(d.AzureMachineClass.Spec.Properties.StorageProfile.DataDisks, vmName, dataDiskSuffix)
	}

	/*
		Subnet fetching
	*/
	klog.V(3).Infof("Fetching subnet details for VM %q", vmName)
	// Getting the subnet object for subnetName
	subnet, err := clients.subnet.Get(
		ctx,
		vnetResourceGroup,
		vnetName,
		subnetName,
		"",
	)
	if err != nil {
		return nil, onARMAPIErrorFail(prometheusServiceSubnet, err, "Subnet.Get failed for %s due to %s", subnetName, err)
	}
	onARMAPISuccess(prometheusServiceSubnet, "subnet.Get")

	NIC, err := clients.nic.Get(ctx, resourceGroupName, nicName, "")
	if err != nil {
		if isResourceNotFoundError(err) {
			/*
				NIC creation
				Fetching NIC with matching name failed, hence create a new one.
			*/

			// Creating NICParameters for new NIC creation request
			NICParameters := d.getNICParameters(vmName, &subnet)

			// NIC creation request
			klog.V(3).Infof("NIC creation started for %q", nicName)
			NICFuture, err := clients.nic.CreateOrUpdate(ctx, resourceGroupName, *NICParameters.Name, NICParameters)
			if err != nil {
				// Since machine creation failed, delete any infra resources created
				deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
				if deleteErr != nil {
					klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
				}

				return nil, onARMAPIErrorFail(prometheusServiceNIC, err, "NIC.CreateOrUpdate failed for %s", *NICParameters.Name)
			}

			// Wait until NIC is created
			err = NICFuture.WaitForCompletionRef(ctx, clients.nic.Client)
			if err != nil {
				// Since machine creation failed, delete any infra resources created
				deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
				if deleteErr != nil {
					klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
				}

				return nil, onARMAPIErrorFail(prometheusServiceNIC, err, "NIC.WaitForCompletionRef failed for %s", *NICParameters.Name)
			}
			onARMAPISuccess(prometheusServiceNIC, "NIC.CreateOrUpdate")

			// Fetch NIC details
			NIC, err = NICFuture.Result(clients.nic)
			if err != nil {
				// Since machine creation failed, delete any infra resources created
				deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
				if deleteErr != nil {
					klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
				}

				return nil, err
			}
			klog.V(3).Infof("NIC creation was successful for %q", nicName)
		} else {
			// Get on NIC returns a non 404 error. Exiting creation with the error.

			// Since machine creation failed, delete any infra resources created
			deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
			if deleteErr != nil {
				klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
			}

			return nil, onARMAPIErrorFail(prometheusServiceNIC, err, "NIC.Get failed for %s", nicName)
		}
	} else {
		klog.V(3).Infof("Found existing NIC with matching name, hence adopting NIC with name %q", nicName)
	}

	/*
		VM creation
	*/
	imageRefClass := d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference
	// if ID is not set the image is referenced using a URN
	if imageRefClass.ID == "" {

		imageReference := getImageReference(d)
		vmImage, err := clients.images.Get(
			ctx,
			d.AzureMachineClass.Spec.Location,
			*imageReference.Publisher,
			*imageReference.Offer,
			*imageReference.Sku,
			*imageReference.Version)

		if err != nil {
			//Since machine creation failed, delete any infra resources created
			deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
			if deleteErr != nil {
				klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
			}

			return nil, onARMAPIErrorFail(prometheusServiceVM, err, "VirtualMachineImagesClient.Get failed for %s", d.AzureMachineClass.Name)
		}

		if vmImage.Plan != nil {
			// If VMImage.Plan exists, check if agreement is accepted and if not accept it for the subscription

			agreement, err := clients.marketplace.Get(
				ctx,
				*vmImage.Plan.Publisher,
				*vmImage.Plan.Product,
				*vmImage.Plan.Name,
			)

			if err != nil {
				//Since machine creation failed, delete any infra resources created
				deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
				if deleteErr != nil {
					klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
				}

				return nil, onARMAPIErrorFail(prometheusServiceVM, err, "MarketplaceAgreementsClient.Get failed for %s", d.AzureMachineClass.Name)
			}

			if agreement.Accepted == nil || *agreement.Accepted == false {
				// Need to accept the terms at least once for the subscription
				klog.V(2).Info("Accepting terms for subscription to make use of the plan")

				agreement.Accepted = to.BoolPtr(true)
				_, err = clients.marketplace.Create(
					ctx,
					*vmImage.Plan.Publisher,
					*vmImage.Plan.Product,
					*vmImage.Plan.Name,
					agreement,
				)

				if err != nil {
					//Since machine creation failed, delete any infra resources created
					deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
					if deleteErr != nil {
						klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
					}

					return nil, onARMAPIErrorFail(prometheusServiceVM, err, "MarketplaceAgreementsClient.Create failed for %s", d.AzureMachineClass.Name)
				}
			}
		}

		vmImageRef = &vmImage
	}

	// Creating VMParameters for new VM creation request
	VMParameters := d.getVMParameters(vmName, vmImageRef, *NIC.ID)

	// VM creation request
	klog.V(3).Infof("VM creation began for %q", vmName)
	VMFuture, err := clients.vm.CreateOrUpdate(ctx, resourceGroupName, *VMParameters.Name, VMParameters)
	if err != nil {
		//Since machine creation failed, delete any infra resources created
		deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
		if deleteErr != nil {
			klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
		}

		return nil, onARMAPIErrorFail(prometheusServiceVM, err, "VM.CreateOrUpdate failed for %s", *VMParameters.Name)
	}

	// Wait until VM is created
	klog.V(3).Infof("Waiting for VM create call completion for %q", vmName)
	err = VMFuture.WaitForCompletionRef(ctx, clients.vm.Client)
	if err != nil {
		// Since machine creation failed, delete any infra resources created
		deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
		if deleteErr != nil {
			klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
		}

		return nil, onARMAPIErrorFail(prometheusServiceVM, err, "VM.WaitForCompletionRef failed for %s", *VMParameters.Name)
	}

	// Fetch VM details
	VM, err := VMFuture.Result(clients.vm)
	if err != nil {
		// Since machine creation failed, delete any infra resources created
		deleteErr := clients.deleteVMNicDisks(ctx, resourceGroupName, vmName, nicName, diskName, dataDiskNames)
		if deleteErr != nil {
			klog.Errorf("Error occurred during resource clean up: %s", deleteErr)
		}

		return nil, onARMAPIErrorFail(prometheusServiceVM, err, "VMFuture.Result failed for %s", *VMParameters.Name)
	}
	onARMAPISuccess(prometheusServiceVM, "VM.CreateOrUpdate")
	klog.V(3).Infof("VM has been created succesfully for %q", vmName)

	return &VM, nil
}

func (clients *azureDriverClients) getAllVMs(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachine, error) {
	var items []compute.VirtualMachine
	result, err := clients.vm.List(ctx, resourceGroupName)
	if err != nil {
		return items, onARMAPIErrorFail(prometheusServiceVM, err, "vm.List")
	}
	items = append(items, result.Values()...)
	for result.NotDone() {
		err = result.NextWithContext(ctx)
		if err != nil {
			return items, onARMAPIErrorFail(prometheusServiceVM, err, "vm.List")
		}
		items = append(items, result.Values()...)
	}
	onARMAPISuccess(prometheusServiceVM, "vm.List")
	return items, nil
}

func (clients *azureDriverClients) getAllNICs(ctx context.Context, resourceGroupName string) ([]network.Interface, error) {
	var items []network.Interface
	result, err := clients.nic.List(ctx, resourceGroupName)
	if err != nil {
		return items, onARMAPIErrorFail(prometheusServiceNIC, err, "nic.List")
	}
	items = append(items, result.Values()...)
	for result.NotDone() {
		err = result.NextWithContext(ctx)
		if err != nil {
			return items, onARMAPIErrorFail(prometheusServiceNIC, err, "nic.List")
		}
		items = append(items, result.Values()...)
	}
	onARMAPISuccess(prometheusServiceNIC, "nic.List")
	return items, nil
}

func (clients *azureDriverClients) getAllDisks(ctx context.Context, resourceGroupName string) ([]compute.Disk, error) {
	var items []compute.Disk
	result, err := clients.disk.ListByResourceGroup(ctx, resourceGroupName)
	if err != nil {
		return items, onARMAPIErrorFail(prometheusServiceDisk, err, "disk.ListByResourceGroup")
	}
	items = append(items, result.Values()...)
	for result.NotDone() {
		err = result.NextWithContext(ctx)
		if err != nil {
			return items, onARMAPIErrorFail(prometheusServiceDisk, err, "disk.ListByResourceGroup")
		}
		items = append(items, result.Values()...)
	}
	onARMAPISuccess(prometheusServiceDisk, "disk.ListByResourceGroup")
	return items, nil
}

// getRelevantVMs is a helper method used to list actual vm instances
func (clients *azureDriverClients) getRelevantVMs(ctx context.Context, machineID string, resourceGroupName string, location string, tags azureTags) (VMs, error) {
	var (
		listOfVMs         = make(VMs)
		searchClusterName = ""
		searchNodeRole    = ""
	)

	for key := range tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" ||
		searchNodeRole == "" ||
		resourceGroupName == "" {
		return listOfVMs, nil
	}

	machines, err := clients.getAllVMs(ctx, resourceGroupName)
	if err != nil {
		return listOfVMs, err
	}

	if len(machines) > 0 {
		for _, server := range machines {
			if !verifyAzureTags(server.Tags, searchClusterName, searchNodeRole) {
				klog.V(2).Infof("%q VM found, but not verified with tags %s and %s", *server.Name, searchClusterName, searchNodeRole)
				continue
			}

			instanceID := encodeMachineID(location, *server.Name)

			if machineID == "" {
				listOfVMs[instanceID] = *server.Name
			} else if machineID == instanceID {
				listOfVMs[instanceID] = *server.Name
				klog.V(3).Infof("Found machine with name: %q", *server.Name)
				break
			}
		}
	}

	return listOfVMs, nil
}

// getRelevantNICs is helper method used to list NICs
func (clients *azureDriverClients) getRelevantNICs(ctx context.Context, machineID string, resourceGroupName string, location string, tags azureTags) (VMs, error) {
	var (
		listOfVMs         = make(VMs)
		searchClusterName = ""
		searchNodeRole    = ""
	)

	for key := range tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" || searchNodeRole == "" || resourceGroupName == "" {
		return listOfVMs, nil
	}

	interfaces, err := clients.getAllNICs(ctx, resourceGroupName)
	if err != nil {
		return listOfVMs, err
	}

	if len(interfaces) > 0 {
		for _, nic := range interfaces {
			isNic, machineName := vmNameFromDependencyName(*nic.Name, nicSuffix)
			if !isNic {
				continue
			}

			if !verifyAzureTags(nic.Tags, searchClusterName, searchNodeRole) {
				klog.V(2).Infof("%q NIC found, but not verified with tags %s and %s", *nic.Name, searchClusterName, searchNodeRole)
				continue
			}

			instanceID := encodeMachineID(location, machineName)

			if machineID == "" {
				listOfVMs[instanceID] = machineName
			} else if machineID == instanceID {
				listOfVMs[instanceID] = machineName
				klog.V(3).Infof("Found nic with name %q, hence appending machine %q", *nic.Name, machineName)
				break
			}

		}
	}

	return listOfVMs, nil
}

// getRelevantDisks is a helper method used to list disks
func (clients *azureDriverClients) getRelevantDisks(ctx context.Context, machineID string, resourceGroupName string, location string, tags azureTags) (VMs, error) {
	var (
		listOfVMs         = make(VMs)
		searchClusterName = ""
		searchNodeRole    = ""
	)

	for key := range tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" ||
		searchNodeRole == "" ||
		resourceGroupName == "" {
		return listOfVMs, nil
	}

	disks, err := clients.getAllDisks(ctx, resourceGroupName)
	if err != nil {
		return listOfVMs, err
	}

	if len(disks) > 0 {
		for _, disk := range disks {
			if disk.OsType != "" {
				isDisk, machineName := vmNameFromDependencyName(*disk.Name, diskSuffix)
				if !isDisk {
					continue
				}

				if !verifyAzureTags(disk.Tags, searchClusterName, searchNodeRole) {
					klog.V(2).Infof("%q Disk found, but not verified with tags %s and %s", *disk.Name, searchClusterName, searchNodeRole)
					continue
				}

				instanceID := encodeMachineID(location, machineName)

				if machineID == "" {
					listOfVMs[instanceID] = machineName
				} else if machineID == instanceID {
					listOfVMs[instanceID] = machineName
					klog.V(3).Infof("Found disk with name %q, hence appending machine %q", *disk.Name, machineName)
					break
				}
			}
		}
	}

	return listOfVMs, nil
}

func (clients *azureDriverClients) fetchAttachedVMfromNIC(ctx context.Context, resourceGroupName, nicName string) (string, error) {
	nic, err := clients.nic.Get(ctx, resourceGroupName, nicName, "")
	if err != nil {
		return "", err
	}
	if nic.VirtualMachine == nil {
		return "", nil
	}
	return *nic.VirtualMachine.ID, nil
}

func (clients *azureDriverClients) fetchAttachedVMfromDisk(ctx context.Context, resourceGroupName, diskName string) (string, error) {
	disk, err := clients.disk.Get(ctx, resourceGroupName, diskName)
	if err != nil {
		return "", err
	}
	if disk.ManagedBy == nil {
		return "", nil
	}
	return *disk.ManagedBy, nil
}

func (clients *azureDriverClients) deleteVMNicDisks(ctx context.Context, resourceGroupName string, VMName string, nicName string, diskName string, dataDiskNames []string) error {

	// We try to fetch the VM, detach its data disks and finally delete it
	if vm, vmErr := clients.vm.Get(ctx, resourceGroupName, VMName, ""); vmErr == nil {

		clients.waitForDataDiskDetachment(ctx, resourceGroupName, vm)
		if deleteErr := clients.deleteVM(ctx, resourceGroupName, VMName); deleteErr != nil {
			return deleteErr
		}

		onARMAPISuccess(prometheusServiceVM, "VM Get was successful for %s", *vm.Name)
	} else if !notFound(vmErr) {
		// If some other error occurred, which is not 404 Not Found (the VM doesn't exist) then bubble up
		return onARMAPIErrorFail(prometheusServiceVM, vmErr, "vm.Get")
	}

	// Fetch the NIC and deleted it
	nicDeleter := func() error {
		if vmHoldingNic, err := clients.fetchAttachedVMfromNIC(ctx, resourceGroupName, nicName); err != nil {
			if notFound(err) {
				// Resource doesn't exist, no need to delete
				return nil
			}
			return err
		} else if vmHoldingNic != "" {
			return fmt.Errorf("Cannot delete NIC %s because it is attached to VM %s", nicName, vmHoldingNic)
		}

		return clients.deleteNIC(ctx, resourceGroupName, nicName)
	}

	// Fetch the system disk and delete it
	diskDeleter := clients.getDeleterForDisk(ctx, resourceGroupName, diskName)

	deleters := []func() error{nicDeleter, diskDeleter}

	if dataDiskNames != nil {
		for _, dataDiskName := range dataDiskNames {
			dataDiskDeleter := clients.getDeleterForDisk(ctx, resourceGroupName, dataDiskName)
			deleters = append(deleters, dataDiskDeleter)
		}
	}

	return runInParallel(deleters)
}

func (clients *azureDriverClients) getDeleterForDisk(ctx context.Context, resourceGroupName string, diskName string) func() error {
	return func() error {
		if vmHoldingDisk, err := clients.fetchAttachedVMfromDisk(ctx, resourceGroupName, diskName); err != nil {
			if notFound(err) {
				// Resource doesn't exist, no need to delete
				return nil
			}
			return err
		} else if vmHoldingDisk != "" {
			return fmt.Errorf("Cannot delete disk %s because it is attached to VM %s", diskName, vmHoldingDisk)
		}

		return clients.deleteDisk(ctx, resourceGroupName, diskName)
	}
}

// waitForDataDiskDetachment waits for data disks to be detached
func (clients *azureDriverClients) waitForDataDiskDetachment(ctx context.Context, resourceGroupName string, vm compute.VirtualMachine) error {
	klog.V(2).Infof("Data disk detachment began for %q", *vm.Name)
	defer klog.V(2).Infof("Data disk detached for %q", *vm.Name)

	if len(*vm.StorageProfile.DataDisks) > 0 {
		// There are disks attached hence need to detach them
		vm.StorageProfile.DataDisks = &[]compute.DataDisk{}

		future, err := clients.vm.CreateOrUpdate(ctx, resourceGroupName, *vm.Name, vm)
		if err != nil {
			return onARMAPIErrorFail(prometheusServiceVM, err, "Failed to CreateOrUpdate. Error Message - %s", err)
		}
		err = future.WaitForCompletionRef(ctx, clients.vm.Client)
		if err != nil {
			return onARMAPIErrorFail(prometheusServiceVM, err, "Failed to CreateOrUpdate. Error Message - %s", err)
		}
		onARMAPISuccess(prometheusServiceVM, "VM CreateOrUpdate was successful for %s", *vm.Name)
	}

	return nil
}

func (clients *azureDriverClients) deleteVM(ctx context.Context, resourceGroupName string, vmName string) error {
	klog.V(2).Infof("VM deletion has began for %q", vmName)
	defer klog.V(2).Infof("VM deleted for %q", vmName)

	future, err := clients.vm.Delete(ctx, resourceGroupName, vmName)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceVM, err, "vm.Delete")
	}
	err = future.WaitForCompletionRef(ctx, clients.vm.Client)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceVM, err, "vm.Delete")
	}
	onARMAPISuccess(prometheusServiceVM, "VM deletion was successful for %s", vmName)
	return nil
}

func (clients *azureDriverClients) checkOrphanNics(ctx context.Context, resourceGroupName, vmName string) error {
	klog.V(2).Infof("Check for NIC leftovers belonging to deleted machine %q", vmName)
	nicResults, err := clients.nic.List(ctx, resourceGroupName)
	if err != nil {
		return err
	}
	for _, nic := range nicResults.Values() {
		if nic.Name == nil || !strings.Contains(*nic.Name, vmName) {
			continue
		}
		klog.V(2).Infof("Found orphan NIC %q belonging to deleted machine %q", *nic.Name, vmName)
		if err := clients.deleteNIC(ctx, resourceGroupName, *nic.Name); err != nil && !notFound(err) {
			return err
		}
	}
	return nil
}

func (clients *azureDriverClients) checkOrphanDisks(ctx context.Context, resourceGroupName, vmName string) error {
	klog.V(2).Infof("Check for disk leftovers belonging to deleted machine %q", vmName)
	diskResults, err := clients.disk.ListByResourceGroup(ctx, resourceGroupName)
	if err != nil {
		return err
	}
	for _, disk := range diskResults.Values() {
		if disk.Name == nil || !strings.Contains(*disk.Name, vmName) {
			continue
		}
		klog.V(2).Infof("Found orphan disk %q belonging to deleted machine %q", *disk.Name, vmName)
		if err := clients.deleteDisk(ctx, resourceGroupName, *disk.Name); err != nil && !notFound(err) {
			return err
		}
	}
	return nil
}

func (clients *azureDriverClients) checkNICStatus(ctx context.Context, resourceGroupName string, nicName string, shouldExist bool) func() error {
	return func() error {
		nic, err := clients.nic.Get(ctx, resourceGroupName, nicName, "")

		// Case-1: If NIC should exist, check below if condition
		if shouldExist {
			if err == nil && nic.ID != nil {
				// NIC found
				return nil
			}

			klog.V(4).Infof("NIC %q not found", nicName)
			return fmt.Errorf("NIC %q not found", nicName)
		}

		// Case-2: If NIC should not exist, check below condition
		if err != nil && isResourceNotFoundError(err) {
			// NIC not found, hence deletion is successful
			return nil
		}

		klog.V(4).Infof("NIC %q found", nicName)
		return fmt.Errorf("NIC %q found", nicName)
	}
}

func (clients *azureDriverClients) deleteNIC(ctx context.Context, resourceGroupName string, nicName string) error {
	const (
		intialInterval     = 10 * time.Second
		maxInterval        = 2 * time.Minute
		maxElapsedTime     = 10 * time.Minute
		nicDeletionTimeout = 10 * time.Minute
	)

	klog.V(2).Infof("NIC delete started for %q", nicName)
	defer klog.V(2).Infof("NIC deleted for %q", nicName)

	nicDeletionCtx, cancel := context.WithTimeout(ctx, nicDeletionTimeout)
	defer cancel()

	future, err := clients.nic.Delete(nicDeletionCtx, resourceGroupName, nicName)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceNIC, err, "nic.Delete")
	}

	err = future.WaitForCompletionRef(nicDeletionCtx, clients.nic.Client)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceNIC, err, "nic.Delete")
	}

	err = backoff.WaitUntil(
		nicDeletionCtx,
		intialInterval,
		maxInterval,
		maxElapsedTime,
		clients.checkNICStatus(nicDeletionCtx, resourceGroupName, nicName, false),
	)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceNIC, err, "nic.Delete")
	}

	onARMAPISuccess(prometheusServiceNIC, "NIC deletion was successful for %s", nicName)
	return nil
}

func (clients *azureDriverClients) deleteDisk(ctx context.Context, resourceGroupName string, diskName string) error {
	klog.V(2).Infof("Disk delete started for %q", diskName)
	defer klog.V(2).Infof("Disk deleted for %q", diskName)

	future, err := clients.disk.Delete(ctx, resourceGroupName, diskName)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceDisk, err, "disk.Delete")
	}
	if err = future.WaitForCompletionRef(ctx, clients.disk.Client); err != nil {
		return onARMAPIErrorFail(prometheusServiceDisk, err, "disk.Delete")
	}
	onARMAPISuccess(prometheusServiceDisk, "Disk deletion was successful for %s", diskName)
	return nil
}

func onARMAPISuccess(prometheusService string, format string, v ...interface{}) {
	prometheusSuccess(prometheusService)
}

func onARMAPIErrorFail(prometheusService string, err error, format string, v ...interface{}) error {
	prometheusFail(prometheusService)
	return onErrorFail(err, format, v...)
}

func notFound(err error) bool {
	isDetailedError, _, detailedError := retrieveRequestID(err)
	return isDetailedError && detailedError.Response.StatusCode == 404
}

func retrieveRequestID(err error) (bool, string, *autorest.DetailedError) {
	switch err.(type) {
	case autorest.DetailedError:
		detailedErr := autorest.DetailedError(err.(autorest.DetailedError))
		if detailedErr.Response != nil {
			requestID := strings.Join(detailedErr.Response.Header["X-Ms-Request-Id"], "")
			return true, requestID, &detailedErr
		}
		return false, "", nil
	default:
		return false, "", nil
	}
}

// onErrorFail prints a failure message and exits the program if err is not nil.
func onErrorFail(err error, format string, v ...interface{}) error {
	if err != nil {
		message := fmt.Sprintf(format, v...)
		if hasRequestID, requestID, detailedErr := retrieveRequestID(err); hasRequestID {
			klog.Errorf("Azure ARM API call with x-ms-request-id=%s failed. %s: %s\n", requestID, message, *detailedErr)
		} else {
			klog.Errorf("%s: %s\n", message, err)
		}
	}
	return err
}

func runInParallel(funcs []func() error) error {
	//
	// Execute multiple functions (which return an error) as go functions concurrently.
	//
	var wg sync.WaitGroup
	wg.Add(len(funcs))

	errors := make([]error, len(funcs))
	for i, funOuter := range funcs {
		go func(results []error, idx int, funInner func() error) {
			defer wg.Done()
			if funInner == nil {
				results[idx] = fmt.Errorf("Received nil function")
				return
			}
			err := funInner()
			results[idx] = err
		}(errors, i, funOuter)
	}

	wg.Wait()

	var trimmedErrorMessages []string
	for _, e := range errors {
		if e != nil {
			trimmedErrorMessages = append(trimmedErrorMessages, e.Error())
		}
	}
	if len(trimmedErrorMessages) > 0 {
		return fmt.Errorf(strings.Join(trimmedErrorMessages, "\n"))
	}
	return nil
}

func encodeMachineID(location, vmName string) string {
	return fmt.Sprintf("azure:///%s/%s", location, vmName)
}

func decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}

const (
	nicSuffix      = "-nic"
	diskSuffix     = "-os-disk"
	dataDiskSuffix = "-data-disk"
)

func getAzureDataDiskNames(azureDataDisks []v1alpha1.AzureDataDisk, vmname, suffix string) []string {
	azureDataDiskNames := make([]string, len(azureDataDisks))
	for i, disk := range azureDataDisks {
		var diskLun *int32
		if disk.Lun != nil {
			diskLun = disk.Lun
		} else {
			lun := int32(i)
			diskLun = &lun
		}
		azureDataDiskNames[i] = dependencyNameFromVMNameAndDependency(getAzureDataDiskPrefix(disk.Name, diskLun), vmname, suffix)
	}
	return azureDataDiskNames
}

func getAzureDataDiskPrefix(name string, lun *int32) string {
	if name != "" {
		return fmt.Sprintf("%s-%d", name, *lun)
	}
	return fmt.Sprintf("%d", *lun)
}

func dependencyNameFromVMNameAndDependency(dependency, vmName, suffix string) string {
	return vmName + "-" + dependency + suffix
}

func dependencyNameFromVMName(vmName, suffix string) string {
	return vmName + suffix
}

func vmNameFromDependencyName(dependencyName, suffix string) (hasProperSuffix bool, vmName string) {
	if strings.HasSuffix(dependencyName, suffix) {
		hasProperSuffix = true
		vmName = dependencyName[:len(dependencyName)-len(suffix)]
	} else {
		hasProperSuffix = false
		vmName = ""
	}
	return
}

const (
	prometheusServiceSubnet = "subnet"
	prometheusServiceVM     = "virtual_machine"
	prometheusServiceNIC    = "network_interfaces"
	prometheusServiceDisk   = "disks"
)

func prometheusSuccess(service string) {
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": service}).Inc()
}

func prometheusFail(service string) {
	metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": service}).Inc()
}

// GetVolNames parses volume names from pv specs
func (d *AzureDriver) GetVolNames(specs []corev1.PersistentVolumeSpec) ([]string, error) {
	names := []string{}
	for i := range specs {
		spec := &specs[i]
		if spec.AzureDisk != nil {
			name := spec.AzureDisk.DiskName
			names = append(names, name)
		} else if spec.CSI != nil && spec.CSI.Driver == azureDiskDriverName && spec.CSI.VolumeHandle != "" {
			name := spec.CSI.VolumeHandle
			names = append(names, name)
		}
	}
	return names, nil
}

func retry(fn func() error, retries int, delay time.Duration) error {
	var retryCount = 0
	for {
		err := fn()
		if err == nil {
			return nil
		}
		retryCount++
		if retryCount == retries {
			return fmt.Errorf("max amount of retries reached (%s)", err.Error())
		}
		time.Sleep(delay)
	}
}

func verifyAzureTags(tags map[string]*string, clusterNameTag, nodeRoleTag string) bool {
	if tags == nil {
		return false
	}

	var clusterNameMatched, nodeRoleMatched bool
	for key := range tags {
		if strings.Contains(key, clusterNameTag) {
			clusterNameMatched = true
		}
		if strings.Contains(key, nodeRoleTag) {
			nodeRoleMatched = true
		}
	}
	if !clusterNameMatched || !nodeRoleMatched {
		return false
	}

	return true
}

// isResourceNotFoundError returns true when resource is not found at provider
func isResourceNotFoundError(err error) bool {
	const (
		resourceNotFoundStatusCode = "404"
	)

	if e, ok := err.(autorest.DetailedError); ok {
		statusCode := fmt.Sprintf("%v", e.StatusCode)
		if statusCode == resourceNotFoundStatusCode {
			return true
		}
	}

	return false
}
