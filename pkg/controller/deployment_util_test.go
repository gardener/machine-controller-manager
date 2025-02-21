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
