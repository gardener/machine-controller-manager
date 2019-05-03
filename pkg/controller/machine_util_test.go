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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("machine_util", func() {

	Describe("#syncMachineNodeTemplates", func() {

		type setup struct {
			machine *machinev1.Machine
		}
		type action struct {
			node *corev1.Node
		}
		type expect struct {
			node *corev1.Node
			err  bool
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlObjects := []runtime.Object{}
				coreObjects := []runtime.Object{}

				machineObject := data.setup.machine

				nodeObject := data.action.node
				coreObjects = append(coreObjects, nodeObject)
				controlObjects = append(controlObjects, machineObject)

				c, trackers := createController(stop, testNamespace, controlObjects, nil, coreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				err := c.syncMachineNodeTemplates(machineObject)

				waitForCacheSync(stop, c)

				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				//updatedNodeObject, _ := c.nodeLister.Get(nodeObject.Name)
				updatedNodeObject, _ := c.targetCoreClient.Core().Nodes().Get(nodeObject.Name, metav1.GetOptions{})

				if data.expect.node != nil {
					Expect(updatedNodeObject.Spec.Taints).Should(Equal(data.expect.node.Spec.Taints))
					Expect(updatedNodeObject.Annotations).Should(Equal(data.expect.node.Annotations))
					Expect(updatedNodeObject.Labels).Should(Equal(data.expect.node.Labels))
				}
			},

			Entry("when nodeTemplate is not updated in node-object", &data{
				setup: setup{
					machine: newMachine(
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
											Key:    "key1",
											Value:  "value1",
											Effect: "NoSchedule",
										},
										},
									},
								},
							},
						},
						&machinev1.MachineStatus{
							Node: "test-node",
						},
						nil, nil, nil),
				},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
							},
							Labels: map[string]string{
								"key1": "value1",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "key1",
									Value:  "value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-node-0",
							Namespace: testNamespace,
							Annotations: map[string]string{
								"anno1": "anno1",
							},
							Labels: map[string]string{
								"key1": "value1",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "key1",
									Value:  "value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
					err: false,
				},
			}),

			Entry("when nodeTemplate is updated in node-object", &data{
				setup: setup{
					machine: newMachine(
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
										Taints: []corev1.Taint{
											{
												Key:    "key1",
												Value:  "value1",
												Effect: "NoSchedule",
											},
										},
									},
								},
							},
						},
						&machinev1.MachineStatus{
							Node: "test-node",
						},
						nil, nil, nil),
				},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{},
					},
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
							},
							Annotations: map[string]string{
								"anno1": "anno1",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "key1",
									Value:  "value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
					err: false,
				},
			}),

			Entry("when node object does not exist", &data{
				setup: setup{
					machine: newMachine(
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
										Taints: []corev1.Taint{
											{
												Key:    "key1",
												Value:  "value1",
												Effect: "NoSchedule",
											},
										},
									},
								},
							},
						},
						&machinev1.MachineStatus{
							Node: "test-node",
						},
						nil, nil, nil),
				},
				action: action{
					node: &corev1.Node{},
				},
				expect: expect{
					node: &corev1.Node{},
					err:  false, // we should not return error if node-object does not exist to ensure rest of the steps are then executed.
				},
			}),
		)

	})

	Describe("#SyncMachineLabels", func() {

		type setup struct{}
		type action struct {
			node    *corev1.Node
			machine *machinev1.Machine
		}
		type expect struct {
			node          *corev1.Node
			labelsChanged bool
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				c, trackers := createController(stop, testNamespace, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				testNode := data.action.node
				testMachine := data.action.machine
				expectedNode := data.expect.node
				labelsChanged := SyncMachineLabels(testMachine, testNode)

				waitForCacheSync(stop, c)

				Expect(testNode.Labels).Should(Equal(expectedNode.Labels))
				Expect(labelsChanged).To(Equal(data.expect.labelsChanged))
			},

			Entry("when labels have not been updated", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"key1": "value1",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
							},
						},
					},
					labelsChanged: false,
				},
			}),

			Entry("when labels values are updated ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"key1": "valueChanged",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "valueChanged",
							},
						},
					},
					labelsChanged: true,
				},
			}),

			Entry("when new label keys are added ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"key1": "value1",
											"key2": "value2",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
					labelsChanged: true,
				},
			}),

			Entry("when label keys are deleted ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"key1": "value1",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Labels: map[string]string{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
					labelsChanged: false,
				},
			}),
		)

	})

	Describe("#SyncMachineAnnotations", func() {

		type setup struct{}
		type action struct {
			node    *corev1.Node
			machine *machinev1.Machine
		}
		type expect struct {
			node               *corev1.Node
			annotationsChanged bool
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				c, trackers := createController(stop, testNamespace, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				testNode := data.action.node
				testMachine := data.action.machine
				expectedNode := data.expect.node
				annotationsChanged := SyncMachineAnnotations(testMachine, testNode)

				waitForCacheSync(stop, c)

				Expect(testNode.Annotations).Should(Equal(expectedNode.Annotations))
				Expect(annotationsChanged).To(Equal(data.expect.annotationsChanged))
			},

			Entry("when annotations have not been updated", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Annotations: map[string]string{
											"anno1": "anno1",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
							},
						},
					},
					annotationsChanged: false,
				},
			}),

			Entry("when annoatations values are updated ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Annotations: map[string]string{
											"anno1": "annoChanged",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "annoChanged",
							},
						},
					},
					annotationsChanged: true,
				},
			}),

			Entry("when new annotation keys are added ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Annotations: map[string]string{
											"anno1": "anno1",
											"anno2": "anno2",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
								"anno2": "anno2",
							},
						},
					},
					annotationsChanged: true,
				},
			}),

			Entry("when annotations are deleted ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
								"anno2": "anno2",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"anno1": "anno1",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Annotations: map[string]string{
											"anno1": "anno1",
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "anno1",
								"anno2": "anno2",
							},
						},
					},
					annotationsChanged: false,
				},
			}),
		)

	})

	Describe("#SyncMachineTaints", func() {

		type setup struct{}
		type action struct {
			node    *corev1.Node
			machine *machinev1.Machine
		}
		type expect struct {
			node          *corev1.Node
			taintsChanged bool
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				c, trackers := createController(stop, testNamespace, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				testNode := data.action.node
				testMachine := data.action.machine
				expectedNode := data.expect.node
				taintsChanged := SyncMachineTaints(testMachine, testNode)

				waitForCacheSync(stop, c)

				Expect(testNode.Spec.Taints).Should(Equal(expectedNode.Spec.Taints))
				Expect(taintsChanged).To(Equal(data.expect.taintsChanged))
			},

			Entry("when taints have not been updated", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									Spec: corev1.NodeSpec{
										Taints: []corev1.Taint{
											{
												Key:    "Key1",
												Value:  "Value1",
												Effect: "NoSchedule",
											},
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
					taintsChanged: false,
				},
			}),

			Entry("when taints values are updated ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									Spec: corev1.NodeSpec{
										Taints: []corev1.Taint{
											{
												Key:    "Key1",
												Value:  "ValueChanged",
												Effect: "NoSchedule",
											},
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "ValueChanged",
									Effect: "NoSchedule",
								},
							},
						},
					},
					taintsChanged: true,
				},
			}),

			Entry("when new taints are added ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									Spec: corev1.NodeSpec{
										Taints: []corev1.Taint{
											{
												Key:    "Key1",
												Value:  "Value1",
												Effect: "NoSchedule",
											},
											{
												Key:    "Key2",
												Value:  "Value2",
												Effect: "NoSchedule",
											},
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
								{
									Key:    "Key2",
									Value:  "Value2",
									Effect: "NoSchedule",
								},
							},
						},
					},
					taintsChanged: true,
				},
			}),

			Entry("when taints are deleted ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
								{
									Key:    "Key2",
									Value:  "Value2",
									Effect: "NoSchedule",
								},
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-label": "test-label",
								},
							},
							Spec: machinev1.MachineSpec{
								NodeTemplateSpec: machinev1.NodeTemplateSpec{
									Spec: corev1.NodeSpec{
										Taints: []corev1.Taint{
											{
												Key:    "Key1",
												Value:  "Value1",
												Effect: "NoSchedule",
											},
										},
									},
								},
							},
						},
						nil, nil, nil, nil),
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
								{
									Key:    "Key2",
									Value:  "Value2",
									Effect: "NoSchedule",
								},
							},
						},
					},
					taintsChanged: false,
				},
			}),
		)

	})
})
