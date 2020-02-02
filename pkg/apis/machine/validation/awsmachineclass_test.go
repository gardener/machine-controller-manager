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

// Package validation is used to validate all the machine CRD objects
package validation

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestAWSMachineClassSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSMachineClass Suite")
}

func getSpec() *machine.AWSMachineClassSpec {
	return &machine.AWSMachineClassSpec{
		AMI:          "ami-99fn8a892f94e765a",
		Region:       "eu-west-1",
		BlockDevices: []machine.AWSBlockDeviceMappingSpec{},
		EbsOptimized: true,
		IAM: machine.AWSIAMProfileSpec{
			Name: "iam-name",
		},
		MachineType: "m5.large",
		KeyName:     "keyname",
		Monitoring:  true,
		NetworkInterfaces: []machine.AWSNetworkInterfaceSpec{
			{
				SubnetID:         "subnet-d660fb99",
				SecurityGroupIDs: []string{"sg-0c3a49f760fe5cbfe", "sg-0c3a49f789898dwwdw", "sg-0c3a49f789898ddsdfe"},
			},
		},
		Tags: map[string]string{
			"kubernetes.io/cluster/env": "shared",
			"kubernetes.io/role/env":    "1",
		},
		SecretRef: &corev1.SecretReference{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
	}
}

var _ = Describe("AWSMachineClass Validation", func() {

	Context("Validate AWSMachineClass Spec", func() {

		It("should validate an object successfully", func() {

			spec := getSpec()
			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			Expect(err).To(Equal(field.ErrorList{}))
		})

		It("should get an error - fields required", func() {

			spec := getSpec()
			spec.AMI = ""
			spec.Region = ""
			spec.MachineType = ""
			spec.NetworkInterfaces = []machine.AWSNetworkInterfaceSpec{}
			spec.IAM = machine.AWSIAMProfileSpec{}
			spec.KeyName = ""
			spec.SecretRef = &corev1.SecretReference{}
			spec.Tags = map[string]string{}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     "FieldValueRequired",
					Field:    "spec.ami",
					BadValue: "",
					Detail:   "AMI is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.region",
					BadValue: "",
					Detail:   "Region is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.machineType",
					BadValue: "",
					Detail:   "MachineType is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.iam.name",
					BadValue: "",
					Detail:   "IAM Name is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.keyName",
					BadValue: "",
					Detail:   "KeyName is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.networkInterfaces[]",
					BadValue: "",
					Detail:   "Mention at least one NetworkInterface",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.secretRef.name",
					BadValue: "",
					Detail:   "name is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.secretRef.namespace",
					BadValue: "",
					Detail:   "namespace is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.tags.kubernetes.io/cluster/",
					BadValue: "",
					Detail:   "Tag required of the form kubernetes.io/cluster/****",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.tags.kubernetes.io/role/",
					BadValue: "",
					Detail:   "Tag required of the form kubernetes.io/role/****",
				},
			}

			Expect(err).To(Equal(errExpected))
		})

		It("should get an error on NetworkInterfaces validation", func() {
			spec := getSpec()
			spec.NetworkInterfaces = []machine.AWSNetworkInterfaceSpec{
				{
					SubnetID:         "",
					SecurityGroupIDs: []string{"", "sg-0c3a49f789898dwwdw", "sg-0c3a49f789898ddsdfe"},
				},
			}
			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     "FieldValueRequired",
					Field:    "spec.networkInterfaces.subnetID",
					BadValue: "",
					Detail:   "SubnetID is required",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.networkInterfaces.securityGroupIDs",
					BadValue: "",
					Detail:   "securityGroupIDs cannot be blank for networkInterface:0 securityGroupID:0",
				},
			}
			Expect(err).To(Equal(errExpected))
		})

		It("should get an error on BlockDevices validation - wrong ebs definition", func() {
			spec := getSpec()
			spec.BlockDevices = []machine.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/test",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 0,
						VolumeType: "",
					},
				},
			}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices[0].ebs.volumeType",
					BadValue: "",
					Detail:   "Please mention a valid ebs volume type:  gp2, io1, st1, sc1, or standard.",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices[0].ebs.volumeSize",
					BadValue: "",
					Detail:   "Please mention a valid ebs volume size",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices[0].ebs.volumeType",
					BadValue: "",
					Detail:   "Please mention a valid ebs volume type",
				},
			}
			Expect(err).To(Equal(errExpected))
		})

		It("should get an error on BlockDevices validation - wrong iops ebs definition", func() {
			spec := getSpec()
			spec.BlockDevices = []machine.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/test",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 0,
						VolumeType: "io1",
						Iops:       0,
					},
				},
			}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices[0].ebs.volumeSize",
					BadValue: "",
					Detail:   "Please mention a valid ebs volume size",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices[0].ebs.iops",
					BadValue: "",
					Detail:   "Please mention a valid ebs volume iops",
				},
			}
			Expect(err).To(Equal(errExpected))
		})

		It("should get an error on BlockDevices validation - duplicated root device name", func() {
			spec := getSpec()
			spec.BlockDevices = []machine.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/root",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 10,
						VolumeType: "gp2",
					},
				},
				{
					DeviceName: "/root",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 15,
						VolumeType: "gp2",
					},
				},
			}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices",
					BadValue: "",
					Detail:   "Only one '/root' partition can be specified",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices",
					BadValue: "",
					Detail:   "Device name '/root' duplicated 2 times, DeviceName must be uniq",
				},
			}
			Expect(err).To(Equal(errExpected))
		})

		It("should get an error on BlockDevices validation - duplicated device name", func() {
			spec := getSpec()
			spec.BlockDevices = []machine.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/dev/vdga",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 10,
						VolumeType: "gp2",
					},
				},
				{
					DeviceName: "/dev/vdga",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 15,
						VolumeType: "gp2",
					},
				},
			}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices",
					BadValue: "",
					Detail:   "Device name with '/root' partition must be specified",
				},
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices",
					BadValue: "",
					Detail:   "Device name '/dev/vdga' duplicated 2 times, DeviceName must be uniq",
				},
			}
			Expect(err).To(Equal(errExpected))
		})

		It("should get an error on BlockDevices validation - invalid volumeType", func() {
			spec := getSpec()
			spec.BlockDevices = []machine.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/dev/vdga",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 10,
						VolumeType: "gp2-invalid",
					},
				},
			}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			errExpected := field.ErrorList{
				{
					Type:     "FieldValueRequired",
					Field:    "spec.blockDevices[0].ebs.volumeType",
					BadValue: "",
					Detail:   "Please mention a valid ebs volume type:  gp2, io1, st1, sc1, or standard.",
				},
			}
			Expect(err).To(Equal(errExpected))
		})

		It("should validate an object successfully - with one EBS without root device name", func() {
			spec := getSpec()
			spec.BlockDevices = []machine.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 10,
						VolumeType: "gp2",
					},
				},
			}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			Expect(err).To(Equal(field.ErrorList{}))
		})

		It("should validate an object successfully - with multiples EBS", func() {
			spec := getSpec()
			spec.BlockDevices = []machine.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/root",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 10,
						VolumeType: "gp2",
					},
				},
				{
					DeviceName: "/dev/vdga",
					Ebs: machine.AWSEbsBlockDeviceSpec{
						VolumeSize: 15,
						VolumeType: "gp2",
					},
				},
			}

			err := validateAWSMachineClassSpec(spec, field.NewPath("spec"))

			Expect(err).To(Equal(field.ErrorList{}))
		})

	})

})
