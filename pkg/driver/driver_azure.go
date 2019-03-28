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
	"encoding/base64"
	"fmt"
	"strings"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
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

	// Add tags to the machine resources
	tagList := map[string]*string{}
	for idx, element := range d.AzureMachineClass.Spec.Tags {
		tagList[idx] = to.StringPtr(element)
	}

	subnet, err := subnetClient.Get(
		resourceGroup,
		d.AzureMachineClass.Spec.SubnetInfo.VnetName,
		d.AzureMachineClass.Spec.SubnetInfo.SubnetName,
		"",
	)
	err = onErrorFail(err, fmt.Sprintf("subnetClient.Get failed for subnet %q", d.AzureMachineClass.Spec.SubnetInfo.SubnetName))
	if err != nil {
		return "Error", "Error", err
	}

	enableIPForwarding := true
	nicParameters := network.Interface{
		Location: &location,
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			IPConfigurations: &[]network.InterfaceIPConfiguration{
				{
					Name: &nicName,
					InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: network.Dynamic,
						Subnet:                    &subnet,
					},
				},
			},
			EnableIPForwarding: &enableIPForwarding,
		},
		Tags: &tagList,
	}

	var cancel chan struct{}

	_, errChan := interfacesClient.CreateOrUpdate(resourceGroup, nicName, nicParameters, cancel)
	err = onErrorFail(<-errChan, fmt.Sprintf("interfacesClient.CreateOrUpdate for NIC '%s' failed", nicName))
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
		return "Error", "Error", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()

	nicParameters, err = interfacesClient.Get(resourceGroup, nicName, "")
	err = onErrorFail(err, fmt.Sprintf("interfaces.Get for NIC '%s' failed", nicName))
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
		// Delete the created NIC
		_, errChan = interfacesClient.Delete(resourceGroup, nicName, cancel)
		errNIC := onErrorFail(<-errChan, fmt.Sprintf("Getting NIC details failed, inturn deletion for corresponding newly created NIC '%s' failed", nicName))
		if errNIC != nil {
			metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
			// When deletion of NIC returns an error
			return "Error", "Error", errNIC
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()

		return "Error", "Error", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()

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

	cancel = nil
	_, errChan = vmClient.CreateOrUpdate(resourceGroup, vmName, vm, cancel)
	err = onErrorFail(<-errChan, "createVM failed")
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()
		// Delete the created NIC
		_, errChan = interfacesClient.Delete(resourceGroup, nicName, cancel)
		errNIC := onErrorFail(<-errChan, fmt.Sprintf("Creation of VM failed, inturn deletion for corresponding newly created NIC '%s' failed", nicName))
		if errNIC != nil {
			// When deletion of NIC returns an error
			metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
			return "Error", "Error", errNIC
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()

		return "Error", "Error", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()

	return d.encodeMachineID(location, vmName), vmName, err
}

// Delete method is used to delete an azure machine
func (d *AzureDriver) Delete() error {

	result, err := d.GetVMs(d.MachineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machine-ID
		glog.V(2).Infof("No VM matching the machine-ID found on the provider %q", d.MachineID)
		return nil
	}

	d.setup()
	var (
		vmName        = d.decodeMachineID(d.MachineID)
		nicName       = vmName + "-nic"
		diskName      = vmName + "-os-disk"
		resourceGroup = d.AzureMachineClass.Spec.ResourceGroup
		cancel        = make(chan struct{})
	)

	listOfResources := make(map[string]string)
	d.getvms(d.MachineID, listOfResources)
	if len(listOfResources) != 0 {

		err = d.waitForDataDiskDetachment(vmName)
		if err != nil {
			return err
		}
		glog.V(2).Infof("Disk detachment was successful for %q", vmName)

		_, errChan := vmClient.Delete(resourceGroup, vmName, cancel)
		err = onErrorFail(<-errChan, fmt.Sprintf("vmClient.Delete failed for '%s'", vmName))
		if err != nil {
			metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()
			return err
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()
		glog.V(2).Infof("VM deletion was successful for %q", vmName)

	} else {
		glog.Warningf("VM was not found for %q", vmName)
	}

	listOfResources = make(map[string]string)
	d.getnics(d.MachineID, listOfResources)
	if len(listOfResources) != 0 {
		_, errChan := interfacesClient.Delete(resourceGroup, nicName, cancel)
		err = onErrorFail(<-errChan, fmt.Sprintf("interfacesClient.Delete for NIC '%s' failed", nicName))
		if err != nil {
			metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
			return err
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
		glog.V(2).Infof("NIC deletion was successful for %q", nicName)
	} else {
		glog.Warningf("NIC was not found for %q", nicName)
	}

	listOfResources = make(map[string]string)
	d.getdisks(d.MachineID, listOfResources)
	if len(listOfResources) != 0 {
		_, errChan := diskClient.Delete(resourceGroup, diskName, cancel)
		err = onErrorFail(<-errChan, fmt.Sprintf("diskClient.Delete for NIC '%s' failed", diskName))
		if err != nil {
			metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "disks"}).Inc()
			return err
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "disks"}).Inc()
		glog.V(2).Infof("OS-Disk deletion was successful for %q", diskName)
	} else {
		glog.Warningf("OS-Disk was not found for %q", diskName)
	}

	return err
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
func (d *AzureDriver) GetVMs(machineID string) (VMs, error) {
	var err error
	listOfVMs := make(map[string]string)

	err = d.getvms(machineID, listOfVMs)
	if err != nil {
		return listOfVMs, err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()

	err = d.getnics(machineID, listOfVMs)
	if err != nil {
		return listOfVMs, err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()

	err = d.getdisks(machineID, listOfVMs)
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "disks"}).Inc()
	return listOfVMs, err
}

// getvms is a helper method used to list actual vm instances
func (d *AzureDriver) getvms(machineID string, listOfVMs VMs) error {

	searchClusterName := ""
	searchNodeRole := ""

	for key := range d.AzureMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" ||
		searchNodeRole == "" ||
		d.AzureMachineClass.Spec.ResourceGroup == "" {
		return nil
	}

	d.setup()
	result, err := vmClient.List(d.AzureMachineClass.Spec.ResourceGroup)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()
		glog.Errorf("Failed to list VMs. Error Message - %s", err)
		return err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()

	if result.Value != nil && len(*result.Value) > 0 {
		for _, server := range *result.Value {

			clusterName := ""
			nodeRole := ""

			if server.Tags == nil {
				continue
			}
			for key := range *server.Tags {
				if strings.Contains(key, "kubernetes.io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes.io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {

				instanceID := d.encodeMachineID(d.AzureMachineClass.Spec.Location, *server.Name)

				if machineID == "" {
					listOfVMs[instanceID] = *server.Name
				} else if machineID == instanceID {
					listOfVMs[instanceID] = *server.Name
					glog.V(3).Infof("Found machine with name: %q", *server.Name)
					break
				}
			}

		}
	}

	return nil
}

// waitForDataDiskDetachment waits for data disks to be detached
func (d *AzureDriver) waitForDataDiskDetachment(machineID string) error {
	var cancel <-chan struct{}
	d.setup()

	vm, err := vmClient.Get(d.AzureMachineClass.Spec.ResourceGroup, machineID, compute.InstanceView)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()
		glog.Errorf("Failed to list VMs. Error Message - %s", err)
		return err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "virtual_machine"}).Inc()

	if len(*vm.StorageProfile.DataDisks) > 0 {
		// There are disks attached hence need to detach them
		vm.StorageProfile.DataDisks = &[]compute.DataDisk{}

		_, errChan := vmClient.CreateOrUpdate(d.AzureMachineClass.Spec.ResourceGroup, machineID, vm, cancel)
		err = onErrorFail(<-errChan, fmt.Sprintf("vmClient.CreateOrUpdate failed for '%s'", machineID))
		if err != nil {
			return fmt.Errorf("Cannot detach disks for machine. Error: %v", err)
		}
	}

	return nil
}

// getnics is helper method used to list NICs
func (d *AzureDriver) getnics(machineID string, listOfVMs VMs) error {

	searchClusterName := ""
	searchNodeRole := ""

	for key := range d.AzureMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" ||
		searchNodeRole == "" ||
		d.AzureMachineClass.Spec.ResourceGroup == "" {
		return nil
	}

	d.setup()
	result, err := interfacesClient.List(d.AzureMachineClass.Spec.ResourceGroup)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
		glog.Errorf("Failed to list NICs. Error Message - %s", err)
		return err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()

	if result.Value != nil && len(*result.Value) > 0 {
		for _, nic := range *result.Value {

			clusterName := ""
			nodeRole := ""

			if nic.Tags == nil {
				continue
			}
			for key := range *nic.Tags {
				if strings.Contains(key, "kubernetes.io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes.io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {

				machineName := *nic.Name
				// Removing last 4 characters from NIC name to get the machine object name
				machineName = machineName[:len(machineName)-4]
				instanceID := d.encodeMachineID(d.AzureMachineClass.Spec.Location, machineName)

				if machineID == "" {
					listOfVMs[instanceID] = machineName
				} else if machineID == instanceID {
					listOfVMs[instanceID] = machineName
					glog.V(3).Infof("Found nic with name %q, hence appending machine %q", *nic.Name, machineName)
					break
				}
			}

		}
	}

	return nil
}

// getdisks is a helper method used to list disks
func (d *AzureDriver) getdisks(machineID string, listOfVMs VMs) error {

	searchClusterName := ""
	searchNodeRole := ""

	for key := range d.AzureMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" ||
		searchNodeRole == "" ||
		d.AzureMachineClass.Spec.ResourceGroup == "" {
		return nil
	}

	d.setup()
	result, err := diskClient.List()
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": "disks"}).Inc()
		glog.Errorf("Failed to list OS Disks. Error Message - %s", err)
		return err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "disks"}).Inc()

	if result.Value != nil && len(*result.Value) > 0 {
		for _, disk := range *result.Value {

			clusterName := ""
			nodeRole := ""

			if disk.Tags == nil {
				continue
			}
			for key := range *disk.Tags {
				if strings.Contains(key, "kubernetes.io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes.io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {

				machineName := *disk.Name
				// Removing last 8 characters from disk name to get the machine object name
				machineName = machineName[:len(machineName)-8]
				instanceID := d.encodeMachineID(d.AzureMachineClass.Spec.Location, machineName)

				if machineID == "" {
					listOfVMs[instanceID] = machineName
				} else if machineID == instanceID {
					listOfVMs[instanceID] = machineName
					glog.V(3).Infof("Found disk with name %q, hence appending machine %q", *disk.Name, machineName)
					break
				}
			}

		}
	}

	return nil
}

func (d *AzureDriver) setup() {
	subscriptionID := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureSubscriptionID]))
	authorizer, err := d.getAuthorizer(azure.PublicCloud)
	onErrorFail(err, "utils.GetAuthorizer failed")
	createClients(subscriptionID, authorizer)
}

func (d *AzureDriver) getAuthorizer(env azure.Environment) (*autorest.BearerAuthorizer, error) {
	tenantID := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureTenantID]))
	clientID := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureClientID]))
	clientSecret := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureClientSecret]))

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
		glog.Errorf("%s: %s\n", message, err)
		return err
	}
	return nil
}

func (d *AzureDriver) encodeMachineID(location, vmName string) string {
	return fmt.Sprintf("azure:///%s/%s", location, vmName)
}

func (d *AzureDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}
