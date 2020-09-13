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

package driver

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Driver AWS", func() {

	Context("#generateTags", func() {

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

			tagsGenerated, err := awsDriver.generateTags(tags, resourceTypeInstance)
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

			tagsGenerated, err := awsDriver.generateTags(tags, resourceTypeInstance)
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

	Context("#generateBlockDevices", func() {

		It("should convert multiples blockDevices successfully", func() {
			awsDriver := &AWSDriver{}
			disks := []v1alpha1.AWSBlockDeviceMappingSpec{
				{
					DeviceName: "/root",
					Ebs: v1alpha1.AWSEbsBlockDeviceSpec{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           false,
						VolumeSize:          32,
						VolumeType:          "gp2",
					},
				},
				{
					DeviceName: "/dev/xvdg",
					Ebs: v1alpha1.AWSEbsBlockDeviceSpec{
						DeleteOnTermination: aws.Bool(false),
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
						DeleteOnTermination: aws.Bool(true),
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

		It("should not encrypt blockDevices by default", func() {
			awsDriver := &AWSDriver{}
			disks := []v1alpha1.AWSBlockDeviceMappingSpec{
				{
					Ebs: v1alpha1.AWSEbsBlockDeviceSpec{
						VolumeSize: 32,
						VolumeType: "gp2",
					},
				},
			}

			rootDevice := aws.String("/dev/sda")
			disksGenerated, err := awsDriver.generateBlockDevices(disks, rootDevice)
			expectedDisks := []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda"),
					Ebs: &ec2.EbsBlockDevice{
						Encrypted:  aws.Bool(false),
						VolumeSize: aws.Int64(32),
						Iops:       nil,
						VolumeType: aws.String("gp2"),
					},
				},
			}

			Expect(disksGenerated).To(Equal(expectedDisks))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("#GetVolNames", func() {
		var hostPathPVSpec = corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/mnt/data",
				},
			},
		}

		It("should handle in-tree PV (with .spec.awsElasticBlockStore)", func() {
			driver := &AWSDriver{}
			pvs := []corev1.PersistentVolumeSpec{
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						AWSElasticBlockStore: &corev1.AWSElasticBlockStoreVolumeSource{
							VolumeID: "aws://eu-west-1a/vol-1234",
						},
					},
				},
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						AWSElasticBlockStore: &corev1.AWSElasticBlockStoreVolumeSource{
							VolumeID: "aws:///vol-2345",
						},
					},
				},
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						AWSElasticBlockStore: &corev1.AWSElasticBlockStoreVolumeSource{
							VolumeID: "vol-3456",
						},
					},
				},
				hostPathPVSpec,
			}

			actual, err := driver.GetVolNames(pvs)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual).To(Equal([]string{"vol-1234", "vol-2345", "vol-3456"}))
		})

		It("should handle out-of-tree PV (with .spec.csi.volumeHandle)", func() {
			driver := &AWSDriver{}
			pvs := []corev1.PersistentVolumeSpec{
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "io.kubernetes.storage.mock",
							VolumeHandle: "vol-2345",
						},
					},
				},
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "ebs.csi.aws.com",
							VolumeHandle: "vol-1234",
						},
					},
				},
				hostPathPVSpec,
			}

			actual, err := driver.GetVolNames(pvs)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual).To(Equal([]string{"vol-1234"}))
		})
	})
})
