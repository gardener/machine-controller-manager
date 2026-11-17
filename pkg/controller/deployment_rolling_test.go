// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/controller/autoscaler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var _ = Describe("deployment_rolling", func() {
	Describe("#taintNodesBackingMachineSets", func() {
		type setup struct {
			nodes       []*corev1.Node
			machineSets []*machinev1.MachineSet
			machines    []*machinev1.Machine
		}
		type expect struct {
			nodes       []*corev1.Node
			machineSets []*machinev1.MachineSet
			err         bool
		}
		type data struct {
			setup  setup
			action []*machinev1.MachineSet
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			Namespace: testNamespace,
		}
		machineSets := newMachineSets(
			1,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
					Labels: map[string]string{
						"key": "value",
					},
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "OpenStackMachineClass",
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

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				for _, o := range data.setup.machineSets {
					controlMachineObjects = append(controlMachineObjects, o)
				}
				for _, o := range data.setup.machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range data.setup.nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				err := controller.taintNodesBackingMachineSets(
					context.TODO(),
					data.action,
					&corev1.Taint{
						Key:    PreferNoScheduleKey,
						Value:  "True",
						Effect: "PreferNoSchedule",
					},
				)
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				for _, expectedMachineSet := range data.expect.machineSets {
					actualMachineSet, err := controller.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), expectedMachineSet.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualMachineSet.Annotations).Should(Equal(expectedMachineSet.Annotations))
				}

				for _, expectedNode := range data.expect.nodes {
					actualNode, err := controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), expectedNode.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualNode.Spec.Taints).Should(ConsistOf(expectedNode.Spec.Taints))
				}
			},
			Entry("taints on nodes backing machineSet", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "OpenStackMachineClass",
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
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
									Name: "test-machine-class",
								},
							},
						},
						3,
						500,
						nil,
						nil,
						map[string]string{
							PreferNoScheduleKey: "True",
						},
						nil,
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    PreferNoScheduleKey,
									Value:  "True",
									Effect: "PreferNoSchedule",
								},
							},
						},
						nil,
					),
					err: false,
				},
			}),
		)
	})

	Describe("#annotateNodesBackingMachineSets", func() {
		type setup struct {
			nodes               []*corev1.Node
			machineSets         []*machinev1.MachineSet
			machines            []*machinev1.Machine
			existingAnnotations map[string]string
		}
		type expect struct {
			nodeAnnotations map[string]string
			err             bool
		}
		type action struct {
			machineSets []*machinev1.MachineSet
			annotations map[string]string
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			Namespace: testNamespace,
		}
		machineSets := newMachineSets(
			1,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
					Labels: map[string]string{
						"key": "value",
					},
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "OpenStackMachineClass",
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

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				for _, o := range data.setup.machineSets {
					controlMachineObjects = append(controlMachineObjects, o)
				}
				for _, o := range data.setup.machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range data.setup.nodes {
					o.Annotations = data.setup.existingAnnotations
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				err := controller.annotateNodesBackingMachineSets(
					context.TODO(),
					data.action.machineSets,
					data.action.annotations,
				)
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				for _, expectedNode := range data.setup.nodes {
					actualNode, err := controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), expectedNode.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualNode.Annotations).Should(Equal(data.expect.nodeAnnotations))
				}
			},
			Entry("annotate nodes backing machineSets when there are annotations present", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					existingAnnotations: map[string]string{
						"anno1": "anno1",
					},
				},
				action: action{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
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
					annotations: map[string]string{
						"anno2": "anno2",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "anno2",
					},
					err: false,
				},
			}),
			Entry("annotate nodes backing machineSets when there are not other annotations", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					existingAnnotations: map[string]string{
						"anno1": "anno1",
					},
				},
				action: action{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
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
					annotations: map[string]string{
						"anno1": "anno1",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
					},
					err: false,
				},
			}),
			Entry("annotate nodes backing machineSets when there are same annotations but different value", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					existingAnnotations: map[string]string{
						"anno1": "annoDummy",
					},
				},
				action: action{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
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
					annotations: map[string]string{
						"anno1": "anno1",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
					},
					err: false,
				},
			}),
			Entry("when there are no nodes backiing the machinesets", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						0,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					existingAnnotations: map[string]string{
						"anno2": "anno2",
					},
				},
				action: action{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
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
					annotations: map[string]string{
						"anno1": "anno1",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "anno2",
					},
					err: false,
				},
			}),
		)
	})

	Describe("#removeAutoscalerAnnotationsIfRequired", func() {
		type setup struct {
			nodes               []*corev1.Node
			machineSets         []*machinev1.MachineSet
			machines            []*machinev1.Machine
			existingAnnotations map[string]string
		}
		type expect struct {
			nodeAnnotations map[string]string
			err             bool
		}
		type action struct {
			machineSets []*machinev1.MachineSet
			annotations map[string]string
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			Namespace: testNamespace,
		}
		machineSets := newMachineSets(
			1,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
					Labels: map[string]string{
						"key": "value",
					},
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "OpenStackMachineClass",
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

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				for _, o := range data.setup.machineSets {
					controlMachineObjects = append(controlMachineObjects, o)
				}
				for _, o := range data.setup.machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range data.setup.nodes {
					o.Annotations = data.setup.existingAnnotations
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				err := controller.removeAutoscalerAnnotationsIfRequired(
					context.TODO(),
					data.action.machineSets,
					data.action.annotations,
				)
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				for _, expectedNode := range data.setup.nodes {
					actualNode, err := controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), expectedNode.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualNode.Annotations).Should(Equal(data.expect.nodeAnnotations))
				}
			},
			Entry("remove autoscaler annotation when by-mcm annotation is present", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					existingAnnotations: map[string]string{
						"anno1": "anno1",
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey: autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue,
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey:      autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue,
					},
				},
				action: action{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
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
					annotations: map[string]string{
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey: autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue,
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey:      autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue,
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
					},
					err: false,
				},
			}),
			Entry("do not remove autoscaler annotation when by-mcm annotation is not present", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					existingAnnotations: map[string]string{
						"anno1": "anno1",
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey: autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue,
					},
				},
				action: action{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
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
					annotations: map[string]string{
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey: autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue,
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey:      autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue,
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey: autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue,
					},
					err: false,
				},
			}),
			Entry("when there are no autoscaler annotations present", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						nil,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					existingAnnotations: map[string]string{
						"anno1": "anno1",
					},
				},
				action: action{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
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
					annotations: map[string]string{
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey: autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue,
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey:      autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue,
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
					},
					err: false,
				},
			}),
		)
	})

	Describe("#reconcileOldMachineSets with a maintenance machine", func() {
		var (
			specTemplate  *machinev1.MachineTemplateSpec
			oldMachineSet *machinev1.MachineSet
			newMachineSet *machinev1.MachineSet
			deployment    *machinev1.MachineDeployment
			machineLabels map[string]string
		)

		BeforeEach(func() {
			machineLabels = map[string]string{"test-label": "test-label"}
			specTemplate = &machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: machineLabels},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{Kind: "MachineClass", Name: "test-machine-class"},
				},
			}
			oldMachineSet = newMachineSets(1, specTemplate, 1, 0,
				&machinev1.MachineSetStatus{Replicas: 1, AvailableReplicas: 0, MaintenanceReplicas: 1}, nil, nil, machineLabels)[0]
			newMachineSet = newMachineSets(1, specTemplate, 2, 0,
				&machinev1.MachineSetStatus{Replicas: 2, AvailableReplicas: 2}, nil, nil, machineLabels)[0]
			newMachineSet.Name = "machineset-new"

			deployment = &machinev1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "deployment", Namespace: testNamespace},
				Spec: machinev1.MachineDeploymentSpec{
					Replicas: 3,
					Strategy: machinev1.MachineDeploymentStrategy{
						Type: machinev1.RollingUpdateMachineDeploymentStrategyType,
						RollingUpdate: &machinev1.RollingUpdateMachineDeployment{
							UpdateConfiguration: machinev1.UpdateConfiguration{
								MaxUnavailable: new(intstr.FromInt32(1)),
								MaxSurge:       new(intstr.FromInt32(1)),
							},
						},
					},
				},
			}
		})

		It("should not scale down the old MachineSet backing a node under planned maintenance", func(ctx SpecContext) {
			maintenanceMachine := newMachinesFromMachineSet(1, oldMachineSet,
				&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineMaintenance}}, nil, nil)[0]

			controlMachineObjects := []runtime.Object{oldMachineSet, newMachineSet, maintenanceMachine}
			c, trackers := createController(ctx.Done(), testNamespace, controlMachineObjects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(ctx.Done(), c)

			allISs := []*machinev1.MachineSet{oldMachineSet, newMachineSet}
			scaledDown, err := c.reconcileOldMachineSets(ctx, allISs, []*machinev1.MachineSet{oldMachineSet}, newMachineSet, deployment)
			Expect(err).ToNot(HaveOccurred())
			Expect(scaledDown).To(BeFalse(), "old MachineSet must not be scaled down while it backs a maintenance node")

			updatedOldIS, err := c.controlMachineClient.MachineSets(testNamespace).Get(ctx, oldMachineSet.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedOldIS.Spec.Replicas).To(Equal(int32(1)), "old MachineSet replicas must stay at 1 to retain the maintenance machine")
		})

		It("should not scale a mixed old MachineSet below its maintenance-replica count", func(ctx SpecContext) {
			oldMachineSet.Spec.Replicas = 2
			oldMachineSet.Status = machinev1.MachineSetStatus{Replicas: 2, AvailableReplicas: 1, MaintenanceReplicas: 1}
			// Allow both replicas to be removed so the test exercises the maintenance limit.
			deployment.Spec.Strategy.RollingUpdate.MaxUnavailable = ptr.To(intstr.FromInt32(2))

			controlMachineObjects := []runtime.Object{oldMachineSet, newMachineSet}
			c, trackers := createController(ctx.Done(), testNamespace, controlMachineObjects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(ctx.Done(), c)

			allISs := []*machinev1.MachineSet{oldMachineSet, newMachineSet}
			scaledDown, err := c.scaleDownOldMachineSetsForRollingUpdate(ctx, allISs, []*machinev1.MachineSet{oldMachineSet}, deployment)
			Expect(err).ToNot(HaveOccurred())
			Expect(scaledDown).To(Equal(int32(1)), "only the available replica may be scaled down, the maintenance one must stay")

			updatedOldIS, err := c.controlMachineClient.MachineSets(testNamespace).Get(ctx, oldMachineSet.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedOldIS.Spec.Replicas).To(Equal(int32(1)), "old MachineSet replicas must stay at 1 to retain the maintenance machine")
		})
	})
})
