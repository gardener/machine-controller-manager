/*
Copyright 2018 The Kubernetes Authors.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/********************** AzureMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureMachineClass TODO
type AzureMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec AzureMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureMachineClassList is a collection of AzureMachineClasses.
type AzureMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []AzureMachineClass `json:"items"`
}

// AzureMachineClassSpec is the specification of a cluster.
type AzureMachineClassSpec struct {
	Location      string                        `json:"location,omitempty"`
	Tags          map[string]string             `json:"tags,omitempty"`
	Properties    AzureVirtualMachineProperties `json:"properties,omitempty"`
	ResourceGroup string                        `json:"resourceGroup,omitempty"`
	SubnetInfo    AzureSubnetInfo               `json:"subnetInfo,omitempty"`
	SecretRef     *corev1.SecretReference       `json:"secretRef,omitempty"`
}

// AzureVirtualMachineProperties is describes the properties of a Virtual Machine.
type AzureVirtualMachineProperties struct {
	HardwareProfile AzureHardwareProfile `json:"hardwareProfile,omitempty"`
	StorageProfile  AzureStorageProfile  `json:"storageProfile,omitempty"`
	OsProfile       AzureOSProfile       `json:"osProfile,omitempty"`
	NetworkProfile  AzureNetworkProfile  `json:"networkProfile,omitempty"`
	AvailabilitySet AzureSubResource     `json:"availabilitySet,omitempty"`
}

// AzureHardwareProfile is specifies the hardware settings for the virtual machine.
// Refer github.com/Azure/azure-sdk-for-go/arm/compute/models.go for VMSizes
type AzureHardwareProfile struct {
	VMSize string `json:"vmSize,omitempty"`
}

// AzureStorageProfile is specifies the storage settings for the virtual machine disks.
type AzureStorageProfile struct {
	ImageReference AzureImageReference `json:"imageReference,omitempty"`
	OsDisk         AzureOSDisk         `json:"osDisk,omitempty"`
}

// AzureImageReference is specifies information about the image to use. You can specify information about platform images,
// marketplace images, or virtual machine images. This element is required when you want to use a platform image,
// marketplace image, or virtual machine image, but is not used in other creation operations.
type AzureImageReference struct {
	ID        string `json:"id,omitempty"`
	Publisher string `json:"publisher,omitempty"`
	Offer     string `json:"offer,omitempty"`
	Sku       string `json:"sku,omitempty"`
	Version   string `json:"version,omitempty"`
}

// AzureOSDisk is specifies information about the operating system disk used by the virtual machine. <br><br> For more
// information about disks, see [About disks and VHDs for Azure virtual
// machines](https://docs.microsoft.com/azure/virtual-machines/virtual-machines-windows-about-disks-vhds?toc=%2fazure%2fvirtual-machines%2fwindows%2ftoc.json).
type AzureOSDisk struct {
	Name         string                     `json:"name,omitempty"`
	Caching      string                     `json:"caching,omitempty"`
	ManagedDisk  AzureManagedDiskParameters `json:"managedDisk,omitempty"`
	DiskSizeGB   int32                      `json:"diskSizeGB,omitempty"`
	CreateOption string                     `json:"createOption,omitempty"`
}

// AzureManagedDiskParameters is the parameters of a managed disk.
type AzureManagedDiskParameters struct {
	ID                 string `json:"id,omitempty"`
	StorageAccountType string `json:"storageAccountType,omitempty"`
}

// AzureOSProfile is specifies the operating system settings for the virtual machine.
type AzureOSProfile struct {
	ComputerName       string                  `json:"computerName,omitempty"`
	AdminUsername      string                  `json:"adminUsername,omitempty"`
	AdminPassword      string                  `json:"adminPassword,omitempty"`
	CustomData         string                  `json:"customData,omitempty"`
	LinuxConfiguration AzureLinuxConfiguration `json:"linuxConfiguration,omitempty"`
}

// AzureLinuxConfiguration is specifies the Linux operating system settings on the virtual machine. <br><br>For a list of
// supported Linux distributions, see [Linux on Azure-Endorsed
// Distributions](https://docs.microsoft.com/azure/virtual-machines/virtual-machines-linux-endorsed-distros?toc=%2fazure%2fvirtual-machines%2flinux%2ftoc.json)
// <br><br> For running non-endorsed distributions, see [Information for Non-Endorsed
// Distributions](https://docs.microsoft.com/azure/virtual-machines/virtual-machines-linux-create-upload-generic?toc=%2fazure%2fvirtual-machines%2flinux%2ftoc.json).
type AzureLinuxConfiguration struct {
	DisablePasswordAuthentication bool                  `json:"disablePasswordAuthentication,omitempty"`
	SSH                           AzureSSHConfiguration `json:"ssh,omitempty"`
}

// AzureSSHConfiguration is SSH configuration for Linux based VMs running on Azure
type AzureSSHConfiguration struct {
	PublicKeys AzureSSHPublicKey `json:"publicKeys,omitempty"`
}

// AzureSSHPublicKey is contains information about SSH certificate public key and the path on the Linux VM where the public
// key is placed.
type AzureSSHPublicKey struct {
	Path    string `json:"path,omitempty"`
	KeyData string `json:"keyData,omitempty"`
}

// AzureNetworkProfile is specifies the network interfaces of the virtual machine.
type AzureNetworkProfile struct {
	NetworkInterfaces AzureNetworkInterfaceReference `json:"networkInterfaces,omitempty"`
}

// AzureNetworkInterfaceReference is describes a network interface reference.
type AzureNetworkInterfaceReference struct {
	ID                                        string `json:"id,omitempty"`
	*AzureNetworkInterfaceReferenceProperties `json:"properties,omitempty"`
}

// AzureNetworkInterfaceReferenceProperties is describes a network interface reference properties.
type AzureNetworkInterfaceReferenceProperties struct {
	Primary bool `json:"primary,omitempty"`
}

// AzureSubResource is the Sub Resource definition.
type AzureSubResource struct {
	ID string `json:"id,omitempty"`
}

// AzureSubnetInfo is the information containing the subnet details
type AzureSubnetInfo struct {
	VnetName   string `json:"vnetName,omitempty"`
	SubnetName string `json:"subnetName,omitempty"`
}

const (
	// AzureClientID is a constant for a key name that is part of the Azure cloud credentials.
	AzureClientID string = "azureClientId"
	// AzureClientSecret is a constant for a key name that is part of the Azure cloud credentials.
	AzureClientSecret string = "azureClientSecret"
	// AzureSubscriptionID is a constant for a key name that is part of the Azure cloud credentials.
	AzureSubscriptionID string = "azureSubscriptionId"
	// AzureTenantID is a constant for a key name that is part of the Azure cloud credentials.
	AzureTenantID string = "azureTenantId"
)
