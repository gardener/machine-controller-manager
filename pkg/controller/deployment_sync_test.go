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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("deployment_sync", func() {

	Describe("#getNewMachineSet", func() {
		type setup struct {
			machineDeployment  *machinev1.MachineDeployment
			isList             []*machinev1.MachineSet
			oldISs             []*machinev1.MachineSet
			createIfNotExisted bool
		}
		type expect struct {
			machineSets []*machinev1.MachineSet
			err         bool
		}
		type data struct {
			setup  setup
			action []*machinev1.MachineSet
			expect expect
		}

		machineDeployment := &machinev1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "MachineDeployment-test",
				Namespace: testNamespace,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "MachineDeployment",
				APIVersion: "machine.sapcloud.io/v1alpha1",
			},
			Spec: machinev1.MachineDeploymentSpec{
				Replicas: 1,
				Template: machinev1.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"test-label": "test-label",
						},
					},
					Spec: machinev1.MachineSpec{
						Class: machinev1.ClassSpec{
							Kind: "AWSMachineClass",
							Name: "test-machine-class",
						},
						NodeTemplateSpec: machinev1.NodeTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"key1": "value1",
								},
								Annotations: map[string]string{
									"anno1": "anno1",
								},
							},
							Spec: corev1.NodeSpec{
								Taints: []corev1.Taint{{
									Key:    "taintKey",
									Value:  "taintValue",
									Effect: "NoSchedule",
								},
								},
							},
						},
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test-label": "test-label",
					},
				},
				Strategy: machinev1.MachineDeploymentStrategy{
					Type: "RollingUpdate",
					RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
						MaxUnavailable: &intstr.IntOrString{IntVal: 1},
						MaxSurge:       &intstr.IntOrString{IntVal: 1},
					},
				},
			},
		}

		It("when new MachinSet is not found in isList", func() {
			stop := make(chan struct{})
			defer close(stop)

			testMachineSet := newMachineSets(
				1,
				&machinev1.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"test-label-dummy": "test-label-dummy",
						},
					},
					Spec: machinev1.MachineSpec{
						Class: machinev1.ClassSpec{
							Kind: "AWSMachineClass",
							Name: "test-machine-class",
						},
					},
				},
				3,
				500,
				nil,
				nil,
				nil,
				nil,
			)

			controlObjects := []runtime.Object{}
			controlObjects = append(controlObjects, testMachineSet[0], machineDeployment)
			c, trackers := createController(stop, testNamespace, controlObjects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			newMachineSet, err := c.getNewMachineSet(
				machineDeployment,
				testMachineSet,
				nil,
				true,
			)

			Expect(err).To(BeNil())
			Expect(newMachineSet.Name).Should(Not(BeNil()))
			Expect(newMachineSet.Name).Should(Not(Equal(testMachineSet[0].Name)))
		})

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlObjects := []runtime.Object{}
				mSetObjects := data.action
				controlObjects = append(controlObjects, machineDeployment)

				for i := range mSetObjects {
					controlObjects = append(controlObjects, mSetObjects[i])
				}

				c, trackers := createController(stop, testNamespace, controlObjects, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				newMachineSet, err := c.getNewMachineSet(
					data.setup.machineDeployment,
					data.action,
					nil,
					true,
				)

				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				for _, expectedMachineSet := range data.expect.machineSets {
					Expect(newMachineSet.Name).Should(Equal(expectedMachineSet.Name))
					Expect(apiequality.Semantic.DeepEqual(expectedMachineSet.Spec.Template.Spec.NodeTemplateSpec, newMachineSet.Spec.Template.Spec.NodeTemplateSpec)).Should(BeTrue())

				}
			},

			Entry("when newMachineSet is found in isList and nodeTemplate is added", &data{
				setup: setup{
					machineDeployment:  machineDeployment,
					createIfNotExisted: true,
					oldISs:             nil,
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "test-machine-class",
							},
						},
					},
					3,
					500,
					nil,
					nil,
					nil,
					nil,
				),
				expect: expect{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "AWSMachineClass",
									Name: "test-machine-class",
								},
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"key1": "value1",
										},
										Annotations: map[string]string{
											"anno1": "anno1",
										},
									},
									Spec: corev1.NodeSpec{
										Taints: []corev1.Taint{{
											Key:    "taintKey",
											Value:  "taintValue",
											Effect: "NoSchedule",
										},
										},
									},
								},
							},
						},
						3,
						500,
						nil,
						nil,
						nil,
						nil,
					),
					err: false,
				},
			}),

			Entry("when newMachineSet's nodeTemplate is modified", &data{
				setup: setup{
					machineDeployment:  machineDeployment,
					createIfNotExisted: false,
					oldISs:             nil,
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "test-machine-class",
							},
							NodeTemplateSpec: machinev1.NodeTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										"key1": "value1-dummy",
									},
									Annotations: map[string]string{
										"anno1": "anno1-dummy",
									},
								},
								Spec: corev1.NodeSpec{
									Taints: []corev1.Taint{{
										Key:    "taintKey",
										Value:  "taintValue-dummy",
										Effect: "NoSchedule",
									},
									},
								},
							},
						},
					},
					3,
					500,
					nil,
					nil,
					nil,
					nil,
				),
				expect: expect{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "AWSMachineClass",
									Name: "test-machine-class",
								},
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"key1": "value1",
										},
										Annotations: map[string]string{
											"anno1": "anno1",
										},
									},
									Spec: corev1.NodeSpec{
										Taints: []corev1.Taint{{
											Key:    "taintKey",
											Value:  "taintValue",
											Effect: "NoSchedule",
										},
										},
									},
								},
							},
						},
						3,
						500,
						nil,
						nil,
						nil,
						nil,
					),
					err: false,
				},
			}),
		)

	})

})
