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

// Package driver contains the cloud provider specific implementations to manage machines
package driver

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/Azure/azure-sdk-for-go/arm/disk"
	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/glog"
)

// AzureDriver is the driver struct for holding azure machine information
type AzureDriver struct {
	AzureMachineClass *v1alpha1.AzureMachineClass
	CloudConfig       *corev1.Secret
	UserData          string
	MachineID         string
	MachineName       string
}

var (
	interfacesClient network.InterfacesClient
	vmClient         compute.VirtualMachinesClient
	subnetClient     network.SubnetsClient
	diskClient       disk.DisksClient
)

// Create method is used to create an azure machine
func (d *AzureDriver) Create() (string, string, error) {
	d.setup()
	var (
		vmName        = d.MachineName //VM-name has to be in small letters
		nicName       = vmName + "-nic"
		diskName      = vmName + "-os-disk"
		location      = d.AzureMachineClass.Spec.Location
		resourceGroup = d.AzureMachineClass.Spec.ResourceGroup
		UserDataEnc   = base64.StdEncoding.EncodeToString([]byte(d.UserData))
	)

	subnet, err := subnetClient.Get(
		resourceGroup,
		d.AzureMachineClass.Spec.SubnetInfo.VnetName,
		d.AzureMachineClass.Spec.SubnetInfo.SubnetName,
		"",
	)
	err = onErrorFail(err, fmt.Sprintf("subnetClient.Get failed for subnet %q", subnet.Name))

	enableIPForwarding := true
	nicParameters := network.Interface{
		Location: &location,
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			IPConfigurations: &[]network.InterfaceIPConfiguration{
				{
					Name: &nicName,
					InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: network.Dynamic,
						Subnet: &subnet,
					},
				},
			},
			EnableIPForwarding: &enableIPForwarding,
		},
	}
	cancel := make(chan struct{})
	_, errChan := interfacesClient.CreateOrUpdate(resourceGroup, nicName, nicParameters, cancel)
	err = onErrorFail(<-errChan, fmt.Sprintf("interfacesClient.CreateOrUpdate for NIC '%s' failed", nicName))
	nicParameters, err = interfacesClient.Get(resourceGroup, nicName, "")
	err = onErrorFail(err, fmt.Sprintf("interfaces.Get for NIC '%s' failed", nicName))

	// Add tags to the created machine
	tagList := map[string]*string{}
	for idx, element := range d.AzureMachineClass.Spec.Tags {
		tagList[idx] = to.StringPtr(element)
	}

	vm := compute.VirtualMachine{
		Location: &location,
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			HardwareProfile: &compute.HardwareProfile{
				VMSize: compute.VirtualMachineSizeTypes(d.AzureMachineClass.Spec.Properties.HardwareProfile.VMSize),
			},
			StorageProfile: &compute.StorageProfile{
				ImageReference: &compute.ImageReference{
					Publisher: &d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Publisher,
					Offer:     &d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Offer,
					Sku:       &d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Sku,
					Version:   &d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Version,
				},
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
						ID: nicParameters.ID,
						NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
							Primary: to.BoolPtr(false),
						},
					},
				},
			},
			AvailabilitySet: &compute.SubResource{
				ID: &d.AzureMachineClass.Spec.Properties.AvailabilitySet.ID,
			},
		},
		Tags: &tagList,
	}
	_, errChan = vmClient.CreateOrUpdate(resourceGroup, vmName, vm, cancel)
	err = onErrorFail(<-errChan, "createVM failed")
	//glog.Infof("Created machine '%s' successfully\n", vmName)

	return d.encodeMachineID(location, vmName), vmName, err
}

// Delete method is used to delete an azure machine
func (d *AzureDriver) Delete() error {
	d.setup()
	var (
		vmName        = d.decodeMachineID(d.MachineID)
		nicName       = vmName + "-nic"
		diskName      = vmName + "-os-disk"
		resourceGroup = d.AzureMachineClass.Spec.ResourceGroup
		cancel        = make(chan struct{})
	)

	_, errChan := vmClient.Delete(resourceGroup, vmName, cancel)
	err := onErrorFail(<-errChan, fmt.Sprintf("vmClient.Delete failed for '%s'", vmName))

	_, errChan = interfacesClient.Delete(resourceGroup, nicName, cancel)
	err = onErrorFail(<-errChan, fmt.Sprintf("interfacesClient.Delete for NIC '%s' failed", nicName))

	_, errChan = diskClient.Delete(resourceGroup, diskName, cancel)
	err = onErrorFail(<-errChan, fmt.Sprintf("diskClient.Delete for NIC '%s' failed", nicName))

	//glog.Infof("Deleted machine '%s' successfully\n", vmName)

	return err
}

// GetExisting method is used to fetch the machineID for an azure machine
func (d *AzureDriver) GetExisting() (string, error) {
	return d.MachineID, nil
}

func (d *AzureDriver) setup() {
	subscriptionID := strings.TrimSpace(string(d.CloudConfig.Data["azureSubscriptionId"]))
	authorizer, err := d.getAuthorizer(azure.PublicCloud)
	onErrorFail(err, "utils.GetAuthorizer failed")
	createClients(subscriptionID, authorizer)
}

func (d *AzureDriver) getAuthorizer(env azure.Environment) (*autorest.BearerAuthorizer, error) {
	tenantID := strings.TrimSpace(string(d.CloudConfig.Data["azureTenantId"]))
	clientID := strings.TrimSpace(string(d.CloudConfig.Data["azureClientId"]))
	clientSecret := strings.TrimSpace(string(d.CloudConfig.Data["azureClientSecret"]))

	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, err
	}

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, clientID, clientSecret, env.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	return autorest.NewBearerAuthorizer(spToken), nil
}

func createClients(subscriptionID string, authorizer *autorest.BearerAuthorizer) {
	subnetClient = network.NewSubnetsClient(subscriptionID)
	subnetClient.Authorizer = authorizer

	interfacesClient = network.NewInterfacesClient(subscriptionID)
	interfacesClient.Authorizer = authorizer

	vmClient = compute.NewVirtualMachinesClient(subscriptionID)
	vmClient.Authorizer = authorizer

	diskClient = disk.NewDisksClient(subscriptionID)
	diskClient.Authorizer = authorizer
}

// onErrorFail prints a failure message and exits the program if err is not nil.
func onErrorFail(err error, message string) error {
	if err != nil {
		glog.Infof("%s: %s\n", message, err)
		return err
	}
	return nil
}

// getEnvVarOrExit returns the value of specified environment variable or terminates if it's not defined.
func getEnvVarOrExit(varName string) string {
	value := os.Getenv(varName)
	if value == "" {
		fmt.Printf("Missing environment variable '%s'\n", varName)
		os.Exit(1)
	}

	return value
}

func (d *AzureDriver) encodeMachineID(location, vmName string) string {
	return fmt.Sprintf("azure:///%s/%s", location, vmName)
}

func (d *AzureDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}
