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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
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

func (d *AzureDriver) getDeploymentParameters(vmName string) map[string]interface{} {
	return map[string]interface{}{
		"vmName":                        vmName,
		"nicName":                       dependencyNameFromVMName(vmName, nicSuffix),
		"diskName":                      dependencyNameFromVMName(vmName, diskSuffix),
		"location":                      d.AzureMachineClass.Spec.Location,
		"vnetName":                      d.AzureMachineClass.Spec.SubnetInfo.VnetName,
		"subnetName":                    d.AzureMachineClass.Spec.SubnetInfo.SubnetName,
		"nicEnableIPForwarding":         true,
		"vmSize":                        d.AzureMachineClass.Spec.Properties.HardwareProfile.VMSize,
		"adminUsername":                 d.AzureMachineClass.Spec.Properties.OsProfile.AdminUsername,
		"customData":                    d.UserData,
		"disablePasswordAuthentication": d.AzureMachineClass.Spec.Properties.OsProfile.LinuxConfiguration.DisablePasswordAuthentication,
		"sshPublicKeyPath":              d.AzureMachineClass.Spec.Properties.OsProfile.LinuxConfiguration.SSH.PublicKeys.Path,
		"sshPublicKeyData":              d.AzureMachineClass.Spec.Properties.OsProfile.LinuxConfiguration.SSH.PublicKeys.KeyData,
		"osDiskCaching":                 d.AzureMachineClass.Spec.Properties.StorageProfile.OsDisk.Caching,
		"osDiskSizeGB":                  d.AzureMachineClass.Spec.Properties.StorageProfile.OsDisk.DiskSizeGB,
		"diskStorageAccountType":        d.AzureMachineClass.Spec.Properties.StorageProfile.OsDisk.ManagedDisk.StorageAccountType,
		"diskImagePublisher":            d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Publisher,
		"diskImageOffer":                d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Offer,
		"diskImageSku":                  d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Sku,
		"diskImageVersion":              d.AzureMachineClass.Spec.Properties.StorageProfile.ImageReference.Version,
		"availabilitySetID":             d.AzureMachineClass.Spec.Properties.AvailabilitySet.ID,
		"tags":                          d.AzureMachineClass.Spec.Tags,
	}
}

// Create method is used to create an azure machine
func (d *AzureDriver) Create() (string, string, error) {
	clients, err := d.setup()
	if err != nil {
		return "Error", "Error", err
	}

	var (
		ctx                  = context.Background()
		vmName               = strings.ToLower(d.MachineName)
		location             = d.AzureMachineClass.Spec.Location
		resourceGroupName    = d.AzureMachineClass.Spec.ResourceGroup
		deploymentName       = fmt.Sprintf("gardener-shoot-deployment-%s", vmName) // Deployments per resource group in the deployment history == 800
		deploymentTemplate   = clients.getTemplate()
		deploymentParameters = d.getDeploymentParameters(vmName)
	)

	_, err = clients.createDeployment(ctx, resourceGroupName, deploymentName, deploymentTemplate, deploymentParameters)
	if err != nil {
		return "Error", "Error", err
	}

	return encodeMachineID(location, vmName), vmName, nil
}

// Delete method is used to delete an azure machine
func (d *AzureDriver) Delete() error {
	clients, err := d.setup()
	if err != nil {
		return err
	}

	var (
		ctx               = context.Background()
		vmName            = decodeMachineID(d.MachineID)
		nicName           = dependencyNameFromVMName(vmName, nicSuffix)
		diskName          = dependencyNameFromVMName(vmName, diskSuffix)
		resourceGroupName = d.AzureMachineClass.Spec.ResourceGroup
	)

	return clients.deleteVMNicDisk(ctx, resourceGroupName, vmName, nicName, diskName)
}

/*
func (d *AzureDriver) deleteOldImplementation() error {
	clients, err := d.setup()
	if err != nil {
		return err
	}

	var (
		ctx               = context.Background()
		vmName            = decodeMachineID(d.MachineID)
		resourceGroupName = d.AzureMachineClass.Spec.ResourceGroup
		location          = d.AzureMachineClass.Spec.Location
		tags              = d.AzureMachineClass.Spec.Tags
		nicName           = dependencyNameFromVMName(vmName, nicSuffix)
		diskName          = dependencyNameFromVMName(vmName, diskSuffix)
	)

	//
	// TODO Check why SAP uses this
	//

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

	if listOfVMs, _ := clients.getRelevantVMs(ctx, d.MachineID, resourceGroupName, location, tags); len(listOfVMs) != 0 {
		if err := clients.powerOffVM(ctx, resourceGroupName, vmName); err != nil {
			return err
		}
		if err := clients.deleteVM(ctx, resourceGroupName, vmName); err != nil {
			return err
		}
	} else {
		glog.Warningf("VM was not found for %q", vmName)
	}

	if listOfVMs, _ := clients.getRelevantNICs(ctx, d.MachineID, resourceGroupName, location, tags); len(listOfVMs) != 0 {
		if err := clients.deleteNIC(ctx, resourceGroupName, nicName); err != nil {
			return err
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "network_interfaces"}).Inc()
		glog.V(2).Infof("NIC deletion was successful for %q", nicName)
	} else {
		glog.Warningf("NIC was not found for %q", nicName)
	}

	if listOfVMs, _ := clients.getRelevantDisks(ctx, d.MachineID, resourceGroupName, location, tags); len(listOfVMs) != 0 {
		if err := clients.deleteDisk(ctx, resourceGroupName, diskName); err != nil {
			return err
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": "disks"}).Inc()
		glog.V(2).Infof("OS-Disk deletion was successful for %q", diskName)
	} else {
		glog.Warningf("OS-Disk was not found for %q", diskName)
	}

	return err
}
*/

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

func (d *AzureDriver) setup() (*azureDriverClients, error) {
	var (
		subscriptionID = strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureSubscriptionID]))
		tenantID       = strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureTenantID]))
		clientID       = strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureClientID]))
		clientSecret   = strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AzureClientSecret]))
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

	diskClient := compute.NewDisksClient(subscriptionID)
	diskClient.Authorizer = authorizer

	deploymentsClient := resources.NewDeploymentsClient(subscriptionID)
	deploymentsClient.Authorizer = authorizer

	return &azureDriverClients{subnet: subnetClient, nic: interfacesClient, vm: vmClient, disk: diskClient, deployments: deploymentsClient}, nil
}

func (clients *azureDriverClients) getTemplate() string {
	return `{
		"$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
		"contentVersion": "1.0.0.0",
		"parameters": {
			"vmName":                        { "type": "string" },
			"nicName":                       { "type": "string" },
			"diskName":                      { "type": "string" },
			"location":                      { "type": "string" },
			"vnetName":                      { "type": "string" },
			"subnetName":                    { "type": "string" },
			"nicEnableIPForwarding":         { "type": "bool"   },
			"vmSize":                        { "type": "string" },
			"adminUsername":                 { "type": "string" },
			"disablePasswordAuthentication": { "type": "bool"   },
			"sshPublicKeyPath":              { "type": "string" },
			"sshPublicKeyData":              { "type": "string" },
			"customData":                    { "type": "string" },
			"diskStorageAccountType":        { "type": "string" },
			"diskImagePublisher":            { "type": "string" },
			"diskImageOffer":                { "type": "string" },
			"diskImageSku":                  { "type": "string" },
			"diskImageVersion":              { "type": "string" },
			"osDiskCaching":                 { "type": "string" },
			"osDiskSizeGB":                  { "type": "int"    },
			"availabilitySetID":             { "type": "string" },
			"tags":                          { "type": "object" }
		},
		"variables": {
			"apiVersions": {
				"networkInterfaces": "2018-11-01",
				"virtualMachines":   "2018-10-01"
			}
		},
		"resources": [
			{
				"type":       "Microsoft.Network/networkInterfaces",
				"apiVersion": "[variables('apiVersions').networkInterfaces]",
				"name":       "[parameters('nicName')]",
				"location":   "[parameters('location')]",
				"tags":       "[parameters('tags')]",
				"properties": {
					"enableIPForwarding": "[parameters('nicEnableIPForwarding')]",
					"ipConfigurations": [
						{
							"name": "[parameters('nicName')]",
							"properties": {
								"privateIPAllocationMethod": "Dynamic",
								"subnet": {
									"id": "[concat(resourceId('Microsoft.Network/virtualNetworks', parameters('vnetName')), '/subnets/', parameters('subnetName'))]"
								}
							}
						}
					]
				}
			},
			{
				"type":       "Microsoft.Compute/virtualMachines",
				"apiVersion": "[variables('apiVersions').virtualMachines]",
				"name":       "[parameters('vmName')]",
				"location":   "[parameters('location')]",
				"tags":       "[parameters('tags')]",
				"dependsOn": [
					"[concat('Microsoft.Network/networkInterfaces/', parameters('nicName'))]"
				],
				"properties": {
					"hardwareProfile": { "vmSize": "[parameters('vmSize')]" },
					"storageProfile": {
						"imageReference": {
							"publisher": "[parameters('diskImagePublisher')]",
							"offer":     "[parameters('diskImageOffer')]",
							"sku":       "[parameters('diskImageSku')]",
							"version":   "[parameters('diskImageVersion')]"
						},
						"osDisk": {
							"createOption": "FromImage",
							"name":         "[parameters('diskName')]",
							"caching":      "[parameters('osDiskCaching')]",
							"diskSizeGB":   "[parameters('osDiskSizeGB')]",
							"managedDisk": {
								"storageAccountType": "[parameters('diskStorageAccountType')]"
							}
						}
					},
					"osProfile": {
						"computerName": "[parameters('vmName')]",
						"adminUsername": "[parameters('adminUsername')]",
						"customData": "[base64(string(parameters('customData')))]",
						"linuxConfiguration": {
							"provisionVMAgent": false,
							"disablePasswordAuthentication": "[parameters('disablePasswordAuthentication')]",
							"ssh": {
								"publicKeys": [
									{
										"path":    "[parameters('sshPublicKeyPath')]",
										"keyData": "[parameters('sshPublicKeyData')]"
									}
								]
							}
						}
					},
					"networkProfile": {
						"networkInterfaces": [
							{
								"id": "[resourceId('Microsoft.Network/networkInterfaces', parameters('nicName'))]",
								"properties": { "primary": false }
							}
						]
					},
					"availabilitySet": { "id" : "[parameters('availabilitySetID')]" }
				}
			}
		]
	}`
}

func (clients *azureDriverClients) createDeployment(ctx context.Context, resourceGroupName string, deploymentName string, templateJSON string, plainParameters map[string]interface{}) (resources.DeploymentExtended, error) {
	template := make(map[string]interface{})
	err := json.Unmarshal([]byte(templateJSON), &template)
	if err != nil {
		return resources.DeploymentExtended{}, err
	}

	// little helper to wrap values in that { "value": foo } structure
	createARMParameters := func(m map[string]interface{}) map[string]interface{} {
		result := make(map[string]interface{})
		for k, v := range m {
			result[k] = map[string]interface{}{"value": v}
		}
		return result
	}

	deploymentFuture, err := clients.deployments.CreateOrUpdate(
		ctx, resourceGroupName, deploymentName,
		resources.Deployment{
			Properties: &resources.DeploymentProperties{
				Template:   template,
				Parameters: createARMParameters(plainParameters),
				Mode:       resources.Incremental,
			},
		},
	)
	if err != nil {
		return resources.DeploymentExtended{}, onARMAPIErrorFail(prometheusServiceDeployment, err, "deployments.CreateOrUpdate")
	}

	err = deploymentFuture.Future.WaitForCompletionRef(ctx, clients.deployments.BaseClient.Client)
	if err != nil {
		return resources.DeploymentExtended{}, err
	}

	de, err := deploymentFuture.Result(clients.deployments)
	if err != nil {
		return de, onARMAPIErrorFail(prometheusServiceDeployment, err, "deployments.CreateOrUpdate")
	}

	onARMAPISuccess(prometheusServiceDeployment, "deployments.CreateOrUpdate")
	return de, err
}

func (clients *azureDriverClients) getAllVMs(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachine, error) {
	var items []compute.VirtualMachine
	result, err := clients.vm.List(ctx, resourceGroupName)
	if err != nil {
		return items, onARMAPIErrorFail(prometheusServiceVM, err, "vm.List")
	}
	for _, item := range result.Values() {
		items = append(items, item)
	}
	for result.NotDone() {
		err = result.NextWithContext(ctx)
		if err != nil {
			return items, onARMAPIErrorFail(prometheusServiceVM, err, "vm.List")
		}
		for _, item := range result.Values() {
			items = append(items, item)
		}
	}
	onARMAPISuccess(prometheusServiceVM, "deployments.CreateOrUpdate")
	return items, nil
}

func (clients *azureDriverClients) getAllNICs(ctx context.Context, resourceGroupName string) ([]network.Interface, error) {
	var items []network.Interface
	result, err := clients.nic.List(ctx, resourceGroupName)
	if err != nil {
		return items, onARMAPIErrorFail(prometheusServiceNIC, err, "nic.List")
	}
	for _, item := range result.Values() {
		items = append(items, item)
	}
	for result.NotDone() {
		err = result.NextWithContext(ctx)
		if err != nil {
			return items, onARMAPIErrorFail(prometheusServiceNIC, err, "nic.List")
		}
		for _, item := range result.Values() {
			items = append(items, item)
		}

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
	for _, item := range result.Values() {
		items = append(items, item)
	}
	for result.NotDone() {
		err = result.NextWithContext(ctx)
		if err != nil {
			return items, onARMAPIErrorFail(prometheusServiceDisk, err, "disk.ListByResourceGroup")
		}
		for _, item := range result.Values() {
			items = append(items, item)
		}
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
			clusterName := ""
			nodeRole := ""

			if server.Tags == nil {
				continue
			}
			for key := range server.Tags {
				if strings.Contains(key, "kubernetes.io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes.io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {
				instanceID := encodeMachineID(location, *server.Name)

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
			clusterName := ""
			nodeRole := ""

			if nic.Tags == nil {
				continue
			}
			for key := range nic.Tags {
				if strings.Contains(key, "kubernetes.io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes.io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {
				isNic, machineName := vmNameFromDependencyName(*nic.Name, nicSuffix)
				if !isNic {
					continue
				}

				instanceID := encodeMachineID(location, machineName)

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

	if disks != nil && len(disks) > 0 {
		for _, disk := range disks {
			clusterName := ""
			nodeRole := ""

			if disk.Tags == nil {
				continue
			}
			for key := range disk.Tags {
				if strings.Contains(key, "kubernetes.io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes.io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {
				isDisk, machineName := vmNameFromDependencyName(*disk.Name, diskSuffix)
				if !isDisk {
					continue
				}
				instanceID := encodeMachineID(location, machineName)

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

func (clients *azureDriverClients) deleteVMNicDisk(ctx context.Context, resourceGroupName string, VMName string, nicName string, diskName string) error {

	// We try to fetch the VM, detach its data disks and finally delete it
	if vm, vmErr := clients.vm.Get(ctx, resourceGroupName, VMName, ""); vmErr == nil {
		if dataDiskDetachmentErr := clients.waitForDataDiskDetachment(ctx, resourceGroupName, vm); dataDiskDetachmentErr == nil {
			if deleteErr := clients.deleteVM(ctx, resourceGroupName, VMName); deleteErr != nil {
				return deleteErr
			}
		} else {
			return dataDiskDetachmentErr
		}
		onARMAPISuccess(prometheusServiceVM, "VM Get was successful for %s", *vm.Name)
	} else if !notFound(vmErr) {
		// If some other error occurred, which is not 404 Not Found, because the VM doesn't exist, then bubble up
		return onARMAPIErrorFail(prometheusServiceVM, vmErr, "vm.Get")
	}

	// Fetch the NIC and deleted it
	nicDeleter := func() error {
		if vmHoldingNic, err := clients.fetchAttachedVMfromNIC(ctx, resourceGroupName, nicName); err != nil {
			if notFound(err) {
				return nil // Resource doesn't exist, no need to delete
			}
			return err
		} else if vmHoldingNic != "" {
			return fmt.Errorf("Cannot delete NIC %s because it is attached to VM %s", nicName, vmHoldingNic)
		}

		return clients.deleteNIC(ctx, resourceGroupName, nicName)
	}

	// Fetch the system disk and delete it
	diskDeleter := func() error {
		if vmHoldingDisk, err := clients.fetchAttachedVMfromDisk(ctx, resourceGroupName, diskName); err != nil {
			if notFound(err) {
				return nil // Resource doesn't exist, no need to delete
			}
			return err
		} else if vmHoldingDisk != "" {
			return fmt.Errorf("Cannot delete disk %s because it is attached to VM %s", diskName, vmHoldingDisk)
		}

		return clients.deleteDisk(ctx, resourceGroupName, diskName)
	}

	return runInParallel(nicDeleter, diskDeleter)
}

// waitForDataDiskDetachment waits for data disks to be detached
func (clients *azureDriverClients) waitForDataDiskDetachment(ctx context.Context, resourceGroupName string, vm compute.VirtualMachine) error {
	glog.V(2).Infof("Data disk detachment began for %q", *vm.Name)
	defer glog.V(2).Infof("Data disk detached for %q", *vm.Name)

	if len(*vm.StorageProfile.DataDisks) > 0 {
		// There are disks attached hence need to detach them
		vm.StorageProfile.DataDisks = &[]compute.DataDisk{}

		future, err := clients.vm.CreateOrUpdate(ctx, resourceGroupName, *vm.Name, vm)
		err = future.WaitForCompletionRef(ctx, clients.vm.Client)
		if err != nil {
			glog.Errorf("Failed to CreateOrUpdate. Error Message - %s", err)
			return onARMAPIErrorFail(prometheusServiceVM, err, "vm.CreateOrUpdate")
		}
		onARMAPISuccess(prometheusServiceVM, "VM CreateOrUpdate was successful for %s", *vm.Name)
	}

	return nil
}

func (clients *azureDriverClients) powerOffVM(ctx context.Context, resourceGroupName string, vmName string) error {
	glog.V(2).Infof("VM power-off began for %q", vmName)
	defer glog.V(2).Infof("VM power-off done for %q", vmName)

	future, err := clients.vm.PowerOff(ctx, resourceGroupName, vmName)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceVM, err, "vm.PowerOff")
	}
	err = future.WaitForCompletionRef(ctx, clients.vm.Client)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceVM, err, "vm.PowerOff")
	}
	onARMAPISuccess(prometheusServiceVM, "VM poweroff was successful for %s", vmName)
	return nil
}

func (clients *azureDriverClients) deleteVM(ctx context.Context, resourceGroupName string, vmName string) error {
	glog.V(2).Infof("VM deletion has began for %q", vmName)
	defer glog.V(2).Infof("VM deleted for %q", vmName)

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

func (clients *azureDriverClients) deleteNIC(ctx context.Context, resourceGroupName string, nicName string) error {
	glog.V(2).Infof("NIC delete started for %q", nicName)
	defer glog.V(2).Infof("NIC deleted for %q", nicName)

	future, err := clients.nic.Delete(ctx, resourceGroupName, nicName)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceNIC, err, "nic.Delete")
	}
	err = future.WaitForCompletion(ctx, clients.nic.Client)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceNIC, err, "nic.Delete")
	}
	onARMAPISuccess(prometheusServiceNIC, "NIC deletion was successful for %s", nicName)
	return nil
}

func (clients *azureDriverClients) deleteDisk(ctx context.Context, resourceGroupName string, diskName string) error {
	glog.V(2).Infof("System disk delete started for %q", diskName)
	defer glog.V(2).Infof("System disk deleted for %q", diskName)

	future, err := clients.disk.Delete(ctx, resourceGroupName, diskName)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceDisk, err, "disk.Delete")
	}
	err = future.WaitForCompletionRef(ctx, clients.disk.Client)
	if err != nil {
		return onARMAPIErrorFail(prometheusServiceDisk, err, "disk.Delete")
	}
	onARMAPISuccess(prometheusServiceDisk, "OS-Disk deletion was successful for %s", diskName)
	return nil
}

func onARMAPISuccess(prometheusService string, format string, v ...interface{}) {
	prometheusSuccess(prometheusService)
	glog.V(2).Infof(format, v...)
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
		requestID := strings.Join(detailedErr.Response.Header["X-Ms-Request-Id"], "")
		return true, requestID, &detailedErr
	default:
		return false, "", nil
	}
}

// onErrorFail prints a failure message and exits the program if err is not nil.
func onErrorFail(err error, format string, v ...interface{}) error {
	if err != nil {
		message := fmt.Sprintf(format, v...)
		if hasRequestID, requestID, detailedErr := retrieveRequestID(err); hasRequestID {
			glog.Errorf("Azure ARM API call with x-ms-request-id=%s failed. %s: %s\n", requestID, message, *detailedErr)
		} else {
			glog.Errorf("%s: %s\n", message, err)
		}
	}
	return err
}

func runInParallel(funcs ...(func() error)) error {
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
	nicSuffix  = "-nic"
	diskSuffix = "-os-disk"
)

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
	prometheusServiceDeployment = "deployment"
	prometheusServiceVM         = "virtual_machine"
	prometheusServiceNIC        = "network_interfaces"
	prometheusServiceDisk       = "disks"
)

func prometheusSuccess(service string) {
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "azure", "service": service}).Inc()
}

func prometheusFail(service string) {
	metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "azure", "service": service}).Inc()
}
