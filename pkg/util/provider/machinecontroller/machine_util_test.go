// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/permits"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

const (
	machineDeploy1     = "shoot--testProject--testShoot-testWorkerPool-z1"
	machineDeploy2     = "shoot--testProject--testShoot-testWorkerPool-z2"
	machineSet1Deploy1 = "shoot--testProject--testShoot-testWorkerPool-z1-mset1"
	machineSet2Deploy1 = "shoot--testProject--testShoot-testWorkerPool-z1-mset2"
	machineSet1Deploy2 = "shoot--testProject--testShoot-testWorkerPool-z2-mset1"
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

				// updatedNodeObject, _ := c.nodeLister.Get(nodeObject.Name)
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
						&machinev1.MachineStatus{},
						nil, nil, map[string]string{machinev1.NodeLabelKey: "test-node-0"}, true, metav1.Now()),
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
					err: errSuccessfulALTsync,
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
						&machinev1.MachineStatus{},
						nil, nil, map[string]string{machinev1.NodeLabelKey: "test-node-0"}, true, metav1.Now()),
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
					err: errSuccessfulALTsync,
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
						&machinev1.MachineStatus{},
						nil, nil, map[string]string{machinev1.NodeLabelKey: "test-node-0"}, true, metav1.Now()),
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
						&machinev1.MachineStatus{},
						nil, nil, map[string]string{machinev1.NodeLabelKey: "test-node-0"}, true, metav1.Now()),
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
					err: errSuccessfulALTsync,
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

				var lastAppliedALT machinev1.NodeTemplateSpec
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

				var lastAppliedALT machinev1.NodeTemplateSpec
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

				var lastAppliedALT machinev1.NodeTemplateSpec
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

	Describe("#validateNodeTemplate", func() {
		type setup struct {
			machineClass *machinev1.MachineClass
		}

		type expect struct {
			err error
		}

		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlObjects := []runtime.Object{}

				machineClassObject := data.setup.machineClass

				controlObjects = append(controlObjects, machineClassObject)

				c, trackers := createController(stop, testNamespace, controlObjects, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				err := c.validateNodeTemplate(machineClassObject.NodeTemplate)

				if data.expect.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(data.expect.err))
				}
			},
			Entry("MachineClass with proper nodetemplate field", &data{
				setup: setup{
					machineClass: &machinev1.MachineClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-machine-class",
							Namespace: testNamespace,
						},
						NodeTemplate: &machinev1.NodeTemplate{
							Capacity: corev1.ResourceList{
								"cpu":    resource.MustParse("2"),
								"gpu":    resource.MustParse("2"),
								"memory": resource.MustParse("512Gi"),
							},
							Region:       "test-region",
							Zone:         "test-zone",
							InstanceType: "test-instance-type",
						},
					},
				},
				expect: expect{
					err: nil,
				},
			}),
			Entry("MachineClass with improper capacity in nodetemplate attribute", &data{
				setup: setup{
					machineClass: &machinev1.MachineClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-machine-class",
							Namespace: testNamespace,
						},
						NodeTemplate: &machinev1.NodeTemplate{
							Capacity: corev1.ResourceList{
								"gpu":    resource.MustParse("2"),
								"memory": resource.MustParse("512Gi"),
							},
							Region:       "test-region",
							Zone:         "test-zone",
							InstanceType: "test-instance-type",
						},
					},
				},
				expect: expect{
					err: errors.New("[MachineClass NodeTemplate Capacity should mandatorily have CPU, GPU and Memory configured]"),
				},
			}),
			Entry("MachineClass with missing region in nodetemplate attribute", &data{
				setup: setup{
					machineClass: &machinev1.MachineClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-machine-class",
							Namespace: testNamespace,
						},
						NodeTemplate: &machinev1.NodeTemplate{
							Capacity: corev1.ResourceList{
								"cpu":    resource.MustParse("2"),
								"gpu":    resource.MustParse("2"),
								"memory": resource.MustParse("512Gi"),
							},
							Zone:         "test-zone",
							InstanceType: "test-instance-type",
						},
					},
				},
				expect: expect{
					err: errors.New("[MachineClass NodeTemplate Instance Type, region and zone cannot be empty]"),
				},
			}),
		)
	})

	Describe("#reconcileMachineHealth", func() {
		var c *controller
		var trackers *fakeclient.FakeObjectTrackers

		type setup struct {
			machines []*machinev1.Machine
			nodes    []*corev1.Node
			// targetMachineName is name of machine for which `reconcileMachineHealth()` needs to be called
			// Its must this machine in machine list
			targetMachineName string
			// to check case when lock can't be acquired for long time
			lockAlreadyAcquired bool
		}
		type expect struct {
			retryPeriod   machineutils.RetryPeriod
			err           error
			expectedPhase machinev1.MachinePhase
		}
		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##General Machine Health Reconciliation", func(data *data) {
			stop := make(chan struct{})
			defer close(stop)

			controlMachineObjects := []runtime.Object{}
			targetCoreObjects := []runtime.Object{}

			var targetMachine *machinev1.Machine

			for _, o := range data.setup.machines {
				controlMachineObjects = append(controlMachineObjects, o)
				if o.Name == data.setup.targetMachineName {
					targetMachine = o
				}
			}

			for _, o := range data.setup.nodes {
				targetCoreObjects = append(targetCoreObjects, o)
			}

			c, trackers = createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil)
			defer trackers.Stop()

			c.permitGiver = permits.NewPermitGiver(5*time.Second, 1*time.Second)
			defer c.permitGiver.Close()

			waitForCacheSync(stop, c)

			Expect(targetMachine).ToNot(BeNil())

			retryPeriod, err := c.reconcileMachineHealth(context.TODO(), targetMachine)

			Expect(retryPeriod).To(Equal(data.expect.retryPeriod))

			if data.expect.err == nil {
				Expect(err).To(BeNil())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(data.expect.err))
			}

			updatedTargetMachine, getErr := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), targetMachine.Name, metav1.GetOptions{})
			Expect(getErr).To(BeNil())
			Expect(data.expect.expectedPhase).To(Equal(updatedTargetMachine.Status.CurrentStatus.Phase))
		},
			Entry("simple machine with creation Timeout(20 min)", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachinePending, LastUpdateTime: metav1.NewTime(time.Now().Add(-25 * time.Minute))}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0-0"}, true, metav1.Now()),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineFailed,
				},
			}),
			Entry("Unknown machine if Healthy should be marked Running", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.Now()}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(true, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineRunning,
				},
			}),
			Entry("Running machine if Unhealthy due to kubelet not posting status should be marked Unknown", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineRunning, LastUpdateTime: metav1.Now()}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("Running machine if Unhealthy due to other relevant Node conditions, should be marked Unknown", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineRunning, LastUpdateTime: metav1.Now()}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(true, true, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("Running machine should be marked Unknown if node object not found", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineRunning, LastUpdateTime: metav1.Now()}, Conditions: nodeConditions(true, false, false, false, false)},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("Machine in Unknown state with node obj for over 10min(healthTimeout) should be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					expectedPhase: machinev1.MachineFailed,
				},
			}),
			Entry("Machine in Unknown state WITHOUT backing node obj for over 10min(healthTimeout) should be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					expectedPhase: machinev1.MachineFailed,
				},
			}),
			Entry("simple machine WITHOUT creation Timeout(20 min) and Pending state", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(true, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachinePending, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(true, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.LongRetry,
					err:           nil,
					expectedPhase: machinev1.MachinePending,
				},
			}),
			Entry("pending machine is healthy, but node has `critical-components-not-ready` taint, shouldn't be marked Running", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newHealthyMachine(machineSet1Deploy1, "node-0", machinev1.MachinePending),
					},
					nodes: []*corev1.Node{
						newNode(
							1,
							nil,
							nil,
							&corev1.NodeSpec{
								Taints: []corev1.Taint{
									{
										Key:    "node.gardener.cloud/critical-components-not-ready",
										Effect: corev1.TaintEffectNoSchedule,
									},
								},
							},
							&corev1.NodeStatus{
								Phase:      corev1.NodeRunning,
								Conditions: nodeConditions(true, false, false, false, false),
							},
						),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.LongRetry,
					err:           nil,
					expectedPhase: machinev1.MachinePending,
				},
			}),
			Entry("pending machine is healthy, and node doesn't have `critical-components-not-ready taint`, should be marked Running", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newHealthyMachine(machineSet1Deploy1, "node-0", machinev1.MachinePending),
					},
					nodes: []*corev1.Node{
						newNode(
							1,
							nil,
							nil,
							&corev1.NodeSpec{
								Taints: []corev1.Taint{},
							},
							&corev1.NodeStatus{
								Phase:      corev1.NodeRunning,
								Conditions: nodeConditions(true, false, false, false, false),
							},
						),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineRunning,
				},
			}),
			Entry("unknown machine is healthy, and node doesn't have `critical-components-not-ready` taint, should be marked Running", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newHealthyMachine(machineSet1Deploy1, "node-0", machinev1.MachineUnknown),
					},
					nodes: []*corev1.Node{
						newNode(
							1,
							nil,
							nil,
							&corev1.NodeSpec{
								Taints: []corev1.Taint{},
							},
							&corev1.NodeStatus{
								Phase:      corev1.NodeRunning,
								Conditions: nodeConditions(true, false, false, false, false),
							},
						),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineRunning,
				},
			}),
			// the following test case is needed because we only want to regard
			// the taint on pending machines
			Entry("unknown machine is healthy, and node has `critical-components-not-ready` taint, should still be marked Running", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newHealthyMachine(machineSet1Deploy1, "node-0", machinev1.MachineUnknown),
					},
					nodes: []*corev1.Node{
						newNode(
							1,
							nil,
							nil,
							&corev1.NodeSpec{
								Taints: []corev1.Taint{
									{
										Key:    "node.gardener.cloud/critical-components-not-ready",
										Effect: corev1.TaintEffectNoSchedule,
									},
								},
							},
							&corev1.NodeStatus{
								Phase:      corev1.NodeRunning,
								Conditions: nodeConditions(true, false, false, false, false),
							},
						),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineRunning,
				},
			}),
		)

		DescribeTable("##Meltdown scenario when many machines Unknown for over 10min(healthTimeout)", func(data *data) {
			stop := make(chan struct{})
			defer close(stop)

			controlMachineObjects := []runtime.Object{}
			targetCoreObjects := []runtime.Object{}

			var targetMachine *machinev1.Machine

			for _, o := range data.setup.machines {
				controlMachineObjects = append(controlMachineObjects, o)
				if o.Name == data.setup.targetMachineName {
					targetMachine = o
				}
			}

			for _, o := range data.setup.nodes {
				targetCoreObjects = append(targetCoreObjects, o)
			}

			c, trackers = createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil)
			defer trackers.Stop()

			c.permitGiver = permits.NewPermitGiver(5*time.Second, 1*time.Second)
			defer c.permitGiver.Close()

			waitForCacheSync(stop, c)

			Expect(targetMachine).ToNot(BeNil())

			if data.setup.lockAlreadyAcquired {
				c.permitGiver.RegisterPermits(machineDeploy1, 1)
				c.permitGiver.TryPermit(machineDeploy1, 1*time.Second)
			}

			retryPeriod, err := c.reconcileMachineHealth(context.TODO(), targetMachine)

			if data.setup.lockAlreadyAcquired {
				c.permitGiver.DeletePermits(machineDeploy1)
			}

			Expect(retryPeriod).To(Equal(data.expect.retryPeriod))

			if data.expect.err == nil {
				Expect(err).To(BeNil())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(data.expect.err))
			}

			updatedTargetMachine, getErr := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), targetMachine.Name, metav1.GetOptions{})
			Expect(getErr).To(BeNil())
			Expect(data.expect.expectedPhase).To(Equal(updatedTargetMachine.Status.CurrentStatus.Phase))
		},
			// Note: Test cases assume 1 maxReplacements per machinedeployment

			Entry("2 HealthTimedOut machines: from different machinedeployments,only 1 machine per machineDeployment present, targetMachine should be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy2}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					expectedPhase: machinev1.MachineFailed,
				},
			}),
			Entry("1 HealthTimedOut, 1 Failed machine: from different machinedeployments,only 1 machine per machineDeployment present, targetMachine should be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy2}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineFailed, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					expectedPhase: machinev1.MachineFailed,
				},
			}),
			Entry("2 HealthTimedOut machines: from same machinedeployment,only 2 machines for machineDeployment present, targetMachine should be marked Failed", &data{
				setup: setup{
					machines: newMachines(2,
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
						&metav1.OwnerReference{Name: machineSet1Deploy1},
						nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					expectedPhase: machinev1.MachineFailed,
				},
			}),
			Entry("1 HealthTimedOut , 1 Failed machine: from same machinedeployment,only 2 machines for machineDeployment present, healthTimeoutOut one should NOT be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 1)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineFailed, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           fmt.Errorf("machine %q couldn't be marked FAILED, other machines are getting replaced", machineSet1Deploy1+"-"+"0"),
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("1 HealthTimedOut , 1 Terminating machine: from same machinedeployment,only 2 machines for machineDeployment present, healthTimeoutOut one should NOT be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 1)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineTerminating, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           fmt.Errorf("machine %q couldn't be marked FAILED, other machines are getting replaced", machineSet1Deploy1+"-"+"0"),
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("1 HealthTimedOut , 1 NoPhase machine: from same machinedeployment,only 2 machines for machineDeployment present, healthTimeoutOut one should NOT be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 1)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: "", LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           fmt.Errorf("machine %q couldn't be marked FAILED, other machines are getting replaced", machineSet1Deploy1+"-"+"0"),
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("1 HealthTimedOut , 1 Pending machine: from same machinedeployment,only 2 machines for machineDeployment present, healthTimeoutOut one should NOT be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 1)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachinePending, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           fmt.Errorf("machine %q couldn't be marked FAILED, other machines are getting replaced", machineSet1Deploy1+"-"+"0"),
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("1 HealthTimedOut , 1 CrashLoopBackOff machine: from same machinedeployment,only 2 machines for machineDeployment present, healthTimeoutOut one should NOT be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 1)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineCrashLoopBackOff, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           fmt.Errorf("machine %q couldn't be marked FAILED, other machines are getting replaced", machineSet1Deploy1+"-"+"0"),
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("1 HealthTimedOut , 1 Pending machine: from same machinedeployment,but different machineSets ,only 2 machines for machineDeployment present, healthTimeoutOut one should NOT be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet2Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachinePending, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           fmt.Errorf("machine %q couldn't be marked FAILED, other machines are getting replaced", machineSet1Deploy1+"-"+"0"),
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("Timeout in acquring lock should also occur", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{"name": machineDeploy1, machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName:   machineSet1Deploy1 + "-" + "0",
					lockAlreadyAcquired: true,
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           fmt.Errorf("timedout waiting to acquire lock for machine %q", machineSet1Deploy1+"-"+"0"),
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
		)
	})
})
