package validation

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func getAzureMachineSpec() *machine.AzureMachineClassSpec {
	urn := "CoreOS:CoreOS:Stable:2303.3.0"
	zone := 1
	lun0 := int32(0)
	lun1 := int32(1)
	return &machine.AzureMachineClassSpec{
		Location: "westeurope",
		Properties: machine.AzureVirtualMachineProperties{
			HardwareProfile: machine.AzureHardwareProfile{
				VMSize: "Standard_A1_v2",
			},
			OsProfile: machine.AzureOSProfile{
				AdminUsername: "core",
				LinuxConfiguration: machine.AzureLinuxConfiguration{
					DisablePasswordAuthentication: true,
					SSH: machine.AzureSSHConfiguration{
						PublicKeys: machine.AzureSSHPublicKey{
							KeyData: "this-is-a-key",
							Path:    "/home/core/.ssh/authorized_keys",
						},
					},
				},
			},
			StorageProfile: machine.AzureStorageProfile{
				ImageReference: machine.AzureImageReference{
					URN: &urn,
				},
				OsDisk: machine.AzureOSDisk{
					Caching:      "None",
					CreateOption: "FromImage",
					DiskSizeGB:   75,
					ManagedDisk: machine.AzureManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
				},
				DataDisks: []machine.AzureDataDisk{
					{
						Lun:                &lun0,
						Caching:            "None",
						DiskSizeGB:         75,
						StorageAccountType: "Standard_LRS",
					},
					{
						Lun:                &lun1,
						Caching:            "None",
						DiskSizeGB:         75,
						StorageAccountType: "Standard_LRS",
					},
				},
			},
			Zone: &zone,
		},
		ResourceGroup: "resourcegroup",
		SubnetInfo: machine.AzureSubnetInfo{
			SubnetName: "subnetnamne", VnetName: "vnetname",
		},
		Tags: map[string]string{
			"kubernetes.io-cluster-shoot": "1",
			"kubernetes.io-role-node":     "1",
		},
		SecretRef: &corev1.SecretReference{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
	}
}

var _ = Describe("AzureMachineClass Validation", func() {

	Context("Validate AzureMachineClass Spec", func() {

		It("should validate an object successfully", func() {

			spec := getAzureMachineSpec()
			err := validateAzureMachineClassSpec(spec, field.NewPath("spec"))

			Expect(err).To(Equal(field.ErrorList{}))
		})

		It("should get an error on DataDisks validation", func() {
			spec := getAzureMachineSpec()
			spec.Properties.StorageProfile.DataDisks = []machine.AzureDataDisk{
				{
					Name: "sdb",
				},
			}

			errList := validateAzureMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.properties.storageProfile.dataDisks[0].lun",
					BadValue: "",
					Detail:   "DataDisk Lun is required",
				},
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.properties.storageProfile.dataDisks[0].diskSizeGB",
					BadValue: "",
					Detail:   "DataDisk size must be positive",
				},
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.properties.storageProfile.dataDisks[0].storageAccountType",
					BadValue: "",
					Detail:   "DataDisk storage account type is required",
				},
			}
			Expect(errList).To(ConsistOf(errExpected))

		})

		It("should get an error on duplicated DataDisks lun", func() {
			spec := getAzureMachineSpec()
			lun1 := int32(1)
			spec.Properties.StorageProfile.DataDisks = []machine.AzureDataDisk{
				{
					Lun:                &lun1,
					Caching:            "None",
					DiskSizeGB:         75,
					StorageAccountType: "Standard_LRS",
				},
				{
					Lun:                &lun1,
					Caching:            "None",
					DiskSizeGB:         75,
					StorageAccountType: "Standard_LRS",
				},
			}

			errList := validateAzureMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.properties.storageProfile.dataDisks",
					BadValue: int32(1),
					Detail:   "Data Disk Lun '1' duplicated 2 times, Lun must be unique",
				},
			}
			Expect(errList).To(ConsistOf(errExpected))
		})

	})
})
