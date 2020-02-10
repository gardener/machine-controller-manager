package validation

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func getAliCloudMachineSpec() *machine.AlicloudMachineClassSpec {
	bandwidth := 5
	return &machine.AlicloudMachineClassSpec{
		ImageID:         "coreos_1745_7_0_64_30G_alibase_20180705.vhd",
		InstanceType:    "ecs.n1.medium",
		Region:          "cn-hangzhou",
		ZoneID:          "cn-hangzhou-e",
		SecurityGroupID: "sg-1234567890",
		VSwitchID:       "vsw-1234567890",
		SystemDisk: &machine.AlicloudSystemDisk{
			Category: "cloud_efficiency",
			Size:     75,
		},
		DataDisks: []machine.AlicloudDataDisk{
			{
				Name:               "dd1",
				Category:           "cloud_efficiency",
				Size:               75,
				Encrypted:          true,
				DeleteWithInstance: true,
			},
			{
				Name:               "dd2",
				Category:           "cloud_efficiency",
				Size:               75,
				Encrypted:          true,
				DeleteWithInstance: true,
			},
		},
		InstanceChargeType:      "PostPaid",
		InternetChargeType:      "PayByTraffic",
		InternetMaxBandwidthIn:  &bandwidth,
		InternetMaxBandwidthOut: &bandwidth,
		SpotStrategy:            "NoSpot",
		KeyPairName:             "my-ssh-key",
		Tags: map[string]string{
			"kubernetes.io/cluster/shoot": "1",
			"kubernetes.io/role/role":     "1",
		},
		SecretRef: &corev1.SecretReference{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
	}
}

var _ = Describe("AliCloudMachineClass Validation", func() {

	Context("Validate AliCloudMachineClass Spec", func() {

		It("should validate an object successfully", func() {

			spec := getAliCloudMachineSpec()
			err := validateAlicloudMachineClassSpec(spec, field.NewPath("spec"))

			Expect(err).To(Equal(field.ErrorList{}))
		})

		It("should get an error on DataDisks validation", func() {
			spec := getAliCloudMachineSpec()
			spec.DataDisks = []machine.AlicloudDataDisk{
				{
					Name:               "",
					Category:           "",
					Size:               -1,
					Encrypted:          true,
					DeleteWithInstance: true,
				},
				{
					Name:               "dd1",
					Category:           "cloud_efficiency",
					Size:               75,
					Encrypted:          true,
					DeleteWithInstance: true,
				},
				{
					Name:               "dd1",
					Category:           "cloud_efficiency",
					Size:               75,
					Encrypted:          true,
					DeleteWithInstance: true,
				},
			}

			errList := validateAlicloudMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.dataDisks[0].name",
					BadValue: "",
					Detail:   "Data Disk name is required",
				},
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.dataDisks[0].size",
					BadValue: "",
					Detail:   "Data Disk size must be positive",
				},
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.dataDisks[0].category",
					BadValue: "",
					Detail:   "Data Disk category is required",
				},
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.dataDisks",
					BadValue: "dd1",
					Detail:   "Data Disk Name 'dd1' duplicated 2 times, Name must be unique",
				},
			}
			Expect(errList).To(ConsistOf(errExpected))
		})
	})
})
