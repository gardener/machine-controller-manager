// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var _ = Describe("deployment_util", func() {
	var (
		machineDeployment *machinev1.MachineDeployment
	)

	BeforeEach(func() {
		machineDeployment = &machinev1.MachineDeployment{
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
	})

	DescribeTable("#MaxUnavailable",
		func(strategy machinev1.MachineDeploymentStrategy, replicas int, expected int) {
			machineDeployment.Spec.Strategy = strategy
			machineDeployment.Spec.Replicas = int32(replicas)
			Expect(MaxUnavailable(*machineDeployment)).To(Equal(int32(expected)))
		},

		Entry("strategy is neither RollingUpdate nor InPlaceUpdate", machinev1.MachineDeploymentStrategy{Type: ""}, 2, 0),
		Entry("strategy is RollingUpdate and replicas is zeros", machinev1.MachineDeploymentStrategy{Type: machinev1.RollingUpdateMachineDeploymentStrategyType}, 0, 0),

		// Case: RollingUpdate strategy with valid maxUnavailable and replicas
		Entry("strategy is RollingUpdate with maxUnavailable=1, replicas=5",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge:       ptr.To(intstr.FromInt(1)),
						MaxUnavailable: ptr.To(intstr.FromInt(1)),
					},
				},
			}, 5, 1),

		// Case: RollingUpdate strategy with maxUnavailable greater than replicas
		Entry("strategy is RollingUpdate with maxUnavailable=10, replicas=5",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge:       ptr.To(intstr.FromInt(2)),
						MaxUnavailable: ptr.To(intstr.FromInt(10)),
					},
				},
			}, 5, 5),

		// Case: InPlaceUpdate strategy with ManualUpdate=true
		Entry("strategy is InPlaceUpdate with ManualUpdate=true, maxUnavailable=2, replicas=5",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge:       ptr.To(intstr.FromInt(1)),
						MaxUnavailable: ptr.To(intstr.FromInt(2)),
					},
					OrchestrationType: machinev1.OrchestrationTypeManual,
				},
			}, 5, 2),

		// Case: InPlaceUpdate strategy with ManualUpdate=false and maxUnavailable > replicas
		Entry("strategy is InPlaceUpdate with ManualUpdate=false, maxUnavailable=10, replicas=3",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge:       ptr.To(intstr.FromInt(1)),
						MaxUnavailable: ptr.To(intstr.FromInt(10)),
					},
					OrchestrationType: machinev1.OrchestrationTypeAuto,
				},
			}, 3, 3),

		// Case: InPlaceUpdate strategy with ManualUpdate=true and replicas=0
		Entry("strategy is InPlaceUpdate with ManualUpdate=true, replicas=0",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
					OrchestrationType: machinev1.OrchestrationTypeManual,
				},
			}, 0, 0),
	)

	Describe("#NewISNewReplicas", func() {
		type data struct {
			replicas      int
			oldISReplicas int
			newISReplicas int
			expected      int
			shouldError   bool
		}

		DescribeTable("#table",
			func(strategy machinev1.MachineDeploymentStrategy, testData data) {
				// Setup MachineDeployment and MachineSets
				deployment := &machinev1.MachineDeployment{
					Spec: machinev1.MachineDeploymentSpec{
						Replicas: int32(testData.replicas),
						Strategy: strategy,
					},
				}

				newIS := &machinev1.MachineSet{
					Spec:   machinev1.MachineSetSpec{Replicas: int32(testData.newISReplicas)},
					Status: machinev1.MachineSetStatus{Replicas: int32(testData.newISReplicas)},
				}

				allISs := []*machinev1.MachineSet{
					newIS,
					{Spec: machinev1.MachineSetSpec{Replicas: int32(testData.oldISReplicas)}, Status: machinev1.MachineSetStatus{Replicas: int32(testData.oldISReplicas)}},
				}

				result, err := NewISNewReplicas(deployment, allISs, newIS)

				if testData.shouldError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(int32(testData.expected)))
				}
			},

			Entry("RollingUpdate, scale up within max surge",
				machinev1.MachineDeploymentStrategy{
					Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
					RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxSurge: ptr.To(intstr.FromInt(2)),
						},
					},
				}, data{replicas: 5, oldISReplicas: 4, newISReplicas: 2, expected: 3, shouldError: false},
			),

			Entry("RollingUpdate, max surge reached",
				machinev1.MachineDeploymentStrategy{
					Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
					RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxSurge: ptr.To(intstr.FromInt(2)),
						},
					},
				}, data{replicas: 5, oldISReplicas: 4, newISReplicas: 3, expected: 3, shouldError: false},
			),

			Entry("Recreate strategy",
				machinev1.MachineDeploymentStrategy{
					Type: machinev1.RecreateMachineDeploymentStrategyType,
				}, data{replicas: 5, oldISReplicas: 4, newISReplicas: 2, expected: 5, shouldError: false},
			),

			Entry("InPlaceUpdate with ManualUpdate=true",
				machinev1.MachineDeploymentStrategy{
					Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
						OrchestrationType: machinev1.OrchestrationTypeManual,
					},
				}, data{replicas: 5, oldISReplicas: 4, newISReplicas: 2, expected: 0, shouldError: false},
			),

			Entry("InPlaceUpdate, scale up within max surge",
				machinev1.MachineDeploymentStrategy{
					Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxSurge: ptr.To(intstr.FromInt(2)),
						},
						OrchestrationType: machinev1.OrchestrationTypeAuto,
					},
				}, data{replicas: 5, oldISReplicas: 4, newISReplicas: 2, expected: 3, shouldError: false},
			),

			Entry("InPlaceUpdate with ManualUpdate=false, max surge already reached",
				machinev1.MachineDeploymentStrategy{
					Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxSurge: ptr.To(intstr.FromInt(1)),
						},
						OrchestrationType: machinev1.OrchestrationTypeAuto,
					},
				}, data{replicas: 5, oldISReplicas: 4, newISReplicas: 2, expected: 2, shouldError: false},
			),

			Entry("Unsupported strategy type",
				machinev1.MachineDeploymentStrategy{
					Type: "UnsupportedType",
				}, data{replicas: 5, oldISReplicas: 4, newISReplicas: 2, expected: 0, shouldError: true},
			),
		)
	})

	Describe("#SetNewMachineSetNodeTemplate", func() {
		It("when nodeTemplate is updated in MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			testMachineSet := newMachineSets(
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
			)

			newRevision := "2"
			c, trackers := createController(stop, testNamespace, nil, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			nodeTemplateChanged := SetNewMachineSetNodeTemplate(
				machineDeployment,
				testMachineSet[0],
				newRevision,
				true,
			)

			Expect(nodeTemplateChanged).To(BeTrue())
			Expect(apiequality.Semantic.DeepEqual(testMachineSet[0].Spec.Template.Spec.NodeTemplateSpec, machineDeployment.Spec.Template.Spec.NodeTemplateSpec)).To(BeTrue())
			Expect(testMachineSet[0].Annotations["deployment.kubernetes.io/revision"]).Should(Equal(newRevision))

		})

		It("when nodeTemplate is not updated in MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			testMachineSet := newMachineSets(
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
			)

			newRevision := "0"
			c, trackers := createController(stop, testNamespace, nil, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			nodeTemplateChanged := SetNewMachineSetNodeTemplate(
				machineDeployment,
				testMachineSet[0],
				newRevision,
				true,
			)

			Expect(nodeTemplateChanged).To(BeFalse())
			Expect(apiequality.Semantic.DeepEqual(testMachineSet[0].Spec.Template.Spec.NodeTemplateSpec, machineDeployment.Spec.Template.Spec.NodeTemplateSpec)).To(BeTrue())
		})
	})

	Describe("#copyMachineDeploymentNodeTemplatesToMachineSet", func() {
		It("when nodeTemplate is updated in MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			testMachineSet := newMachineSets(
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
			)

			c, trackers := createController(stop, testNamespace, nil, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			nodeTemplateChanged := copyMachineDeploymentNodeTemplatesToMachineSet(
				machineDeployment,
				testMachineSet[0],
			)

			Expect(nodeTemplateChanged).To(BeTrue())
			Expect(apiequality.Semantic.DeepEqual(testMachineSet[0].Spec.Template.Spec.NodeTemplateSpec, machineDeployment.Spec.Template.Spec.NodeTemplateSpec)).To(BeTrue())
		})

		It("when nodeTemplate is not updated in MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			testMachineSet := newMachineSets(
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
			)

			c, trackers := createController(stop, testNamespace, nil, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			nodeTemplateChanged := copyMachineDeploymentNodeTemplatesToMachineSet(
				machineDeployment,
				testMachineSet[0],
			)

			Expect(nodeTemplateChanged).To(BeFalse())
			Expect(apiequality.Semantic.DeepEqual(testMachineSet[0].Spec.Template.Spec.NodeTemplateSpec, machineDeployment.Spec.Template.Spec.NodeTemplateSpec)).To(BeTrue())
		})

	})

	Describe("#MergeStringMaps", func() {
		It("should merge maps correctly with no conflicts", func() {
			oldMap := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}
			newMap1 := map[string]string{
				"key3": "value3",
			}
			newMap2 := map[string]string{
				"key4": "value4",
			}

			expected := map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
				"key4": "value4",
			}

			result := MergeStringMaps(oldMap, newMap1, newMap2)
			Expect(result).To(Equal(expected))
		})

		It("should overwrite values from oldMap with values from newMaps", func() {
			oldMap := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}
			newMap1 := map[string]string{
				"key2": "newValue2",
			}
			newMap2 := map[string]string{
				"key1": "newValue1",
			}

			expected := map[string]string{
				"key1": "newValue1",
				"key2": "newValue2",
			}

			result := MergeStringMaps(oldMap, newMap1, newMap2)
			Expect(result).To(Equal(expected))
		})

		It("should handle nil oldMap correctly", func() {
			var oldMap map[string]string
			newMap1 := map[string]string{
				"key1": "value1",
			}
			newMap2 := map[string]string{
				"key2": "value2",
			}

			expected := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}

			result := MergeStringMaps(oldMap, newMap1, newMap2)
			Expect(result).To(Equal(expected))
		})

		It("should handle nil newMaps correctly", func() {
			oldMap := map[string]string{
				"key1": "value1",
			}
			var newMap1 map[string]string
			newMap2 := map[string]string{
				"key2": "value2",
			}

			expected := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}

			result := MergeStringMaps(oldMap, newMap1, newMap2)
			Expect(result).To(Equal(expected))
		})

		It("should return an empty map if all inputs are nil", func() {
			var oldMap map[string]string
			var newMap1 map[string]string
			var newMap2 map[string]string
			var expected map[string]string

			result := MergeStringMaps(oldMap, newMap1, newMap2)
			Expect(result).To(Equal(expected))
		})
	})

	Describe("#MergeWithOverwriteAndFilter", func() {
		It("should merge maps correctly with no conflicts", func() {
			current := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}
			oldIS := map[string]string{
				"key3": "value3",
			}
			newIS := map[string]string{
				"key4": "value4",
			}

			expected := map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key4": "value4",
			}

			result := MergeWithOverwriteAndFilter(current, oldIS, newIS)
			Expect(result).To(Equal(expected))
		})

		It("should overwrite values from current with values from newIS", func() {
			current := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}
			oldIS := map[string]string{
				"key3": "value3",
			}
			newIS := map[string]string{
				"key2": "newValue2",
			}

			expected := map[string]string{
				"key1": "value1",
				"key2": "newValue2",
			}

			result := MergeWithOverwriteAndFilter(current, oldIS, newIS)
			Expect(result).To(Equal(expected))
		})

		It("should not include keys from current that are present in oldIS", func() {
			current := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}
			oldIS := map[string]string{
				"key2": "value2",
			}
			newIS := map[string]string{
				"key3": "value3",
			}

			expected := map[string]string{
				"key1": "value1",
				"key3": "value3",
			}

			result := MergeWithOverwriteAndFilter(current, oldIS, newIS)
			Expect(result).To(Equal(expected))
		})

		It("should handle nil current map correctly", func() {
			var current map[string]string
			oldIS := map[string]string{
				"key1": "value1",
			}
			newIS := map[string]string{
				"key2": "value2",
			}

			expected := map[string]string{
				"key2": "value2",
			}

			result := MergeWithOverwriteAndFilter(current, oldIS, newIS)
			Expect(result).To(Equal(expected))
		})

		It("should handle nil oldIS map correctly", func() {
			current := map[string]string{
				"key1": "value1",
			}
			var oldIS map[string]string
			newIS := map[string]string{
				"key2": "value2",
			}

			expected := map[string]string{
				"key1": "value1",
				"key2": "value2",
			}

			result := MergeWithOverwriteAndFilter(current, oldIS, newIS)
			Expect(result).To(Equal(expected))
		})

		It("should handle nil newIS map correctly", func() {
			current := map[string]string{
				"key1": "value1",
			}
			oldIS := map[string]string{
				"key2": "value2",
			}
			var newIS map[string]string

			expected := map[string]string{
				"key1": "value1",
			}

			result := MergeWithOverwriteAndFilter(current, oldIS, newIS)
			Expect(result).To(Equal(expected))
		})

		It("should return an empty map if all inputs are nil", func() {
			var current map[string]string
			var oldIS map[string]string
			var newIS map[string]string

			result := MergeWithOverwriteAndFilter(current, oldIS, newIS)
			Expect(result).To(Equal(map[string]string{}))
		})
	})
})
