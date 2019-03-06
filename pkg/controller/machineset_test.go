/*
Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.

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
package controller

import (
	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("machineset", func() {

	Describe("#getMachineMachineSets", func() {

		// Testcase: It should return error when Machine does not have any labels.
		It("should return error", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			c, trackers := createController(stop, testNamespace, objects, objects, objects)
			defer trackers.Stop()

			testMachine := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Machine-test",
					Namespace: testNamespace,
					Labels:    nil,
				},
			}
			MachineSet, err := c.getMachineMachineSets(testMachine)
			Expect(err).Should(Not(BeNil()))
			Expect(MachineSet).Should(BeNil())
		})

		// Testcase: Return correct MachineSet based on the LabelSelectors.
		It("should not return error", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Spec: machinev1.MachineSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}

			testMachine := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Machine-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet, err := c.getMachineMachineSets(testMachine)
			Expect(err).Should(BeNil())
			Expect(MachineSet).Should(Not(BeNil()))
		})
	})

	Describe("#resolveMachineSetControllerRef", func() {

		// Testcase: It should not return MachineSet if only name matches but UID does not.
		It("should not return MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234", // Dummy UID
				},
			}

			testControllerRef := &metav1.OwnerReference{
				Name: "MachineSet-test",
				Kind: "MachineSet",
				UID:  "1234567", // Actual UID
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet := c.resolveMachineSetControllerRef(testNamespace, testControllerRef)
			Expect(MachineSet).Should(BeNil())
		})

		// Testcase: It should return MachineSet if name and UID matches.
		It("should return MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
			}

			testControllerRef := &metav1.OwnerReference{
				Name: "MachineSet-test",
				Kind: "MachineSet",
				UID:  "1234567",
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet := c.resolveMachineSetControllerRef(testNamespace, testControllerRef)
			Expect(MachineSet).Should(Not(BeNil()))
		})

		// Testcase: It should return MachineSet if name and UID matches.
		It("should not return MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
			}

			testControllerRef := &metav1.OwnerReference{
				Name: "MachineSet-test",
				Kind: "'MachineDeployment'", // Incorrect Kind
				UID:  "1234567",
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet := c.resolveMachineSetControllerRef(testNamespace, testControllerRef)
			Expect(MachineSet).Should(BeNil())
		})
	})
})
