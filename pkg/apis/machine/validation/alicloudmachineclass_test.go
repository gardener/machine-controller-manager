package validation

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"strings"
)

func getAliCloudMachineSpec() *machine.AlicloudMachineClassSpec {
	bandwidth := 5
	deleteVol1 := true
	deleteVol2 := false
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
				DeleteWithInstance: &deleteVol1,
			},
			{
				Name:               "dd2",
				Category:           "cloud_efficiency",
				Size:               75,
				Encrypted:          true,
				DeleteWithInstance: &deleteVol2,
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
			longname := strings.Repeat("longname", 10)
			spec.DataDisks = []machine.AlicloudDataDisk{
				{
					Name:      "",
					Category:  "",
					Size:      10,
					Encrypted: true,
				},
				{
					Name:      "dd1",
					Category:  "cloud_efficiency",
					Size:      32769,
					Encrypted: true,
				},
				{
					Name:      "dd1",
					Category:  "cloud_efficiency",
					Size:      75,
					Encrypted: true,
				},
				{
					Name:      "bad$#%name",
					Category:  "cloud_efficiency",
					Size:      75,
					Encrypted: true,
				},
				{
					Name:      longname,
					Category:  "cloud_efficiency",
					Size:      75,
					Encrypted: true,
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
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.dataDisks[0].size",
					BadValue: 10,
					Detail:   "must be between 20 and 32768, inclusive",
				},
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.dataDisks[0].category",
					BadValue: "",
					Detail:   "Data Disk category is required",
				},
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.dataDisks[1].size",
					BadValue: 32769,
					Detail:   "must be between 20 and 32768, inclusive",
				},
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.dataDisks",
					BadValue: "dd1",
					Detail:   "Data Disk Name 'dd1' duplicated 2 times, Name must be unique",
				},
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.dataDisks[3].name",
					BadValue: "bad$#%name",
					Detail:   "Disk name given: bad$#%name does not match the expected pattern (regex used for validation is '[a-zA-Z0-9\\.\\-_:]+')",
				},
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.dataDisks[4].name",
					BadValue: longname,
					Detail:   "Data Disk Name length must be between 1 and 64",
				},
			}
			Expect(errList).To(ConsistOf(errExpected))
		})
	})
})
