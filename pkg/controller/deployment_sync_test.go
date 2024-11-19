// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
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
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxUnavailable: &intstr.IntOrString{IntVal: 1},
							MaxSurge:       &intstr.IntOrString{IntVal: 1},
						},
					},
				},
			},
		}

		It("when new MachineSet is not found in isList", func() {
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
				context.TODO(),
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
					context.TODO(),
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

	// scale(ctx context.Context, deployment *v1alpha1.MachineDeployment, newIS *v1alpha1.MachineSet, oldISs []*v1alpha1.MachineSet) error
	Describe("#scale", func() {
		const (
			nameMachineSet1 = "ms1"
			nameMachineSet2 = "ms2"
			nameMachineSet3 = "ms3"
		)
		var (
			mSetSpecTemplate = &machinev1.MachineTemplateSpec{
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
			}

			mDeploymentSpecTemplate = &machinev1.MachineTemplateSpec{
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
			}
		)

		type setup struct {
			machineDeployment *machinev1.MachineDeployment
			oldISs            []*machinev1.MachineSet
			newIS             *machinev1.MachineSet
		}
		type expect struct {
			machineSets []*machinev1.MachineSet
			err         bool
		}
		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				var controlObjects []runtime.Object
				Expect(data.setup.machineDeployment).To(Not(BeNil()))
				controlObjects = append(controlObjects, data.setup.machineDeployment)

				for i := range data.setup.oldISs {
					controlObjects = append(controlObjects, data.setup.oldISs[i])
				}

				if data.setup.newIS != nil {
					controlObjects = append(controlObjects, data.setup.newIS)
				}
				c, trackers := createController(stop, testNamespace, controlObjects, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				err := c.scale(
					context.TODO(),
					data.setup.machineDeployment,
					data.setup.newIS,
					data.setup.oldISs,
				)

				waitForCacheSync(stop, c)

				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				currMachineSetsList, err := c.controlMachineClient.MachineSets(testNamespace).List(context.TODO(), metav1.ListOptions{})

				Expect(err).To(BeNil())

				for _, expectedMachineSet := range data.expect.machineSets {
					flag := false
					for _, ms := range currMachineSetsList.Items {
						if ms.Name == expectedMachineSet.Name {
							flag = true
							Expect(ms.Spec.Replicas).To(Equal(expectedMachineSet.Spec.Replicas))
							Expect(ms.Annotations[DesiredReplicasAnnotation]).To(Equal(expectedMachineSet.Annotations[DesiredReplicasAnnotation]))
							Expect(ms.Annotations[MaxReplicasAnnotation]).To(Equal(expectedMachineSet.Annotations[MaxReplicasAnnotation]))
							break
						}
					}
					Expect(flag).To(BeTrue())
				}
			},

			// Cases:
			//  1. deploymentReplicasToAdd > 0
			//  2. deploymentReplicasToAdd =0
			//  3. deploymentReplicasToAdd < 0

			// In the below tests the machineDeployment passed in setup is the machineDeployment after scale-up
			// To determine the previous machineDeployment, look at the oldIS and newIS replicas passed
			// in setup , maxSurge has been set to 2 , both for old and new machineDeployment.

			Entry("if scale-up, then only new machineSet should be scaled-up", &data{
				setup: setup{
					machineDeployment: newMachineDeployment(mDeploymentSpecTemplate, 10, 500, 2, 0, nil, nil, nil, nil),
					oldISs: []*machinev1.MachineSet{newMachineSet(
						mSetSpecTemplate,
						nameMachineSet1,
						1,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					), newMachineSet(
						mSetSpecTemplate,
						nameMachineSet2,
						3,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					)},
					newIS: newMachineSet(
						mSetSpecTemplate,
						nameMachineSet3,
						5,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					),
				},
				expect: expect{
					machineSets: []*machinev1.MachineSet{
						newMachineSet(mSetSpecTemplate,
							nameMachineSet1,
							1,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "10",
								MaxReplicasAnnotation:     "12",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet2,
							3,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "10",
								MaxReplicasAnnotation:     "12",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet3,
							8,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "10",
								MaxReplicasAnnotation:     "12",
							},
							nil,
						),
					},
					err: false,
				},
			}),
			Entry("if scale-down such that leftover replicas to add > 0, then proportional scale-down, left over to be adjusted in latest machineSet", &data{
				setup: setup{
					machineDeployment: newMachineDeployment(mDeploymentSpecTemplate, 3, 500, 2, 0, nil, nil, nil, nil),
					oldISs: []*machinev1.MachineSet{newMachineSet(
						mSetSpecTemplate,
						nameMachineSet1,
						1,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					), newMachineSet(
						mSetSpecTemplate,
						nameMachineSet2,
						3,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					)},
					newIS: newMachineSet(
						mSetSpecTemplate,
						nameMachineSet3,
						5,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					),
				},
				expect: expect{
					machineSets: []*machinev1.MachineSet{
						newMachineSet(mSetSpecTemplate,
							nameMachineSet1,
							1,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "3",
								MaxReplicasAnnotation:     "5",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet2,
							2,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "3",
								MaxReplicasAnnotation:     "5",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet3,
							2, // leftover adjusted, so 2 instead of 3
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "3",
								MaxReplicasAnnotation:     "5",
							},
							nil,
						),
					},
					err: false,
				},
			}),
			Entry("if scale-down such that leftover replicas after proportional scaling =0, then proportional scale-down should be done", &data{
				setup: setup{
					machineDeployment: newMachineDeployment(mDeploymentSpecTemplate, 5, 500, 2, 0, nil, nil, nil, nil),
					oldISs: []*machinev1.MachineSet{newMachineSet(
						mSetSpecTemplate,
						nameMachineSet1,
						1,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "8",
							MaxReplicasAnnotation:     "10",
						},
						nil,
					), newMachineSet(
						mSetSpecTemplate,
						nameMachineSet2,
						3,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "8",
							MaxReplicasAnnotation:     "10",
						},
						nil,
					)},
					newIS: newMachineSet(
						mSetSpecTemplate,
						nameMachineSet3,
						6,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "8",
							MaxReplicasAnnotation:     "10",
						},
						nil,
					),
				},
				expect: expect{
					machineSets: []*machinev1.MachineSet{
						newMachineSet(mSetSpecTemplate,
							nameMachineSet1,
							1,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "5",
								MaxReplicasAnnotation:     "7",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet2,
							2,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "5",
								MaxReplicasAnnotation:     "7",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet3,
							4,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "5",
								MaxReplicasAnnotation:     "7",
							},
							nil,
						),
					},
					err: false,
				},
			}),
			Entry("if no scaling happened, but still for edge cases, machineSet replicas shouldn't change", &data{
				setup: setup{
					machineDeployment: newMachineDeployment(mDeploymentSpecTemplate, 7, 500, 2, 0, nil, nil, nil, nil),
					oldISs: []*machinev1.MachineSet{newMachineSet(
						mSetSpecTemplate,
						nameMachineSet1,
						1,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					), newMachineSet(
						mSetSpecTemplate,
						nameMachineSet2,
						3,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					)},
					newIS: newMachineSet(
						mSetSpecTemplate,
						nameMachineSet3,
						5,
						500,
						nil,
						nil,
						map[string]string{
							DesiredReplicasAnnotation: "7",
							MaxReplicasAnnotation:     "9",
						},
						nil,
					),
				},
				expect: expect{
					machineSets: []*machinev1.MachineSet{
						newMachineSet(mSetSpecTemplate,
							nameMachineSet1,
							1,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "7",
								MaxReplicasAnnotation:     "9",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet2,
							3,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "7",
								MaxReplicasAnnotation:     "9",
							},
							nil,
						),
						newMachineSet(mSetSpecTemplate,
							nameMachineSet3,
							5,
							500,
							nil,
							nil,
							map[string]string{
								DesiredReplicasAnnotation: "7",
								MaxReplicasAnnotation:     "9",
							},
							nil,
						),
					},
					err: false,
				},
			}),
			Entry("if no scaling needed but DesiredReplicas Annotation for the only active machineSet is outdated, it should be updated with the correct value", &data{
				setup: setup{
					machineDeployment: newMachineDeployment(mDeploymentSpecTemplate, 1, 500, 2, 0, nil, nil, nil, nil),
					oldISs: []*machinev1.MachineSet{
						newMachineSet(mSetSpecTemplate, nameMachineSet1, 1, 500, nil, nil, map[string]string{DesiredReplicasAnnotation: "2", MaxReplicasAnnotation: "3"}, nil),
					},
					newIS: nil,
				},
				expect: expect{
					err: false,
					machineSets: []*machinev1.MachineSet{
						newMachineSet(mSetSpecTemplate, nameMachineSet1, 1, 500, nil, nil, map[string]string{DesiredReplicasAnnotation: "1", MaxReplicasAnnotation: "3"}, nil),
					},
				},
			}),
		)
	})
})
