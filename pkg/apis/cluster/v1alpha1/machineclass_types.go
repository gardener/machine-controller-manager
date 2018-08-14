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
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineClass can be used to templatize and re-use provider configuration
// across multiple Machines / MachineSets / MachineDeployments.
// +k8s:openapi-gen=true
// +resource:path=machineclasses,strategy=MachineClassStrategy
type MachineClass struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The total capacity available on this machine type (cpu/memory/disk).
	//
	// WARNING: It is up to the creator of the MachineClass to ensure that
	// this field is consistent with the underlying machine that will
	// be provisioned when this class is used, to inform higher level
	// automation (e.g. the cluster autoscaler).
	Capacity corev1.ResourceList `json:"capacity"`

	// How much capacity is actually allocatable on this machine.
	// Must be equal to or less than the capacity, and when less
	// indicates the resources reserved for system overhead.
	//
	// WARNING: It is up to the creator of the MachineClass to ensure that
	// this field is consistent with the underlying machine that will
	// be provisioned when this class is used, to inform higher level
	// automation (e.g. the cluster autoscaler).
	Allocatable corev1.ResourceList `json:"allocatable"`

	// Provider-specific configuration to use during node creation.
	ProviderConfig runtime.RawExtension `json:"providerConfig"`

	// TODO: should this use an api.ObjectReference to a 'MachineTemplate' instead?
	// A link to the MachineTemplate that will be used to create provider
	// specific configuration for Machines of this class.
	// MachineTemplate corev1.ObjectReference `json:machineTemplate`

	SecretRef *corev1.SecretReference
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []MachineClass `json:"items"`
}

/********************** OpenStackMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenStackMachineClass TODO
type OpenStackMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec OpenStackMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenStackMachineClassList is a collection of OpenStackMachineClasses.
type OpenStackMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []OpenStackMachineClass `json:"items"`
}

// OpenStackMachineClassSpec is the specification of a cluster.
type OpenStackMachineClassSpec struct {
	ImageName        string                  `json:"imageName"`
	Region           string                  `json:"region"`
	AvailabilityZone string                  `json:"availabilityZone"`
	FlavorName       string                  `json:"flavorName"`
	KeyName          string                  `json:"keyName"`
	SecurityGroups   []string                `json:"securityGroups"`
	Tags             map[string]string       `json:"tags,omitempty"`
	NetworkID        string                  `json:"networkID"`
	SecretRef        *corev1.SecretReference `json:"secretRef,omitempty"`
}

/********************** AWSMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AWSMachineClass TODO
type AWSMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec AWSMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AWSMachineClassList is a collection of AWSMachineClasses.
type AWSMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []AWSMachineClass `json:"items"`
}

// AWSMachineClassSpec is the specification of a cluster.
type AWSMachineClassSpec struct {
	AMI               string                      `json:"ami,omitempty"`
	Region            string                      `json:"region,omitempty"`
	BlockDevices      []AWSBlockDeviceMappingSpec `json:"blockDevices,omitempty"`
	EbsOptimized      bool                        `json:"ebsOptimized,omitempty"`
	IAM               AWSIAMProfileSpec           `json:"iam,omitempty"`
	MachineType       string                      `json:"machineType,omitempty"`
	KeyName           string                      `json:"keyName,omitempty"`
	Monitoring        bool                        `json:"monitoring,omitempty"`
	NetworkInterfaces []AWSNetworkInterfaceSpec   `json:"networkInterfaces,omitempty"`
	Tags              map[string]string           `json:"tags,omitempty"`
	SecretRef         *corev1.SecretReference     `json:"secretRef,omitempty"`

	// TODO add more here
}

type AWSBlockDeviceMappingSpec struct {

	// The device name exposed to the machine (for example, /dev/sdh or xvdh).
	DeviceName string `json:"deviceName,omitempty"`

	// Parameters used to automatically set up EBS volumes when the machine is
	// launched.
	Ebs AWSEbsBlockDeviceSpec `json:"ebs,omitempty"`

	// Suppresses the specified device included in the block device mapping of the
	// AMI.
	NoDevice string `json:"noDevice,omitempty"`

	// The virtual device name (ephemeralN). Machine store volumes are numbered
	// starting from 0. An machine type with 2 available machine store volumes
	// can specify mappings for ephemeral0 and ephemeral1.The number of available
	// machine store volumes depends on the machine type. After you connect to
	// the machine, you must mount the volume.
	//
	// Constraints: For M3 machines, you must specify machine store volumes in
	// the block device mapping for the machine. When you launch an M3 machine,
	// we ignore any machine store volumes specified in the block device mapping
	// for the AMI.
	VirtualName string `json:"virtualName,omitempty"`
}

// Describes a block device for an EBS volume.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/EbsBlockDevice
type AWSEbsBlockDeviceSpec struct {

	// Indicates whether the EBS volume is deleted on machine termination.
	DeleteOnTermination bool `json:"deleteOnTermination,omitempty"`

	// Indicates whether the EBS volume is encrypted. Encrypted Amazon EBS volumes
	// may only be attached to machines that support Amazon EBS encryption.
	Encrypted bool `json:"encrypted,omitempty"`

	// The number of I/O operations per second (IOPS) that the volume supports.
	// For io1, this represents the number of IOPS that are provisioned for the
	// volume. For gp2, this represents the baseline performance of the volume and
	// the rate at which the volume accumulates I/O credits for bursting. For more
	// information about General Purpose SSD baseline performance, I/O credits,
	// and bursting, see Amazon EBS Volume Types (http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	//
	// Constraint: Range is 100-20000 IOPS for io1 volumes and 100-10000 IOPS for
	// gp2 volumes.
	//
	// Condition: This parameter is required for requests to create io1 volumes;
	// it is not used in requests to create gp2, st1, sc1, or standard volumes.
	Iops int64 `json:"iops,omitempty"`

	// The size of the volume, in GiB.
	//
	// Constraints: 1-16384 for General Purpose SSD (gp2), 4-16384 for Provisioned
	// IOPS SSD (io1), 500-16384 for Throughput Optimized HDD (st1), 500-16384 for
	// Cold HDD (sc1), and 1-1024 for Magnetic (standard) volumes. If you specify
	// a snapshot, the volume size must be equal to or larger than the snapshot
	// size.
	//
	// Default: If you're creating the volume from a snapshot and don't specify
	// a volume size, the default is the snapshot size.
	VolumeSize int64 `json:"volumeSize,omitempty"`

	// The volume type: gp2, io1, st1, sc1, or standard.
	//
	// Default: standard
	VolumeType string `json:"volumeType,omitempty"`
}

// Describes an IAM machine profile.
type AWSIAMProfileSpec struct {
	// The Amazon Resource Name (ARN) of the machine profile.
	ARN string `json:"arn,omitempty"`

	// The name of the machine profile.
	Name string `json:"name,omitempty"`
}

// Describes a network interface.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/MachineAWSNetworkInterfaceSpecification
type AWSNetworkInterfaceSpec struct {

	// Indicates whether to assign a public IPv4 address to an machine you launch
	// in a VPC. The public IP address can only be assigned to a network interface
	// for eth0, and can only be assigned to a new network interface, not an existing
	// one. You cannot specify more than one network interface in the request. If
	// launching into a default subnet, the default value is true.
	AssociatePublicIPAddress bool `json:"associatePublicIPAddress,omitempty"`

	// If set to true, the interface is deleted when the machine is terminated.
	// You can specify true only if creating a new network interface when launching
	// an machine.
	DeleteOnTermination bool `json:"deleteOnTermination,omitempty"`

	// The description of the network interface. Applies only if creating a network
	// interface when launching an machine.
	Description string `json:"description,omitempty"`

	// The IDs of the security groups for the network interface. Applies only if
	// creating a network interface when launching an machine.
	SecurityGroupIDs []string `json:"securityGroupIDs,omitempty"`

	// The ID of the subnet associated with the network string. Applies only if
	// creating a network interface when launching an machine.
	SubnetID string `json:"subnetID,omitempty"`
}

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

/********************** GCPMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCPMachineClass TODO
type GCPMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec GCPMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCPMachineClassList is a collection of GCPMachineClasses.
type GCPMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []GCPMachineClass `json:"items"`
}

// GCPMachineClassSpec is the specification of a cluster.
type GCPMachineClassSpec struct {
	CanIpForward       bool                    `json:"canIpForward"`
	DeletionProtection bool                    `json:"deletionProtection"`
	Description        *string                 `json:"description,omitempty"`
	Disks              []*GCPDisk              `json:"disks,omitempty"`
	Labels             map[string]string       `json:"labels,omitempty"`
	MachineType        string                  `json:"machineType"`
	Metadata           []*GCPMetadata          `json:"metadata,omitempty"`
	NetworkInterfaces  []*GCPNetworkInterface  `json:"networkInterfaces,omitempty"`
	Scheduling         GCPScheduling           `json:"scheduling"`
	SecretRef          *corev1.SecretReference `json:"secretRef,omitempty"`
	ServiceAccounts    []GCPServiceAccount     `json:"serviceAccounts"`
	Tags               []string                `json:"tags,omitempty"`
	Region             string                  `json:"region"`
	Zone               string                  `json:"zone"`
}

// GCPDisk describes disks for GCP.
type GCPDisk struct {
	AutoDelete bool              `json:"autoDelete"`
	Boot       bool              `json:"boot"`
	SizeGb     int64             `json:"sizeGb"`
	Type       string            `json:"type"`
	Image      string            `json:"image"`
	Labels     map[string]string `json:"labels"`
}

// GCPMetadata describes metadata for GCP.
type GCPMetadata struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// GCPNetworkInterface describes network interfaces for GCP
type GCPNetworkInterface struct {
	Network    string `json:"network,omitempty"`
	Subnetwork string `json:"subnetwork,omitempty"`
}

// GCPScheduling describes scheduling configuration for GCP.
type GCPScheduling struct {
	AutomaticRestart  bool   `json:"automaticRestart"`
	OnHostMaintenance string `json:"onHostMaintenance"`
	Preemptible       bool   `json:"preemptible"`
}

// GCPServiceAccount describes service accounts for GCP.
type GCPServiceAccount struct {
	Email  string   `json:"email"`
	Scopes []string `json:"scopes"`
}

const (
	// AWSAccessKeyID is a constant for a key name that is part of the AWS cloud credentials.
	AWSAccessKeyID string = "providerAccessKeyId"
	// AWSSecretAccessKey is a constant for a key name that is part of the AWS cloud credentials.
	AWSSecretAccessKey string = "providerSecretAccessKey"

	// AzureClientID is a constant for a key name that is part of the Azure cloud credentials.
	AzureClientID string = "azureClientId"
	// AzureClientSecret is a constant for a key name that is part of the Azure cloud credentials.
	AzureClientSecret string = "azureClientSecret"
	// AzureSubscriptionID is a constant for a key name that is part of the Azure cloud credentials.
	AzureSubscriptionID string = "azureSubscriptionId"
	// AzureTenantID is a constant for a key name that is part of the Azure cloud credentials.
	AzureTenantID string = "azureTenantId"

	// GCPServiceAccountJSON is a constant for a key name that is part of the GCP cloud credentials.
	GCPServiceAccountJSON string = "serviceAccountJSON"

	// OpenStackAuthURL is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackAuthURL string = "authURL"
	// OpenStackCACert is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackCACert string = "caCert"
	// OpenStackInsecure is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackInsecure string = "insecure"
	// OpenStackDomainName is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackDomainName string = "domainName"
	// OpenStackTenantName is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackTenantName string = "tenantName"
	// OpenStackUsername is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackUsername string = "username"
	// OpenStackPassword is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackPassword string = "password"
	// OpenStackClientCert is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackClientCert string = "clientCert"
	// OpenStackClientKey is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackClientKey string = "clientKey"
)
