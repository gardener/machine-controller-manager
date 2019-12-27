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
package driver

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestAWSDriverSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Driver AWS Suite")
}

var _ = Describe("Driver AWS", func() {

	Context("GenerateSecurityGroups Driver AWS Spec", func() {

		It("should convert multiples security groups successfully", func() {
			awsDriver := &AWSDriver{}
			sgs := []string{"sg-0c3a49f760fe5cbfe", "sg-0c3a49f789898dwwdw", "sg-0c3a49f789898ddsdfe"}

			sg, err := awsDriver.generateSecurityGroups(sgs)
			expectedSg := []*string{aws.String("sg-0c3a49f760fe5cbfe"), aws.String("sg-0c3a49f789898dwwdw"), aws.String("sg-0c3a49f789898ddsdfe")}
			Expect(sg).To(Equal(expectedSg))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should convert zero security groups successfully", func() {
			awsDriver := &AWSDriver{}
			sgs := []string{}

			sg, err := awsDriver.generateSecurityGroups(sgs)

			var expectedSg []*string
			Expect(sg).To(Equal(expectedSg))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("GenerateTags Driver AWS Spec", func() {

		It("should convert multiples tags successfully", func() {
			awsDriver := &AWSDriver{
				AWSMachineClass: &v1alpha1.AWSMachineClass{},
				CloudConfig:     &corev1.Secret{},
				UserData:        "dXNlciBkYXRhCg==",
				MachineID:       "ami-99fn8a892f94e765a",
				MachineName:     "machine-name",
			}
			tags := map[string]string{
				"tag-1": "value-tag-1",
				"tag-2": "value-tag-2",
				"tag-3": "value-tag-3",
			}

			tagsGenerated, err := awsDriver.generateTags(tags)
			expectedTags := &ec2.TagSpecification{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("tag-1"),
						Value: aws.String("value-tag-1"),
					},
					{
						Key:   aws.String("tag-2"),
						Value: aws.String("value-tag-2"),
					},
					{
						Key:   aws.String("tag-3"),
						Value: aws.String("value-tag-3"),
					},
					{
						Key:   aws.String("Name"),
						Value: aws.String("machine-name"),
					},
				},
			}

			Expect(tagsGenerated.ResourceType).To(Equal(expectedTags.ResourceType))
			Expect(tagsGenerated.Tags).To(ConsistOf(expectedTags.Tags))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should convert zero tags successfully", func() {
			awsDriver := &AWSDriver{}
			tags := map[string]string{}

			tagsGenerated, err := awsDriver.generateTags(tags)
			expectedTags := &ec2.TagSpecification{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(""),
					},
				},
			}

			Expect(tagsGenerated).To(Equal(expectedTags))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("GenerateBlockDevices Driver AWS Spec", func() {

		It("should convert multiples blockDevices successfully", func() {
			awsDriver := &AWSDriver{}
			disks := []v1alpha1.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/root",
					Ebs: v1alpha1.AWSEbsBlockDeviceSpec{
						DeleteOnTermination: true,
						Encrypted:           false,
						VolumeSize:          32,
						VolumeType:          "gp2",
					},
				},
				{
					DeviceName: "/dev/xvdg",
					Ebs: v1alpha1.AWSEbsBlockDeviceSpec{
						DeleteOnTermination: false,
						Encrypted:           true,
						Iops:                100,
						VolumeSize:          64,
						VolumeType:          "io1",
					},
				},
			}

			rootDevice := aws.String("/dev/sda")
			disksGenerated, err := awsDriver.generateBlockDevices(disks, rootDevice)
			expectedDisks := []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(false),
						VolumeSize:          aws.Int64(32),
						Iops:                nil,
						VolumeType:          aws.String("gp2"),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdg"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(false),
						Encrypted:           aws.Bool(true),
						VolumeSize:          aws.Int64(64),
						Iops:                aws.Int64(100),
						VolumeType:          aws.String("io1"),
					},
				},
			}

			Expect(disksGenerated).To(Equal(expectedDisks))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should convert single blockDevices without deviceName successfully", func() {
			awsDriver := &AWSDriver{}
			disks := []v1alpha1.AWSBlockDeviceMappingSpec{
				{
					Ebs: v1alpha1.AWSEbsBlockDeviceSpec{
						DeleteOnTermination: true,
						Encrypted:           false,
						VolumeSize:          32,
						VolumeType:          "gp2",
					},
				},
			}

			rootDevice := aws.String("/dev/sda")
			disksGenerated, err := awsDriver.generateBlockDevices(disks, rootDevice)
			expectedDisks := []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda"),
					Ebs: &ec2.EbsBlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(false),
						VolumeSize:          aws.Int64(32),
						Iops:                nil,
						VolumeType:          aws.String("gp2"),
					},
				},
			}

			Expect(disksGenerated).To(Equal(expectedDisks))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should convert zero blockDevices successfully", func() {
			awsDriver := &AWSDriver{}
			disks := []v1alpha1.AWSBlockDeviceMappingSpec{}

			rootDevice := aws.String("/dev/sda")
			disksGenerated, err := awsDriver.generateBlockDevices(disks, rootDevice)
			var expectedDisks []*ec2.BlockDeviceMapping

			Expect(disksGenerated).To(Equal(expectedDisks))
			Expect(err).ToNot(HaveOccurred())
		})

	})
})
