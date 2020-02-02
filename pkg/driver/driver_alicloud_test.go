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
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strconv"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
)

var _ = Describe("Driver AliCloud", func() {
	Context("Generate Instance Tags", func() {

		It("Should maintain order of cluster and worker tags", func() {
			tags := map[string]string{
				"kubernetes.io/cluster/ali-test": "1",
				"kubernetes.io/role/worker":      "1",
				"taga":                           "tagvala",
				"tagb":                           "tagvalb",
				"tagc":                           "tagvalc",
			}
			c := &AlicloudDriver{}
			res, err := c.toInstanceTags(tags)
			expected := []ecs.RunInstancesTag{
				{
					Key:   "kubernetes.io/cluster/ali-test",
					Value: "1",
				},
				{
					Key:   "kubernetes.io/role/worker",
					Value: "1",
				},
				{
					Key:   "taga",
					Value: "tagvala",
				},
				{
					Key:   "tagb",
					Value: "tagvalb",
				},
				{
					Key:   "tagc",
					Value: "tagvalc",
				},
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(res[0:2]).To(Equal(expected[0:2]))
			Expect(res[2:5]).To(ConsistOf(expected[2:5]))
		})

		It("Should fail if no cluster tags", func() {
			tags := map[string]string{
				"kubernetes.io/role/worker": "1",
				"taga":                      "tagvala",
				"tagb":                      "tagvalb",
				"tagc":                      "tagvalc",
			}
			c := &AlicloudDriver{}
			_, err := c.toInstanceTags(tags)
			Expect(err).To(HaveOccurred())
		})

		It("should order cluster and worker tags", func() {
			tags := map[string]string{
				"taga":                           "tagvala",
				"tagb":                           "tagvalb",
				"kubernetes.io/cluster/ali-test": "1",
				"kubernetes.io/role/worker":      "1",
				"tagc":                           "tagvalc",
			}
			c := &AlicloudDriver{}
			res, err := c.toInstanceTags(tags)

			expected := []ecs.RunInstancesTag{
				{
					Key:   "kubernetes.io/cluster/ali-test",
					Value: "1",
				},
				{
					Key:   "kubernetes.io/role/worker",
					Value: "1",
				},
				{
					Key:   "taga",
					Value: "tagvala",
				},
				{
					Key:   "tagb",
					Value: "tagvalb",
				},
				{
					Key:   "tagc",
					Value: "tagvalc",
				},
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(res[0:2]).To(Equal(expected[0:2]))
			Expect(res[2:5]).To(ConsistOf(expected[2:5]))
		})

		It("Should generate name from ID", func() {
			id := "i-uf69zddmom11ci7est12"
			expectedName := "iZuf69zddmom11ci7est12Z"
			c := &AlicloudDriver{}
			res := c.idToName(id)
			Expect(res).To(Equal(expectedName))
		})

	})

	Context("Generate Data Disk Requests", func() {

		It("should generate multiple data disk requests", func() {
			c := &AlicloudDriver{}
			c.MachineName = "machinename"
			dataDisks := []v1alpha1.AlicloudDataDisk{
				{
					Name:               "dd1",
					Category:           "cloud_efficiency",
					Description:        "this is a disk",
					DeleteWithInstance: true,
					Encrypted:          true,
					Size:               100,
				},
				{
					Name:               "dd2",
					Category:           "cloud_ssd",
					Description:        "this is also a disk",
					DeleteWithInstance: false,
					Encrypted:          false,
					Size:               50,
				},
			}

			generatedDataDisksRequests := c.generateDataDiskRequests(dataDisks)
			expectedDataDiskRequests := []ecs.RunInstancesDataDisk{
				{
					Size:               "100",
					Category:           "cloud_efficiency",
					Encrypted:          strconv.FormatBool(true),
					DiskName:           "machinename-dd1-data-disk",
					Description:        "this is a disk",
					DeleteWithInstance: strconv.FormatBool(true),
				},
				{
					Size:               "50",
					Category:           "cloud_ssd",
					Encrypted:          strconv.FormatBool(false),
					DiskName:           "machinename-dd2-data-disk",
					Description:        "this is also a disk",
					DeleteWithInstance: strconv.FormatBool(false),
				},
			}

			Expect(generatedDataDisksRequests).To(Equal(expectedDataDiskRequests))
		})

		It("should not encrypt or delete with instance by default", func() {
			c := &AlicloudDriver{}
			c.MachineName = "machinename"
			dataDisks := []v1alpha1.AlicloudDataDisk{
				{
					Name:        "dd1",
					Category:    "cloud_efficiency",
					Description: "this is a disk",
					Size:        100,
				},
			}

			generatedDataDisksRequests := c.generateDataDiskRequests(dataDisks)
			expectedDataDiskRequests := []ecs.RunInstancesDataDisk{
				{
					Size:               "100",
					Category:           "cloud_efficiency",
					Encrypted:          strconv.FormatBool(false),
					DiskName:           "machinename-dd1-data-disk",
					Description:        "this is a disk",
					DeleteWithInstance: strconv.FormatBool(false),
				},
			}

			Expect(generatedDataDisksRequests).To(Equal(expectedDataDiskRequests))
		})
	})
})
