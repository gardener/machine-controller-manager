/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

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
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Driver Openstack", func() {

	Context("#GetVolNames", func() {
		var hostPathPVSpec = corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/mnt/data",
				},
			},
		}

		It("should handle in-tree PV (with .spec.cinder)", func() {
			driver := &OpenStackDriver{}
			pvs := []corev1.PersistentVolumeSpec{
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						Cinder: &corev1.CinderPersistentVolumeSource{
							VolumeID: "a5ebd05f-934f-480d-b06f-41b372ed631e",
						},
					},
				},
				hostPathPVSpec,
			}

			actual, err := driver.GetVolNames(pvs)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual).To(Equal([]string{"a5ebd05f-934f-480d-b06f-41b372ed631e"}))
		})

		It("should handle out-of-tree PV (with .spec.csi.volumeHandle)", func() {
			driver := &OpenStackDriver{}
			pvs := []corev1.PersistentVolumeSpec{
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "io.kubernetes.storage.mock",
							VolumeHandle: "vol-2",
						},
					},
				},
				{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "cinder.csi.openstack.org",
							VolumeHandle: "a5ebd05f-934f-480d-b06f-41b372ed631e",
						},
					},
				},
				hostPathPVSpec,
			}

			actual, err := driver.GetVolNames(pvs)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual).To(Equal([]string{"a5ebd05f-934f-480d-b06f-41b372ed631e"}))
		})
	})
})
