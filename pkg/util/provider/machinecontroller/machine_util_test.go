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
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
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
			err  error
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

				c, trackers := createController(stop, testNamespace, controlObjects, nil, coreObjects, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				_, err := c.syncMachineNodeTemplates(context.TODO(), machineObject)

				waitForCacheSync(stop, c)

				if data.expect.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(data.expect.err))
				}

				//updatedNodeObject, _ := c.nodeLister.Get(nodeObject.Name)
				updatedNodeObject, _ := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), nodeObject.Name, metav1.GetOptions{})

				if data.expect.node != nil {
					Expect(updatedNodeObject.Spec.Taints).Should(ConsistOf(data.expect.node.Spec.Taints))
					Expect(updatedNodeObject.Labels).Should(Equal(data.expect.node.Labels))

					// ignore LastAppliedALTAnnotataion
					delete(updatedNodeObject.Annotations, machineutils.LastAppliedALTAnnotation)
					Expect(updatedNodeObject.Annotations).Should(Equal(data.expect.node.Annotations))
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
											"anno1":                               "anno1",
											machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\"}}}}",
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
						nil, nil, nil, true, metav1.Now()),
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
					err: fmt.Errorf("Machine ALTs have been reconciled"),
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
											"anno1":                               "anno1",
											machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"},\"annotations\":{\"anno1\":\"anno1\"}}}",
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
						nil, nil, nil, true, metav1.Now()),
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
					err: fmt.Errorf("Machine ALTs have been reconciled"),
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
						nil, nil, nil, true, metav1.Now()),
				},
				action: action{
					node: &corev1.Node{},
				},
				expect: expect{
					node: &corev1.Node{},
					err:  nil, // we should not return error if node-object does not exist to ensure rest of the steps are then executed.
				},
			}),

			Entry("Multiple taints with same key and value added to taint", &data{
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
												Effect: "NoExecute",
											},
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
						nil, nil, nil, true, metav1.Now()),
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
								"anno1":                               "anno1",
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"},\"annotations\":{\"anno1\":\"anno1\"}}}",
							},
							Labels: map[string]string{
								"key1": "value1",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{},
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
									Effect: "NoExecute",
								},
								{
									Key:    "key1",
									Value:  "value1",
									Effect: "NoSchedule",
								},
							},
						},
					},
					err: fmt.Errorf("Machine ALTs have been reconciled"),
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

				c, trackers := createController(stop, testNamespace, nil, nil, nil, nil)
				defer trackers.Stop()

				waitForCacheSync(stop, c)

				testNode := data.action.node
				testMachine := data.action.machine
				expectedNode := data.expect.node

				var lastAppliedALT v1alpha1.NodeTemplateSpec
				lastAppliedALTJSONString, exists := testNode.Annotations[machineutils.LastAppliedALTAnnotation]
				if exists {
					err := json.Unmarshal([]byte(lastAppliedALTJSONString), &lastAppliedALT)
					if err != nil {
						klog.Errorf("Error occurred while syncing node annotations, labels & taints: %s", err)
					}
				}

				labelsChanged := SyncMachineLabels(testMachine, testNode, lastAppliedALT.Labels)

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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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

			Entry("when label is deleted from machine object", &data{
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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\", \"key2\":\"value2\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
					labelsChanged: true,
				},
			}),

			Entry("when labels values are updated manually on node object", &data{
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
								"key1": "value2",
							},
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
					labelsChanged: true,
				},
			}),

			Entry("when new labels are added on node-object ", &data{
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
								"key1":   "value1",
								"keyNew": "valueNew",
							},
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
								"key1":   "value1",
								"keyNew": "valueNew",
							},
						},
					},
					labelsChanged: false,
				},
			}),

			Entry("when existing labels are deleted from node-object ", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:   "test-node-0",
							Labels: map[string]string{},
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"labels\":{\"key1\":\"value1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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

				c, trackers := createController(stop, testNamespace, nil, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				testNode := data.action.node
				testMachine := data.action.machine
				expectedNode := data.expect.node

				var lastAppliedALT v1alpha1.NodeTemplateSpec
				lastAppliedALTJSONString, exists := testNode.Annotations[machineutils.LastAppliedALTAnnotation]
				if exists {
					err := json.Unmarshal([]byte(lastAppliedALTJSONString), &lastAppliedALT)
					if err != nil {
						klog.Errorf("Error occurred while syncing node annotations, labels & taints: %s", err)
					}
				}

				annotationsChanged := SyncMachineAnnotations(testMachine, testNode, lastAppliedALT.Annotations)

				waitForCacheSync(stop, c)

				// ignore machineutils.LastAppliedALTAnnotation for comparison
				delete(testNode.Annotations, machineutils.LastAppliedALTAnnotation)
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
								"anno1":                               "anno1",
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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

			Entry("when annotations values are updated ", &data{
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
								"anno1":                               "anno1",
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
								"anno1":                               "anno1",
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
								"anno1":                               "anno1",
								"anno2":                               "anno2",
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\", \"anno2\":\"anno2\"}}}",
							},
						},
					},
					machine: newMachine(
						&machinev1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{},
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
						nil, nil, nil, nil, true, metav1.Now()),
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
					annotationsChanged: true,
				},
			}),

			Entry("when annotations values are updated manually on node object", &data{
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
								"anno1":                               "anno2",
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
					annotationsChanged: true,
				},
			}),

			Entry("when new annotations are added on node-objects.", &data{
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
								"anno1":                               "anno1",
								"annoNew":                             "annoNew",
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
								"anno1":   "anno1",
								"annoNew": "annoNew",
							},
						},
					},
					annotationsChanged: false,
				},
			}),

			Entry("when existing annotations are deleted from node-objects. ", &data{
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
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null,\"annotations\":{\"anno1\":\"anno1\"}}}",
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
						nil, nil, nil, nil, true, metav1.Now()),
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

				c, trackers := createController(stop, testNamespace, nil, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				testNode := data.action.node
				testMachine := data.action.machine
				expectedNode := data.expect.node

				var lastAppliedALT v1alpha1.NodeTemplateSpec
				lastAppliedALTJSONString, exists := testNode.Annotations[machineutils.LastAppliedALTAnnotation]
				if exists {
					err := json.Unmarshal([]byte(lastAppliedALTJSONString), &lastAppliedALT)
					if err != nil {
						klog.Errorf("Error occurred while syncing node annotations, labels & taints: %s", err)
					}
				}

				taintsChanged := SyncMachineTaints(testMachine, testNode, lastAppliedALT.Spec.Taints)

				waitForCacheSync(stop, c)

				Expect(testNode.Spec.Taints).Should(ConsistOf(expectedNode.Spec.Taints))
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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"taints\":[{\"key\":\"Key1\",\"value\":\"Value1\",\"effect\":\"NoSchedule\"}]}}",
							},
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
						nil, nil, nil, nil, true, metav1.Now()),
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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"taints\":[{\"key\":\"Key1\",\"value\":\"OldValue\",\"effect\":\"NoSchedule\"}]}}",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "OldValue",
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
												Value:  "NewValue",
												Effect: "NoSchedule",
											},
										},
									},
								},
							},
						},
						nil, nil, nil, nil, true, metav1.Now()),
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
									Value:  "NewValue",
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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"taints\":[{\"key\":\"Key1\",\"value\":\"Value1\",\"effect\":\"NoSchedule\"}]}}",
							},
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
						nil, nil, nil, nil, true, metav1.Now()),
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
							Annotations: map[string]string{
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"taints\":[{\"key\":\"Key1\",\"value\":\"Value1\",\"effect\":\"NoSchedule\"},{\"key\":\"Key2\",\"value\":\"Value2\",\"effect\":\"NoSchedule\"}]}}",
							},
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
						nil, nil, nil, nil, true, metav1.Now()),
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
					taintsChanged: true,
				},
			}),

			Entry("when node taint value is overwritten manually & new taint was added with same key & value but different effect", &data{
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
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"taints\":[{\"key\":\"Key1\",\"value\":\"Value1\",\"effect\":\"NoSchedule\"}]}}",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
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
											{
												Key:    "Key1",
												Value:  "Value1",
												Effect: "NoExecute",
											},
										},
									},
								},
							},
						},
						nil, nil, nil, nil, true, metav1.Now()),
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
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoExecute",
								},
							},
						},
					},
					taintsChanged: true,
				},
			}),

			Entry("when new taints are added on node-object ", &data{
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
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"taints\":[{\"key\":\"Key1\",\"value\":\"Value1\",\"effect\":\"NoSchedule\"}]}}",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    "Key1",
									Value:  "Value1",
									Effect: "NoSchedule",
								},
								{
									Key:    "KeyNew",
									Value:  "ValueNew",
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
						nil, nil, nil, nil, true, metav1.Now()),
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
									Key:    "KeyNew",
									Value:  "ValueNew",
									Effect: "NoSchedule",
								},
							},
						},
					},
					taintsChanged: false,
				},
			}),

			Entry("when existing taints are deleted from node-objects ", &data{
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
								machineutils.LastAppliedALTAnnotation: "{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"taints\":[{\"key\":\"Key1\",\"value\":\"Value1\",\"effect\":\"NoSchedule\"}]}}",
							},
						},
						Spec: corev1.NodeSpec{
							Taints: []corev1.Taint{},
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
						nil, nil, nil, nil, true, metav1.Now()),
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
		)

	})

	Describe("#isMachineStatusSimilar", func() {

		type setup struct {
			m1, m2 machinev1.MachineStatus
		}

		type expect struct {
			equal bool
		}
		type data struct {
			setup setup

			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				equal := isMachineStatusSimilar(data.setup.m1, data.setup.m2)
				Expect(equal).To(Equal(data.expect.equal))
			},
			Entry("when status description is exact match", &data{
				setup: setup{
					m1: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.Now(),
						},
					},
					m2: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.Now(),
						},
					},
				},
				expect: expect{
					equal: true,
				},
			}),
			Entry("when status description is exact match but timeout has occurred", &data{
				setup: setup{
					m1: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.Now(),
						},
					},
					m2: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour)),
						},
					},
				},
				expect: expect{
					equal: false,
				},
			}),
			Entry("when status description is exact match but timeout has not occurred", &data{
				setup: setup{
					m1: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.Now(),
						},
					},
					m2: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Minute)),
						},
					},
				},
				expect: expect{
					equal: true,
				},
			}),
			Entry("when status description is long similar", &data{
				setup: setup{
					m1: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.Now(),
						},
					},
					m2: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "Error occurred with decoding machine error status while getting VM status, aborting without retry. gRPC code: AuthFailure: AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: dsfddsfd-dsfdfd-dsfdsfds-dsfdkjljl32 Set machine status to termination. Now, getting VM Status",
							LastUpdateTime: metav1.Now(),
						},
					},
				},
				expect: expect{
					equal: true,
				},
			}),
			Entry("when status description is short similar", &data{
				setup: setup{
					m1: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: fbc513f0-ca27-4a42-a203-42048d5773a2",
							LastUpdateTime: metav1.Now(),
						},
					},
					m2: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    "AWS was not able to validate the provided access credentials\n\tstatus code: 401, request id: 23423424-342f-sdff-45cs-3242sdfsdfd",
							LastUpdateTime: metav1.Now(),
						},
					},
				},
				expect: expect{
					equal: true,
				},
			}),
			Entry("when status description is different", &data{
				setup: setup{
					m1: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    machineutils.InitiateNodeDeletion,
							LastUpdateTime: metav1.Now(),
						},
					},
					m2: machinev1.MachineStatus{
						LastOperation: machinev1.LastOperation{
							Description:    machineutils.InitiateFinalizerRemoval,
							LastUpdateTime: metav1.Now(),
						},
					},
				},
				expect: expect{
					equal: false,
				},
			}),
		)

	})

})
