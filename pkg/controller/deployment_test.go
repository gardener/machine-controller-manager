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
package controller

import (
	"errors"
	"fmt"
	"time"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("machinedeployment", func() {

	Describe("#addMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("Should enqueue the machinedeployment",
			func(preset func(), machineDeployment *machinev1.MachineDeployment) {
				stop := make(chan struct{})
				preset()
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.addMachineDeployment(testMachineDeployment)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(1))
			},
			Entry("MachineDeployment is added",
				func() {},
				testMachineDeployment,
			),
		)

	})

	Describe("#updateMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("Should enqueue the machinedeployment",
			func(preset func(), machineDeployment *machinev1.MachineDeployment) {
				stop := make(chan struct{})
				preset()
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				testMachineDeploymentUpdated := testMachineDeployment.DeepCopy()
				c.updateMachineDeployment(testMachineDeployment, testMachineDeploymentUpdated)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(1))
			},
			Entry("MachineDeployment is updated",
				func() {},
				testMachineDeployment,
			),
		)

	})

	Describe("#deleteMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("Should enqueue the machinedeployment",
			func(preset func(), machineDeployment *machinev1.MachineDeployment) {
				stop := make(chan struct{})
				preset()
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.deleteMachineDeployment(testMachineDeployment)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(1))
			},
			Entry("MachineDeployment is deleted",
				func() {},
				testMachineDeployment,
			),
		)
	})

	Describe("#addMachineSetToDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {
			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(preset func(testMachineSet *machinev1.MachineSet), queueLength int) {
				ptrBool = true

				testMachineSet = &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineSet-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						UID: "1234567",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineDeployment",
								Name:       "MachineDeployment-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineSet",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineSetSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}

				stop := make(chan struct{})
				preset(testMachineSet)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.addMachineSetToDeployment(testMachineSet)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(queueLength))
			},
			Entry("be enqueued as MachineSet is added",
				func(testMachineSet *machinev1.MachineSet) {}, 1,
			),
			Entry("be enqueued as MachineSet is deleted",
				func(testMachineSet *machinev1.MachineSet) {
					testMachineSet.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				}, 1,
			),
			Entry("be enqueued as controllerRef is nil, and it should find it via label",
				func(testMachineSet *machinev1.MachineSet) {
					testMachineSet.OwnerReferences = nil
				}, 1,
			),
			Entry("not be enqueued as controllerRef is nil, and labels are also not matching",
				func(testMachineSet *machinev1.MachineSet) {
					testMachineSet.OwnerReferences = nil
					testMachineSet.Labels = nil
				}, 0,
			),
			Entry("not be enqueued as controllerRef is not nil, but doesnt match any machine-deployment",
				func(testMachineSet *machinev1.MachineSet) {
					testMachineSet.OwnerReferences = []metav1.OwnerReference{
						{
							Kind:       "MachineDeployment",
							Name:       "MachineDeployment-test-dummy",
							UID:        "1234567",
							Controller: &ptrBool,
						},
					}
				}, 0,
			),
		)
	})

	Describe("#updateMachineSetToDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {
			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(preset func(oldMachineSet *machinev1.MachineSet, newMachineSet *machinev1.MachineSet), queueLength int) {
				ptrBool = true

				testMachineSet = &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineSet-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						UID:             "1234567",
						ResourceVersion: "123",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineDeployment",
								Name:       "MachineDeployment-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineSet",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineSetSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}
				oldMachineSet := testMachineSet
				newMachineSet := oldMachineSet.DeepCopy()
				newMachineSet.ResourceVersion = "345"

				stop := make(chan struct{})
				preset(oldMachineSet, newMachineSet)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.updateMachineSetToDeployment(oldMachineSet, newMachineSet)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(queueLength))
			},
			Entry("not be enqueued as ResourceVersion is same",
				func(oldMachineSet *machinev1.MachineSet, newMachineSet *machinev1.MachineSet) {
					newMachineSet.ResourceVersion = oldMachineSet.ResourceVersion
				}, 0,
			),
			Entry("be enqueued as newMachineSet is being deleted",
				func(oldMachineSet *machinev1.MachineSet, newMachineSet *machinev1.MachineSet) {
					oldMachineSet.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				},
				1,
			),
			Entry("be enqueued as newMachineSet's label has changed",
				func(oldMachineSet *machinev1.MachineSet, newMachineSet *machinev1.MachineSet) {
					newMachineSet.Labels = map[string]string{
						"dummy": "dummy",
					}
				},
				1,
			),
			Entry("be enqueued as newMachineSet's controllerRef has changed",
				func(oldMachineSet *machinev1.MachineSet, newMachineSet *machinev1.MachineSet) {
					newMachineSet.OwnerReferences = nil
				},
				1,
			),
			Entry("not be enqueued as both oldMachineSet and newMachineSet has nil controllerRef",
				func(oldMachineSet *machinev1.MachineSet, newMachineSet *machinev1.MachineSet) {
					newMachineSet.OwnerReferences = nil
					oldMachineSet.OwnerReferences = nil
				},
				0,
			),
			Entry("be enqueued as newMachineSet's controllerRef has changed and points to a other valid MachineDeployment",
				func(oldMachineSet *machinev1.MachineSet, newMachineSet *machinev1.MachineSet) {
					newMachineSet.OwnerReferences = []metav1.OwnerReference{
						{
							Kind:       "MachineDeployment",
							Name:       "MachineSet-test-dummy",
							UID:        "1234567",
							Controller: &ptrBool,
						},
					}
				},
				1,
			),
		)
	})

	Describe("#deleteMachineSetToDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {
			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(preset func(testMachineSet *machinev1.MachineSet), queueLength int) {
				ptrBool = true

				testMachineSet = &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineSet-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						UID: "1234567",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineDeployment",
								Name:       "MachineDeployment-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineSet",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineSetSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}

				stop := make(chan struct{})
				preset(testMachineSet)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.deleteMachineSetToDeployment(testMachineSet)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(queueLength))
			},
			Entry("be enqueued as MachineSet is deleted",
				func(testMachineSet *machinev1.MachineSet) {}, 1,
			),
			Entry("not be enqueued as MachineSet's controllerRef is nil",
				func(testMachineSet *machinev1.MachineSet) {
					testMachineSet.OwnerReferences = nil
				}, 0,
			),
			Entry("not be enqueued as MachineDeployment's UID is different in controllerRef",
				func(testMachineSet *machinev1.MachineSet) {
					testMachineSet.OwnerReferences[0].UID = "111-dummy"
				}, 0,
			),
		)
	})

	Describe("#deleteMachineToMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachine           *machinev1.Machine
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {

			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "MachineDeployment",
							Name:       "MachineDeployment-test",
							UID:        "1234567",
							Controller: &ptrBool,
						},
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(preset func(testMachine *machinev1.Machine, testMachineDeployment *machinev1.MachineDeployment), queueLength int) {
				ptrBool = true

				testMachine = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Machine-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						ResourceVersion: "123",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineSet",
								Name:       "MachineSet-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
				}

				testMachineDeployment = &machinev1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineDeployment-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						UID: "1234567",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineDeployment",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineDeploymentSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}

				stop := make(chan struct{})
				preset(testMachine, testMachineDeployment)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				objects = append(objects, testMachineSet)
				objects = append(objects, testMachine)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.deleteMachineToMachineDeployment(testMachine)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(queueLength))
			},
			Entry("not be enqueued as Machine is deleted",
				func(testMachine *machinev1.Machine, testMachineDeployment *machinev1.MachineDeployment) {}, 0,
			),
		)
	})

	Describe("#enqueueMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(preset func(testMachineDeployment *machinev1.MachineDeployment), machineDeployment *machinev1.MachineDeployment, queueLength int) {
				stop := make(chan struct{})
				preset(testMachineDeployment)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.enqueueMachineDeployment(testMachineDeployment)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(queueLength))
			},
			Entry("be enqueued as valid MachineDeployment object is provided",
				func(testMachineDeployment *machinev1.MachineDeployment) {},
				testMachineDeployment, 1,
			),
		)
	})

	Describe("#enqueueRateLimited", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(preset func(testMachineDeployment *machinev1.MachineDeployment), machineDeployment *machinev1.MachineDeployment, queueLength int) {
				stop := make(chan struct{})
				preset(testMachineDeployment)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.enqueueRateLimited(testMachineDeployment)

				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(queueLength))
			},
			Entry("be enqueued as valid MachineDeployment object is provided",
				func(testMachineDeployment *machinev1.MachineDeployment) {},
				testMachineDeployment, 1,
			),
		)
	})

	Describe("#enqueueMachineDeploymentAfter", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(postset func(), machineDeployment *machinev1.MachineDeployment, queueLength int) {
				stop := make(chan struct{})
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.enqueueMachineDeploymentAfter(testMachineDeployment, 1*time.Second)
				postset()
				waitForCacheSync(stop, c)
				Expect(c.machineDeploymentQueue.Len()).To(Equal(queueLength))
			},
			Entry("be enqueued after 1 second",
				func() {
					time.Sleep(2 * time.Second)
				},
				testMachineDeployment, 1,
			),
			Entry("not be enqueued immediately",
				func() {},
				testMachineDeployment, 0,
			),
		)
	})

	Describe("#getMachineDeploymentForMachine", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachine           *machinev1.Machine
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("MachineDeployment should",
			func(preset func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet), expectedMachineDeploymentName string) {
				ptrBool = true

				testMachine = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Machine-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						ResourceVersion: "123",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineSet",
								Name:       "MachineSet-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
				}

				testMachineSet = &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineSet-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						UID: "1234567",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineDeployment",
								Name:       "MachineDeployment-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineSet",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineSetSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}

				stop := make(chan struct{})
				preset(testMachine, testMachineSet)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				objects = append(objects, testMachineSet)
				objects = append(objects, testMachine)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				actualMachineDeployment := c.getMachineDeploymentForMachine(testMachine)

				waitForCacheSync(stop, c)
				if expectedMachineDeploymentName != "" {
					Expect(actualMachineDeployment.Name).To(Equal(expectedMachineDeploymentName))
				} else {
					Expect(actualMachineDeployment).To(BeNil())
				}

			},
			Entry("return the expected machine deployment",
				func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet) {},
				"MachineDeployment-test",
			),
			Entry("return nil as machine's controller-ref is buggy",
				func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet) {
					testMachine.OwnerReferences[0].Kind = "MachineSetDummy"
				},
				"",
			),
			Entry("return nil as machine's controller-ref has different UID",
				func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet) {
					testMachine.OwnerReferences[0].UID = "000-dummy"
				},
				"",
			),
			Entry("return nil as machine-set's controller-ref is nil",
				func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet) {
					testMachineSet.OwnerReferences = nil
				},
				"",
			),
		)
	})

	Describe("#getMachineSetsForMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {
			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label-1": "test-label-1",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label-1": "test-label-1",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label-1": "test-label-1",
						},
					},
				},
			}
		})

		DescribeTable("this should",
			func(preset func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet1 *machinev1.MachineSet, testMachineSet2 *machinev1.MachineSet), expectedMachineSetNames []string, expectedErr error) {
				ptrBool = true

				testMachineSet = &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineSet-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label-1": "test-label-1",
						},
						UID: "1234567",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineDeployment",
								Name:       "MachineDeployment-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineSet",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineSetSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}
				testMachineSet1 := testMachineSet.DeepCopy()
				testMachineSet1.Name = "MachineSet-test-1"
				testMachineSet2 := testMachineSet1.DeepCopy()
				testMachineSet2.Name = "MachineSet-test-2"
				testMachineSet2.Labels = map[string]string{
					"test-label-1": "test-label-1",
					"test-label-2": "tesst-label-2",
				}

				stop := make(chan struct{})
				preset(testMachineDeployment, testMachineSet1, testMachineSet2)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				objects = append(objects, testMachineSet1)
				objects = append(objects, testMachineSet2)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				actualMachineSets, err := c.getMachineSetsForMachineDeployment(testMachineDeployment)

				waitForCacheSync(stop, c)
				if expectedErr != nil {
					Expect(err).To(Not(BeNil()))
				} else {
					Expect(err).To(BeNil())
				}

				Expect(len(actualMachineSets)).To(Equal(len(expectedMachineSetNames)))

			},
			Entry("return both machinesets as selector matches.",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet1 *machinev1.MachineSet, testMachineSet2 *machinev1.MachineSet) {
				}, []string{"MachineSet-test-1", "MachineSet-test-2"}, nil,
			),
			Entry("return no machinesets as selector matches none.",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet1 *machinev1.MachineSet, testMachineSet2 *machinev1.MachineSet) {
					testMachineSet1.Labels = nil
					testMachineSet2.Labels = nil
					testMachineSet1.OwnerReferences = nil
					testMachineSet2.OwnerReferences = nil
				}, []string{}, nil,
			),
			Entry("return no machinesets as selector is invalid.",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet1 *machinev1.MachineSet, testMachineSet2 *machinev1.MachineSet) {
					testMachineDeployment.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label-1": "dummy",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "dummy-key",
								Values:   []string{"dummy-value"},
								Operator: "dummy",
							},
						}}
				}, []string{}, errors.New("Invalid operator error"),
			),
			Entry("return only one machineset as other one doesnt match the selector",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet1 *machinev1.MachineSet, testMachineSet2 *machinev1.MachineSet) {
					testMachineSet1.OwnerReferences = nil
					testMachineSet1.Labels = nil
				}, []string{"MachineSet-test-2"}, nil,
			),
		)
	})

	Describe("#getMachineMapForMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachine           *machinev1.Machine
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("this should",
			func(preset func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet, testMachineDeployment *machinev1.MachineDeployment), expectedMachineNames []string, expectedError error) {
				ptrBool = true

				testMachine = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Machine-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						ResourceVersion: "123",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineSet",
								Name:       "MachineSet-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
				}

				testMachineSet = &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineSet-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						UID: "1234567",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineDeployment",
								Name:       "MachineDeployment-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineSet",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineSetSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}
				preset(testMachine, testMachineSet, testMachineDeployment)

				testMachine1 := testMachine.DeepCopy()
				testMachine1.Name = "Machine-1"
				testMachine2 := testMachine.DeepCopy()
				testMachine2.Name = "Machine-2"
				testMachine3 := testMachine.DeepCopy()
				testMachine3.Name = "Machine-3"

				stop := make(chan struct{})
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				objects = append(objects, testMachineSet)
				objects = append(objects, testMachine1)
				objects = append(objects, testMachine2)
				objects = append(objects, testMachine3)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				actualMachineMap, err := c.getMachineMapForMachineDeployment(testMachineDeployment, []*machinev1.MachineSet{testMachineSet})

				waitForCacheSync(stop, c)

				if expectedError != nil {
					Expect(err).To(Not(BeNil()))
				} else {
					Expect(err).To(BeNil())
				}

				actualMachines := []string{}
				for i := range actualMachineMap {
					for _, mach := range actualMachineMap[i].Items {
						actualMachines = append(actualMachines, mach.Name)
					}
				}
				Expect(len(actualMachines)).To(Equal(len(expectedMachineNames)))

			},
			Entry("return all the machines in the map",
				func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet, testMachineDeployment *machinev1.MachineDeployment) {
				},
				[]string{"Machine-1", "Machine-2", "Machine-3"}, nil,
			),
			Entry("return none of the machines in the map as selector doesnt match",
				func(testMachine *machinev1.Machine, testMachineSet *machinev1.MachineSet, testMachineDeployment *machinev1.MachineDeployment) {
					testMachine.Labels = nil
					testMachine.OwnerReferences = nil
				},
				[]string{}, nil,
			),
		)
	})

	Describe("#reconcileClusterMachineDeployment", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachineSet        *machinev1.MachineSet
			testMachine           *machinev1.Machine
			testNode              *corev1.Node
			ptrBool               bool
		)
		BeforeEach(func() {
			ptrBool = true
			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID:        "1234567",
					Generation: 5,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 5,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Strategy: machinev1.MachineDeploymentStrategy{
						Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
						RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
							MaxUnavailable: &intstr.IntOrString{IntVal: int32(1)},
							MaxSurge:       &intstr.IntOrString{IntVal: int32(1)},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
				Status: machinev1.MachineDeploymentStatus{
					AvailableReplicas:  5,
					ObservedGeneration: 5,
					ReadyReplicas:      5,
					Replicas:           5,
					UpdatedReplicas:    5,
					Conditions: []machinev1.MachineDeploymentCondition{
						{
							LastTransitionTime: metav1.Now(),
							LastUpdateTime:     metav1.Now(),
							Message:            "Deployment has minimum availability.",
							Reason:             "MinimumReplicasAvailable",
							Status:             "True",
							Type:               "Available",
						},
					},
				},
			}

			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					Annotations: map[string]string{
						"deployment.kubernetes.io/revision": "1",
					},
					UID: "1234567",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "MachineDeployment",
							Name:       "MachineDeployment-test",
							UID:        "1234567",
							Controller: &ptrBool,
						},
					},
					Generation: 5,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 5,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
				Status: machinev1.MachineSetStatus{
					AvailableReplicas:    5,
					FullyLabeledReplicas: 5,
					ObservedGeneration:   5,
					ReadyReplicas:        5,
					Replicas:             5,
					LastOperation: machinev1.LastOperation{
						LastUpdateTime: metav1.Now(),
					},
				},
			}

			testMachine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Machine-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
						"node":       "Node1-test",
					},
					UID: "1234567",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "MachineSet",
							Name:       "MachineSet-test",
							UID:        "1234567",
							Controller: &ptrBool,
						},
					},
					Generation: 5,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Name: "MachineClass-test",
						Kind: "MachineClass",
					},
				},
				Status: machinev1.MachineStatus{
					LastOperation: machinev1.LastOperation{
						LastUpdateTime: metav1.Now(),
					},
					Node: "Node1-test",
				},
			}
			testNode = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Node1-test",
					Namespace: "",
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID:        "1234567",
					Generation: 5,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
			}
		})

		DescribeTable("This should",
			func(preset func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet),
				postcheck func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet []machinev1.MachineSet, testNode *corev1.Node) error) {

				stop := make(chan struct{})
				preset(testMachineDeployment, testMachineSet)
				defer close(stop)

				objects := []runtime.Object{}
				coreObjects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				objects = append(objects, testMachineSet)
				objects = append(objects, testMachine)
				coreObjects = append(coreObjects, testNode)
				c, trackers := createController(stop, testNamespace, objects, nil, coreObjects)
				c.autoscalerScaleDownAnnotationDuringRollout = true

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				Key := testNamespace + "/" + testMachineDeployment.Name
				c.reconcileClusterMachineDeployment(Key)

				waitForCacheSync(stop, c)
				actualMachineDeployment, _ := c.controlMachineClient.MachineDeployments(testNamespace).Get(testMachineDeployment.Name, metav1.GetOptions{})
				actualMachineSets, _ := c.controlMachineClient.MachineSets(testNamespace).List(metav1.ListOptions{})
				testNode, _ := c.targetCoreClient.CoreV1().Nodes().Get(testNode.Name, metav1.GetOptions{})

				Expect(postcheck(actualMachineDeployment, actualMachineSets.Items, testNode)).To(BeNil())
			},
			Entry("reconcile the machinedeployment and return nil",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					return nil
				},
			),
			Entry("create a machineset while reconciling",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					testMachineSet.ObjectMeta = metav1.ObjectMeta{}
					testMachineSet.TypeMeta = metav1.TypeMeta{}
					testMachineSet.Spec = machinev1.MachineSetSpec{}
					testMachineSet.Status = machinev1.MachineSetStatus{}
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					if len(testMachineSets) != 1 {
						return errors.New("It should have created one machine set")
					}
					return nil
				},
			),
			Entry("should not create new machineset if labelselector is empty",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					testMachineDeployment.Spec.Selector = &metav1.LabelSelector{}
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					if len(testMachineSets) > 1 {
						return errors.New("It should not have created one machine set")
					}
					return nil
				},
			),
			Entry("should remove the finalizer from deployment when deleted and no machinesets are available.",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					testMachineDeployment.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					if len(testMachineDeployment.Finalizers) > 0 {
						return errors.New("It should have removed the finalizers")
					}
					return nil
				},
			),
			Entry("should not create new machineset with dummy-unknown strategy",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					testMachineDeployment.Spec.Strategy = machinev1.MachineDeploymentStrategy{
						Type: "Dummy",
					}
					testMachineSet.ObjectMeta = metav1.ObjectMeta{}
					testMachineSet.TypeMeta = metav1.TypeMeta{}
					testMachineSet.Spec = machinev1.MachineSetSpec{}
					testMachineSet.Status = machinev1.MachineSetStatus{}
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					if len(testMachineSets) != 0 {
						return errors.New("It shouldn't have created machine set")
					}
					return nil
				},
			),
			Entry("Recreate case: should create new machineset with recreate strategy",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					testMachineDeployment.Spec.Strategy = machinev1.MachineDeploymentStrategy{
						Type: machinev1.RecreateMachineDeploymentStrategyType,
					}
					testMachineSet.ObjectMeta = metav1.ObjectMeta{}
					testMachineSet.TypeMeta = metav1.TypeMeta{}
					testMachineSet.Spec = machinev1.MachineSetSpec{}
					testMachineSet.Status = machinev1.MachineSetStatus{}
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					if len(testMachineSets) < 1 {
						return errors.New("It should have created one machine set")
					}
					return nil
				},
			),
			Entry("Recreate case: should completely scale-down the old machineset",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					testMachineDeployment.Spec.Strategy = machinev1.MachineDeploymentStrategy{
						Type: machinev1.RecreateMachineDeploymentStrategyType,
					}
					testMachineDeployment.Spec.Template.Spec.Class.Name = "MachineClass-test-new"
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					// Old machine set should exist and should be scaled-down to zero.
					if len(testMachineSets) == 0 || testMachineSets[0].Spec.Replicas != 0 {
						return errors.New("It should have scaled-down old machineset to zero")
					}
					return nil
				},
			),
			Entry("Recreate case: should fully scale-up the new machineset",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					testMachineDeployment.Spec.Strategy = machinev1.MachineDeploymentStrategy{
						Type: machinev1.RecreateMachineDeploymentStrategyType,
					}
					testMachineDeployment.Spec.Template.Spec.Class.Name = "MachineClass-test-new"

					// Recreate strategy creates new machineset only after all old machines are deleted.
					testMachineSet.Spec.Replicas = 0
					testMachineSet.Status = machinev1.MachineSetStatus{
						AvailableReplicas:    0,
						FullyLabeledReplicas: 0,
						ObservedGeneration:   5,
						ReadyReplicas:        0,
						Replicas:             0,
						LastOperation: machinev1.LastOperation{
							LastUpdateTime: metav1.Now(),
						},
					}
					testMachine = &machinev1.Machine{}
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {

					fmt.Println(testMachineSets)
					if len(testMachineSets) != 2 || testMachineSets[1].Spec.Replicas != testMachineDeployment.Spec.Replicas {
						return errors.New("It should have fully scaled-up the new machineset")
					}
					return nil
				},
			),

			Entry("rolling-update case: should create new machine-set with rolling-update",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					// This should trigger rolliing-update.
					testMachineDeployment.Spec.Template.Spec.Class.Name = "MachineClass-test-new"
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					if len(testMachineSets) != 2 {
						return errors.New("It should have created a new machine set")
					}
					return nil
				},
			),
			Entry("rolling-update case: both new and old machineset should have appropriate desired replicas",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					// This should trigger rolliing-update.
					testMachineDeployment.Spec.Template.Spec.Class.Name = "MachineClass-test-new"
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					// As this is the first round of reconciliation.
					expectedReplicaNewMachineSet := testMachineDeployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal
					expectedReplicaOldMachineSet := testMachineDeployment.Spec.Replicas - testMachineDeployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal

					if len(testMachineSets) != 2 {
						return errors.New("It should have created a new machine set")
					}
					newMachineSet := testMachineSets[1]
					oldMachineSet := testMachineSets[0]

					if newMachineSet.Spec.Replicas != expectedReplicaNewMachineSet || oldMachineSet.Spec.Replicas != expectedReplicaOldMachineSet {
						return errors.New("both new and old machine set should have appropriate replicas set")
					}
					return nil
				},
			),
			Entry("rolling-update case: scale down annotation of autoscaler should be added on the node object",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					// This should trigger rolliing-update.
					testMachineDeployment.Spec.Template.Spec.Class.Name = "MachineClass-test-new"
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					// Can not add more advanced tests, as machine-deployment controller adds the annotation directly on nodes, and lifecycle of the node is managed outside of the MCM.

					if len(testMachineSets) != 2 {
						return errors.New("it should have created a new machine set")
					}

					if _, ok := testNode.Annotations[ClusterAutoscalerScaleDownDisabledAnnotationKey]; !ok {
						return errors.New("it should have added an scale-down annotation on the node")
					}
					return nil
				},
			),
			Entry("rolling-back case: machine-deployment should point to old machine class and RollbackTo should be removed",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					// This should trigger rolliing-update..
					testMachineDeployment.Spec.Template.Spec.Class.Name = "MachineClass-test-new"
					testMachineDeployment.Spec.RollbackTo = &machinev1.RollbackConfig{
						Revision: 1,
					}

				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {
					// RollbackTo should be removed after rollback.
					if testMachineDeployment.Spec.RollbackTo != nil {
						return errors.New("RollbackTo field should have been removed from machine-deployment")
					}

					// MachineDeployment should point again to the old machine-set.
					if testMachineDeployment.Spec.Template.Spec.Class.Name == testMachineSet.Name {
						return errors.New("Machinedeployment should point to the old machineset after rollback")
					}

					return nil
				},
			),
			Entry("paused case: Replicas of the machineset should not change after the paused field is set",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet *machinev1.MachineSet) {
					// This should trigger rolliing-update.
					testMachineDeployment.Spec.Template.Spec.Class.Name = "MachineClass-test-new"
					testMachineDeployment.Spec.Paused = true
				},
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSets []machinev1.MachineSet, testNode *corev1.Node) error {

					if len(testMachineSets) != 1 {
						return errors.New("There should be only one old machine-set")
					}
					oldMachineSet := testMachineSets[0]

					if oldMachineSet.Spec.Replicas != testMachineSet.Spec.Replicas {
						return errors.New("old machineset's replicas should not have changed")
					}
					return nil
				},
			),
		)
	})

	Describe("#terminateMachineSets", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
			testMachineSet        *machinev1.MachineSet
			ptrBool               bool
		)
		BeforeEach(func() {
			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label-1": "test-label-1",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label-1": "test-label-1",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label-1": "test-label-1",
						},
					},
				},
			}
		})

		DescribeTable("this should",
			func(preset func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet1 *machinev1.MachineSet, testMachineSet2 *machinev1.MachineSet), expectedNumMachineSets int) {
				ptrBool = true

				testMachineSet = &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "MachineSet-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label-1": "test-label-1",
						},
						UID: "1234567",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineDeployment",
								Name:       "MachineDeployment-test",
								UID:        "1234567",
								Controller: &ptrBool,
							},
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "MachineSet",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Spec: machinev1.MachineSetSpec{
						Replicas: 3,
						Template: machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Name: "MachineClass-test",
									Kind: "MachineClass",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test-label": "test-label",
							},
						},
					},
				}

				testMachineSet1 := testMachineSet.DeepCopy()
				testMachineSet1.Name = "MachineSet-test-1"
				testMachineSet2 := testMachineSet.DeepCopy()
				testMachineSet2.Name = "MachineSet-test-2"
				testMachineSets := []*machinev1.MachineSet{
					testMachineSet1, testMachineSet2,
				}

				stop := make(chan struct{})
				preset(testMachineDeployment, testMachineSet1, testMachineSet2)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				objects = append(objects, testMachineSet1)
				objects = append(objects, testMachineSet2)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.terminateMachineSets(testMachineSets, testMachineDeployment)

				waitForCacheSync(stop, c)
				actualMachineSets, _ := c.controlMachineClient.MachineSets(testNamespace).List(metav1.ListOptions{})

				Expect(len(actualMachineSets.Items)).To(Equal(expectedNumMachineSets))

			},
			Entry("delete all the machinesets",
				func(testMachineDeployment *machinev1.MachineDeployment, testMachineSet1 *machinev1.MachineSet, testMachineSet2 *machinev1.MachineSet) {
				}, 0,
			),
		)
	})

	Describe("#deleteMachineDeploymentFinalizers", func() {
		var (
			testMachineDeployment *machinev1.MachineDeployment
		)
		BeforeEach(func() {

			testMachineDeployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineDeployment-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineDeployment",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}
		})

		DescribeTable("this should",
			func(preset func()) {
				stop := make(chan struct{})
				preset()
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineDeployment)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.deleteMachineDeploymentFinalizers(testMachineDeployment)

				waitForCacheSync(stop, c)
				actualMachineDeployment, _ := c.controlMachineClient.MachineDeployments(testNamespace).Get(testMachineDeployment.Name, metav1.GetOptions{})
				Expect(len(actualMachineDeployment.Finalizers)).To(Equal(0))
			},
			Entry("remove the finalizer from the machine-deployment",
				func() {
					testMachineDeployment.Finalizers = []string{DeleteFinalizerName}
				},
			),
		)
	})
})
