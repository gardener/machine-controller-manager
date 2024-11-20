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

	DescribeTable("#NewISNewReplicas",
		func(strategy machinev1.MachineDeploymentStrategy, replicas int, oldISReplicas int, newISReplicas int, expected int, shouldError bool) {
			// Setup MachineDeployment and MachineSets
			deployment := &machinev1.MachineDeployment{
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: int32(replicas),
					Strategy: strategy,
				},
			}
			allISs := []*machinev1.MachineSet{
				{Spec: machinev1.MachineSetSpec{Replicas: int32(oldISReplicas)}, Status: machinev1.MachineSetStatus{Replicas: int32(oldISReplicas)}},
				{Spec: machinev1.MachineSetSpec{Replicas: int32(newISReplicas)}, Status: machinev1.MachineSetStatus{Replicas: int32(newISReplicas)}},
			}
			newIS := &machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{Replicas: int32(newISReplicas)},
			}

			result, err := NewISNewReplicas(deployment, allISs, newIS)

			if shouldError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(int32(expected)))
			}
		},

		// Case: RollingUpdate strategy, scaling up within max surge
		Entry("RollingUpdate, scale up within max surge",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge: ptr.To(intstr.FromInt(2)),
					},
				},
			}, 5, 4, 2, 3, false,
		),

		// Case: RollingUpdate strategy, max surge already reached
		Entry("RollingUpdate, max surge reached",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge: ptr.To(intstr.FromInt(2)),
					},
				},
			}, 5, 4, 3, 3, false,
		),

		// Case: Recreate strategy
		Entry("Recreate strategy",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.RecreateMachineDeploymentStrategyType,
			}, 5, 4, 2, 5, false,
		),

		// Case: InPlaceUpdate strategy with ManualUpdate=true
		Entry("InPlaceUpdate with ManualUpdate=true",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
					OrchestrationType: machinev1.OrchestrationTypeManual,
				},
			}, 5, 4, 2, 0, false,
		),

		// Case: InPlaceUpdate strategy with ManualUpdate=false, scaling up within max surge
		Entry("RollingUpdate, scale up within max surge",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge: ptr.To(intstr.FromInt(2)),
					},
					OrchestrationType: machinev1.OrchestrationTypeAuto,
				},
			}, 5, 4, 2, 3, false,
		),

		// Case: InPlaceUpdate strategy with ManualUpdate=false and max surge already reached
		Entry("InPlaceUpdate with ManualUpdate=false, max surge already reached",
			machinev1.MachineDeploymentStrategy{
				Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
				InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
					UpdateConfiguration: machinev1.UpdateConfiguration{
						MaxSurge: ptr.To(intstr.FromInt(1)),
					},
					OrchestrationType: machinev1.OrchestrationTypeAuto,
				},
			}, 5, 4, 2, 2, false,
		),

		// Case: Unsupported strategy type
		Entry("Unsupported strategy type",
			machinev1.MachineDeploymentStrategy{
				Type: "UnsupportedType",
			}, 5, 4, 2, 0, true,
		),
	)

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
})
