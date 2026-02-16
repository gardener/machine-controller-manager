// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gardener/machine-controller-manager/pkg/controller/autoscaler"
	"k8s.io/klog/v2"
	"time"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/nodeops"
	"github.com/gardener/machine-controller-manager/pkg/util/permits"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

const (
	machineDeploy1     = "shoot--testProject--testShoot-testWorkerPool-z1"
	machineDeploy2     = "shoot--testProject--testShoot-testWorkerPool-z2"
	machineSet1Deploy1 = "shoot--testProject--testShoot-testWorkerPool-z1-mset1"
	machineSet2Deploy1 = "shoot--testProject--testShoot-testWorkerPool-z1-mset2"
	machineSet1Deploy2 = "shoot--testProject--testShoot-testWorkerPool-z2-mset1"
)

var _ = Describe("machine_util", func() {
	Describe("#syncNodeTemplates", func() {
		type setup struct {
			machine      *machinev1.Machine
			machineClass *machinev1.MachineClass
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
				machineClass := data.setup.machineClass

				nodeObject := data.action.node
				coreObjects = append(coreObjects, nodeObject)
				controlObjects = append(controlObjects, machineObject)

				c, trackers := createController(stop, testNamespace, controlObjects, nil, coreObjects, nil, false)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				_, err := c.syncNodeTemplates(context.TODO(), machineObject, machineClass)

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
					Expect(updatedNodeObject.Status.Capacity).Should(Equal(data.expect.node.Status.Capacity))

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
					machineClass: &machinev1.MachineClass{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-machine-class",
						},
						NodeTemplate: &machinev1.NodeTemplate{
							VirtualCapacity: corev1.ResourceList{
								"virtual.com/dongle": resource.MustParse("2"),
							},
						},
					},
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
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"virtual.com/dongle\":\"2\"}",
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
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								"virtual.com/dongle": resource.MustParse("2"),
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

				c, trackers := createController(stop, testNamespace, nil, nil, nil, nil, false)
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

				c, trackers := createController(stop, testNamespace, nil, nil, nil, nil, false)
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

				c, trackers := createController(stop, testNamespace, nil, nil, nil, nil, false)
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

	Describe("#SyncVirtualCapacity", func() {
		type action struct {
			node                   *corev1.Node
			desiredVirtualCapacity corev1.ResourceList
		}
		type expect struct {
			node                   *corev1.Node
			virtualCapacityChanged bool
		}
		type data struct {
			action action
			expect expect
		}

		DescribeTable("##table",
			func(data *data) error {
				testNode := data.action.node.DeepCopy()
				desiredVirtualCapacity := data.action.desiredVirtualCapacity
				expectedNode := data.expect.node

				var lastAppliedVirtualCapacity corev1.ResourceList
				lastAppliedVirtualCapacityJSONString, exists := testNode.Annotations[machineutils.LastAppliedVirtualCapacityAnnotation]
				if exists {
					err := json.Unmarshal([]byte(lastAppliedVirtualCapacityJSONString), &lastAppliedVirtualCapacity)
					if err != nil {
						return fmt.Errorf("cannot unmarshall %q annotation: %w", machineutils.LastAppliedVirtualCapacityAnnotation, err)
					}
				}

				virtualCapacityChanged := SyncVirtualCapacity(desiredVirtualCapacity, testNode, lastAppliedVirtualCapacity)

				Expect(virtualCapacityChanged).To(Equal(data.expect.virtualCapacityChanged))
				Expect(testNode.Status.Capacity).Should(Equal(expectedNode.Status.Capacity))

				lastAppliedVirtualCapacityJSONString = expectedNode.Annotations[machineutils.LastAppliedVirtualCapacityAnnotation]
				lastAppliedVirtualCapacity = corev1.ResourceList{}
				err := json.Unmarshal([]byte(lastAppliedVirtualCapacityJSONString), &lastAppliedVirtualCapacity)
				if err != nil {
					return fmt.Errorf("cannot unmarshall %q annotation with value %q: %w", machineutils.LastAppliedVirtualCapacityAnnotation, lastAppliedVirtualCapacityJSONString, err)
				}
				Expect(lastAppliedVirtualCapacity).To(Equal(desiredVirtualCapacity))
				return nil
			},

			Entry("when node.status.capacity has not been changed", &data{
				action: action{
					desiredVirtualCapacity: corev1.ResourceList{
						"hc.hana.com/memory": resource.MustParse("1234567"),
					},
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/memory\":\"1234567\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/memory":  resource.MustParse("1234567"),
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
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/memory\":\"1234567\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/memory":  resource.MustParse("1234567"),
							},
						},
					},
					virtualCapacityChanged: false,
				},
			}),

			Entry("when virtual resource value is changed in virtualCapacity", &data{
				action: action{
					desiredVirtualCapacity: corev1.ResourceList{
						"hc.hana.com/memory": resource.MustParse("2234567"),
					},
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/memory\":\"1234567\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/memory":  resource.MustParse("1234567"),
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
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/memory\":\"2234567\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/memory":  resource.MustParse("2234567"),
							},
						},
					},
					virtualCapacityChanged: true,
				},
			}),

			Entry("when virtual resources are added in virtualCapacity", &data{
				action: action{
					desiredVirtualCapacity: corev1.ResourceList{
						"hc.hana.com/memory": resource.MustParse("1111111"),
						"hc.hana.com/cpu":    resource.MustParse("2"),
					},
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/memory\":\"1111111\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/memory":  resource.MustParse("1111111"),
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
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/cpu\":\"2\",\"hc.hana.com/memory\":\"1111111\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/memory":  resource.MustParse("1111111"),
								"hc.hana.com/cpu":     resource.MustParse("2"),
							},
						},
					},
					virtualCapacityChanged: true,
				},
			}),

			Entry("when virtual resources are deleted in virtualCapacity", &data{
				action: action{
					desiredVirtualCapacity: corev1.ResourceList{
						"hc.hana.com/cpu": resource.MustParse("2"),
					},
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/cpu\":\"2\",\"hc.hana.com/memory\":\"1111111\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/cpu":     resource.MustParse("2"),
								"hc.hana.com/memory":  resource.MustParse("1111111"),
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
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.LastAppliedVirtualCapacityAnnotation: "{\"hc.hana.com/cpu\":\"2\"}",
							},
						},
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1234567"),
								"hc.hana.com/cpu":     resource.MustParse("2"),
							},
						},
					},
					virtualCapacityChanged: true,
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

				c, trackers := createController(stop, testNamespace, controlObjects, nil, nil, nil, false)
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

			c, trackers = createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
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
			Entry("Pending machine is marked as Failed when creation timeout (20min) has elapsed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachinePending}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0-0"}, true, metav1.NewTime(time.Now().Add(-25*time.Minute))),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineFailed,
				},
			}),
			Entry("Pending machine stays in the Pending phase if creation timeout (20min) has not elapsed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachinePending}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0-0"}, true, metav1.NewTime(time.Now().Add(-15*time.Minute))),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.LongRetry,
					err:           nil,
					expectedPhase: machinev1.MachinePending,
				},
			}),
			Entry("Pending machine is marked as Failed when creation timeout (20min) has elapsed, even if lastUpdate time is recent", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachinePending, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0-0"}, true, metav1.NewTime(time.Now().Add(-25*time.Minute))),
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
			Entry("Machine in Unknown state WITHOUT backing node obj for over 10min(healthTimeout) should not be marked Failed if DisableHealthTimeout is set", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0), Spec: machinev1.MachineSpec{MachineConfiguration: &machinev1.MachineConfiguration{DisableHealthTimeout: ptr.To(true)}}},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineUnknown, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.LongRetry,
					expectedPhase: machinev1.MachineUnknown,
				},
			}),
			Entry("Machine in InPlaceUpdating state for over 10min(InPlaceUpdateTimeout) should be marked InPlaceUpdateFailed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineInPlaceUpdating, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
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
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineInPlaceUpdateFailed,
				},
			}),
			Entry("Machine in InPlaceUpdating state with node label node.machine.sapcloud.io/update-result=successful should be marked InPlaceUpdateSuccessful", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineInPlaceUpdating, LastUpdateTime: metav1.NewTime(time.Now())}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{machinev1.LabelKeyNodeUpdateResult: machinev1.LabelValueNodeUpdateSuccessful}, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineInPlaceUpdateSuccessful,
				},
			}),
			Entry("Machine in InPlaceUpdating state with node label node.machine.sapcloud.io/update-result=failed should be marked InPlaceUpdateFailed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineInPlaceUpdating, LastUpdateTime: metav1.NewTime(time.Now())}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{machinev1.LabelKeyNodeUpdateResult: machinev1.LabelValueNodeUpdateFailed}, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.ShortRetry,
					err:           errSuccessfulPhaseUpdate,
					expectedPhase: machinev1.MachineInPlaceUpdateFailed,
				},
			}),
			Entry("Machine in InPlaceUpdateFailed state for over 10min(healthTimeout) should be marked Failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineInPlaceUpdateFailed, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
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
			Entry("Machine in InPlaceUpdateFailed state for over 10min(healthTimeout) should not be marked Failed if DisableHealthTimeout is set", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0), Spec: machinev1.MachineSpec{MachineConfiguration: &machinev1.MachineConfiguration{DisableHealthTimeout: ptr.To(true)}}},
							&machinev1.MachineStatus{Conditions: nodeConditions(false, false, false, false, false), CurrentStatus: machinev1.CurrentStatus{Phase: machinev1.MachineInPlaceUpdateFailed, LastUpdateTime: metav1.NewTime(time.Now().Add(-15 * time.Minute))}},
							&metav1.OwnerReference{Name: machineSet1Deploy1},
							nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(false, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod:   machineutils.LongRetry,
					expectedPhase: machinev1.MachineInPlaceUpdateFailed,
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

			c, trackers = createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
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

	Describe("#updateNodeConditionBasedOnLabel", func() {
		type setup struct {
			machines []*machinev1.Machine
			nodes    []*corev1.Node
			// targetMachineName is name of machine for which `reconcileMachineHealth()` needs to be called
			// Its must this machine in machine list
			targetMachineName string
		}

		type expect struct {
			retryPeriod machineutils.RetryPeriod
			err         error
			node        *corev1.Node
		}
		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
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

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()

				c.permitGiver = permits.NewPermitGiver(5*time.Second, 1*time.Second)
				defer c.permitGiver.Close()

				waitForCacheSync(stop, c)

				retryPeriod, err := c.updateNodeConditionBasedOnLabel(context.TODO(), targetMachine)

				Expect(retryPeriod).To(Equal(data.expect.retryPeriod))
				if data.expect.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(data.expect.err))
				}

				if data.expect.node != nil {
					updatedNode, getErr := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), getNodeName(targetMachine), metav1.GetOptions{})
					Expect(getErr).To(BeNil())
					updatedNodeCondition := nodeops.GetCondition(updatedNode, machinev1.NodeInPlaceUpdate)
					expectedNodeCondition := nodeops.GetCondition(data.expect.node, machinev1.NodeInPlaceUpdate)

					Expect(updatedNodeCondition.Type).To(Equal(expectedNodeCondition.Type))
					Expect(updatedNodeCondition.Status).To(Equal(expectedNodeCondition.Status))
					Expect(updatedNodeCondition.Reason).To(Equal(expectedNodeCondition.Reason))
					Expect(updatedNodeCondition.Message).To(Equal(expectedNodeCondition.Message))
				}
			},

			Entry("when node is not found", &data{
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
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node:        nil,
				},
			}),
			Entry("when node has LabelKeyNodeCandidateForUpdate", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							nil,
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{machinev1.LabelKeyNodeCandidateForUpdate: "true"},
							nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning, Conditions: nodeConditions(true, false, false, false, false)}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.CandidateForUpdate,
									Message:            "Node is a candidate for in-place update",
								},
							},
						},
					},
				},
			}),
			Entry("when node has LabelKeyNodeCandidateForUpdate but still not selected for update", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							nil,
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{
							machinev1.LabelKeyNodeCandidateForUpdate: "true",
						},
							nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning,
								Conditions: append(nodeConditions(true, false, false, false, false),
									corev1.NodeCondition{
										Type:               machinev1.NodeInPlaceUpdate,
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Now(),
										Reason:             machinev1.CandidateForUpdate,
										Message:            "Node is a candidate for in-place update"})}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.CandidateForUpdate,
									Message:            "Node is a candidate for in-place update",
								},
							},
						},
					},
				},
			}),
			Entry("when node has LabelKeyNodeCandidateForUpdate also the in-place update reason is UpdateSuccessful", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							nil,
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{
							machinev1.LabelKeyNodeCandidateForUpdate: "true",
						},
							nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning,
								Conditions: append(nodeConditions(true, false, false, false, false),
									corev1.NodeCondition{
										Type:               machinev1.NodeInPlaceUpdate,
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Now(),
										Reason:             machinev1.UpdateSuccessful,
										Message:            "Node in-place update successful"})}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.CandidateForUpdate,
									Message:            "Node is a candidate for in-place update",
								},
							},
						},
					},
				},
			}),
			Entry("when node has LabelKeyNodeSelectedForUpdate", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							nil,
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{
							machinev1.LabelKeyNodeCandidateForUpdate: "true",
							machinev1.LabelKeyNodeSelectedForUpdate:  "true",
						},
							nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning,
								Conditions: append(nodeConditions(true, false, false, false, false),
									corev1.NodeCondition{
										Type:               machinev1.NodeInPlaceUpdate,
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Now(),
										Reason:             machinev1.CandidateForUpdate,
										Message:            "Node is a candidate for in-place update"})}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
								machinev1.LabelKeyNodeSelectedForUpdate:  "true",
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.SelectedForUpdate,
									Message:            "Node is selected for in-place update",
								},
							},
						},
					},
				},
			}),
			Entry("when node has LabelKeyNodeSelectedForUpdate but drain is not successful yet", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							nil,
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{
							machinev1.LabelKeyNodeCandidateForUpdate: "true",
							machinev1.LabelKeyNodeSelectedForUpdate:  "true",
						},
							nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning,
								Conditions: append(nodeConditions(true, false, false, false, false),
									corev1.NodeCondition{
										Type:               machinev1.NodeInPlaceUpdate,
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Now(),
										Reason:             machinev1.SelectedForUpdate,
										Message:            "Node is selected for in-place update"})}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod: machineutils.MediumRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
								machinev1.LabelKeyNodeSelectedForUpdate:  "true",
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.SelectedForUpdate,
									Message:            "Node is selected for in-place update",
								},
							},
						},
					},
				},
			}),
			Entry("when node update is successful", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							nil,
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1, map[string]string{
							machinev1.LabelKeyNodeCandidateForUpdate: "true",
							machinev1.LabelKeyNodeSelectedForUpdate:  "true",
							machinev1.LabelKeyNodeUpdateResult:       "successful",
						},
							nil, &corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning,
								Conditions: append(nodeConditions(true, false, false, false, false),
									corev1.NodeCondition{
										Type:               machinev1.NodeInPlaceUpdate,
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Now(),
										Reason:             machinev1.ReadyForUpdate,
										Message:            "Node is ready for in-place update"})}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
								machinev1.LabelKeyNodeSelectedForUpdate:  "true",
								machinev1.LabelKeyNodeUpdateResult:       "successful",
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.UpdateSuccessful,
									Message:            "Node in-place update successful",
								},
							},
						},
					},
				},
			}),
			Entry("when node update has failed", &data{
				setup: setup{
					machines: []*machinev1.Machine{
						newMachine(
							&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
							nil,
							nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					},
					nodes: []*corev1.Node{
						newNode(1,
							map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
								machinev1.LabelKeyNodeSelectedForUpdate:  "true",
								machinev1.LabelKeyNodeUpdateResult:       "failed",
							},
							map[string]string{machinev1.AnnotationKeyMachineUpdateFailedReason: "OS image not found"},
							&corev1.NodeSpec{}, &corev1.NodeStatus{Phase: corev1.NodeRunning,
								Conditions: append(nodeConditions(true, false, false, false, false),
									corev1.NodeCondition{
										Type:               machinev1.NodeInPlaceUpdate,
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Now(),
										Reason:             machinev1.DrainSuccessful,
										Message:            "Node draining successful"})}),
					},
					targetMachineName: machineSet1Deploy1 + "-" + "0",
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Labels: map[string]string{
								machinev1.LabelKeyNodeCandidateForUpdate: "true",
								machinev1.LabelKeyNodeSelectedForUpdate:  "true",
								machinev1.LabelKeyNodeUpdateResult:       "failed",
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.UpdateFailed,
									Message:            fmt.Sprintf("Node in-place update failed: %s", "OS image not found"),
								},
							},
						},
					},
				},
			}),
		)
	})

	Describe("#inPlaceUpdate", func() {
		type setup struct {
			machine *machinev1.Machine
			node    *corev1.Node
		}

		type expect struct {
			retryPeriod machineutils.RetryPeriod
			err         error
			node        *corev1.Node
		}

		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				targetCoreObjects := []runtime.Object{}

				controlMachineObjects = append(controlMachineObjects, data.setup.machine)
				targetCoreObjects = append(targetCoreObjects, data.setup.node)

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()

				c.permitGiver = permits.NewPermitGiver(5*time.Second, 1*time.Second)
				defer c.permitGiver.Close()

				waitForCacheSync(stop, c)

				retryPeriod, err := c.inPlaceUpdate(context.TODO(), data.setup.machine)

				Expect(retryPeriod).To(Equal(data.expect.retryPeriod))
				if data.expect.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(data.expect.err.Error()))
				}

				if data.expect.node != nil {
					updatedNode, getErr := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), getNodeName(data.setup.machine), metav1.GetOptions{})
					Expect(getErr).To(BeNil())
					updatedNodeCondition := nodeops.GetCondition(updatedNode, machinev1.NodeInPlaceUpdate)
					expectedNodeCondition := nodeops.GetCondition(data.expect.node, machinev1.NodeInPlaceUpdate)

					Expect(updatedNodeCondition.Type).To(Equal(expectedNodeCondition.Type))
					Expect(updatedNodeCondition.Status).To(Equal(expectedNodeCondition.Status))
					Expect(updatedNodeCondition.Reason).To(Equal(expectedNodeCondition.Reason))
					Expect(updatedNodeCondition.Message).To(Equal(expectedNodeCondition.Message))
				}
			},

			Entry("when node is not present", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0-test", machinev1.LabelKeyNodeSelectedForUpdate: "true"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{}),
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node:        nil,
				},
			}),
			Entry("when node condition is nil", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0", machinev1.LabelKeyNodeSelectedForUpdate: "true"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{}),
				},
				expect: expect{
					retryPeriod: machineutils.LongRetry,
					err:         nil,
					node:        nil,
				},
			}),
			Entry("when node condition reason is SelectedForUpdate", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0", machinev1.LabelKeyNodeSelectedForUpdate: "true"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:               machinev1.NodeInPlaceUpdate,
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Now(),
								Reason:             machinev1.SelectedForUpdate,
								Message:            "Node is selected for in-place update",
							},
						},
					}),
				},
				expect: expect{
					retryPeriod: machineutils.MediumRetry,
					err:         fmt.Errorf("node %s is ready for in-place update", "node-0"),
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.ReadyForUpdate,
									Message:            "Node is ready for in-place update",
								},
							},
						},
					},
				},
			}),
			Entry("when node condition reason is DrainSuccessful", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0", machinev1.LabelKeyNodeSelectedForUpdate: "true"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:               machinev1.NodeInPlaceUpdate,
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Now(),
								Reason:             machinev1.DrainSuccessful,
								Message:            "Node draining successful",
							},
						},
					}),
				},
				expect: expect{
					retryPeriod: machineutils.MediumRetry,
					err:         fmt.Errorf("node %s is ready for in-place update", "node-0"),
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.ReadyForUpdate,
									Message:            "Node is ready for in-place update",
								},
							},
						},
					},
				},
			}),
			Entry("when node condition reason is ReadyForUpdate", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0", machinev1.LabelKeyNodeSelectedForUpdate: "true"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:               machinev1.NodeInPlaceUpdate,
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Now(),
								Reason:             machinev1.ReadyForUpdate,
								Message:            "Node is ready for in-place update",
							},
						},
					}),
				},
				expect: expect{
					retryPeriod: machineutils.MediumRetry,
					err:         fmt.Errorf("node %s is ready for in-place update", "node-0"),
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.ReadyForUpdate,
									Message:            "Node is ready for in-place update",
								},
							},
						},
					},
				},
			}),
		)
	})

	Describe("#drainNodeForInPlace", func() {
		type setup struct {
			machine *machinev1.Machine
			node    *corev1.Node
		}

		type expect struct {
			retryPeriod machineutils.RetryPeriod
			err         error
			node        *corev1.Node
		}

		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				targetCoreObjects := []runtime.Object{}

				controlMachineObjects = append(controlMachineObjects, data.setup.machine)
				if data.setup.node != nil {
					targetCoreObjects = append(targetCoreObjects, data.setup.node)
				}

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()

				c.permitGiver = permits.NewPermitGiver(5*time.Second, 1*time.Second)
				defer c.permitGiver.Close()

				waitForCacheSync(stop, c)

				retryPeriod, err := c.drainNodeForInPlace(context.TODO(), data.setup.machine)

				Expect(retryPeriod).To(Equal(data.expect.retryPeriod))
				if data.expect.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(data.expect.err.Error()))
				}

				if data.expect.node != nil {
					updatedNode, getErr := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), getNodeName(data.setup.machine), metav1.GetOptions{})
					Expect(getErr).To(BeNil())
					updatedNodeCondition := nodeops.GetCondition(updatedNode, machinev1.NodeInPlaceUpdate)
					expectedNodeCondition := nodeops.GetCondition(data.expect.node, machinev1.NodeInPlaceUpdate)

					Expect(updatedNodeCondition.Type).To(Equal(expectedNodeCondition.Type))
					Expect(updatedNodeCondition.Status).To(Equal(expectedNodeCondition.Status))
					Expect(updatedNodeCondition.Reason).To(Equal(expectedNodeCondition.Reason))
					Expect(updatedNodeCondition.Message).To(Equal(expectedNodeCondition.Message))
				}
			},

			Entry("when node is not found", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0-0"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{}),
				},
				expect: expect{
					retryPeriod: machineutils.ShortRetry,
					err:         fmt.Errorf("nodes \"node-0-0\" not found"),
					node:        nil,
				},
			}),
			Entry("when node is not ready for over 5 minutes", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						&machinev1.MachineStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               corev1.NodeReady,
									Status:             corev1.ConditionFalse,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
								},
							},
						},
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{}),
				},
				expect: expect{
					retryPeriod: machineutils.ShortRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.DrainSuccessful,
									Message:            "Node draining successful",
								},
							},
						},
					},
				},
			}),
			Entry("when node is in ReadonlyFilesystem for over 5 minutes", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						&machinev1.MachineStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               "ReadonlyFilesystem",
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
								},
							},
						},
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{}),
				},
				expect: expect{
					retryPeriod: machineutils.ShortRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.DrainSuccessful,
									Message:            "Node draining successful",
								},
							},
						},
					},
				},
			}),
			Entry("when force drain is triggered", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0", "force-drain": "True"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{}),
				},
				expect: expect{
					retryPeriod: machineutils.ShortRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.DrainSuccessful,
									Message:            "Node draining successful",
								},
							},
						},
					},
				},
			}),
			Entry("when normal drain is triggered", &data{
				setup: setup{
					machine: newMachine(
						&machinev1.MachineTemplateSpec{ObjectMeta: *newObjectMeta(&metav1.ObjectMeta{GenerateName: machineSet1Deploy1}, 0)},
						nil,
						nil, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}, true, metav1.Now()),
					node: newNode(1, nil, nil, &corev1.NodeSpec{}, &corev1.NodeStatus{}),
				},
				expect: expect{
					retryPeriod: machineutils.ShortRetry,
					err:         nil,
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:               machinev1.NodeInPlaceUpdate,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
									Reason:             machinev1.DrainSuccessful,
									Message:            "Node draining successful",
								},
							},
						},
					},
				},
			}),
		)
	})
	Describe("#preserveMachine", func() {
		type setup struct {
			machinePhase           machinev1.MachinePhase
			nodeName               string
			preserveValue          string
			isCAAnnotationPresent  bool
			preservedNodeCondition corev1.NodeCondition
		}
		type expect struct {
			preserveNodeCondition   corev1.NodeCondition
			isPreserveExpiryTimeSet bool
			isCAAnnotationPresent   bool
			err                     error
		}
		type testCase struct {
			setup  setup
			expect expect
		}
		DescribeTable("##preserveMachine behaviour scenarios",
			func(tc *testCase) {
				stop := make(chan struct{})
				defer close(stop)

				var controlMachineObjects []runtime.Object
				var targetCoreObjects []runtime.Object

				machine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: testNamespace,
						Labels: map[string]string{
							machinev1.NodeLabelKey: tc.setup.nodeName,
						},
					},
					Spec: machinev1.MachineSpec{},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:              tc.setup.machinePhase,
							LastUpdateTime:     metav1.Now(),
							PreserveExpiryTime: nil,
						},
					},
				}
				if tc.setup.nodeName != "" && tc.setup.nodeName != "err-backing-node" {

					node := corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:        tc.setup.nodeName,
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{},
						},
					}
					if tc.setup.isCAAnnotationPresent {
						node.Annotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey] = "true"
					}
					targetCoreObjects = append(targetCoreObjects, &node)
				}
				controlMachineObjects = append(controlMachineObjects, machine)

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()
				waitForCacheSync(stop, c)
				_, err := c.preserveMachine(context.TODO(), machine, tc.setup.preserveValue)
				if tc.expect.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(tc.expect.err.Error()))
				}
				updatedMachine, getErr := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), machine.Name, metav1.GetOptions{})
				Expect(getErr).To(BeNil())
				if tc.expect.isPreserveExpiryTimeSet {
					Expect(updatedMachine.Status.CurrentStatus.PreserveExpiryTime.IsZero()).To(BeFalse())
				} else {
					Expect(updatedMachine.Status.CurrentStatus.PreserveExpiryTime.IsZero()).To(BeTrue())
				}
				if tc.setup.nodeName == "" || tc.setup.nodeName == "err-backing-node" {
					return
				}
				updatedNode, getErr := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), tc.setup.nodeName, metav1.GetOptions{})
				Expect(getErr).To(BeNil())
				Expect(updatedNode.Annotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey]).To(Equal(autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue))
				if tc.expect.preserveNodeCondition.Type != "" {
					updatedNodeCondition := nodeops.GetCondition(updatedNode, tc.expect.preserveNodeCondition.Type)
					Expect(updatedNodeCondition.Status).To(Equal(tc.expect.preserveNodeCondition.Status))
					Expect(updatedNodeCondition.Reason).To(Equal(tc.expect.preserveNodeCondition.Reason))
					Expect(updatedNodeCondition.Message).To(ContainSubstring(tc.expect.preserveNodeCondition.Message))
				}
			},
			Entry("when preserve=now and there is no backing node", &testCase{
				setup: setup{
					machinePhase:  machinev1.MachineUnknown,
					nodeName:      "",
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
				},
			}),
			Entry("when preserve=now, the machine is Running, and there is a backing node", &testCase{
				setup: setup{
					machinePhase:  machinev1.MachineRunning,
					nodeName:      "node-1",
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   true,
					preserveNodeCondition: corev1.NodeCondition{
						Type:   machinev1.NodePreserved,
						Status: corev1.ConditionTrue,
						Reason: machinev1.PreservedByUser,
					},
				},
			}),
			Entry("when preserve=now, the machine has Failed, and there is a backing node", &testCase{
				setup: setup{
					machinePhase:  machinev1.MachineFailed,
					nodeName:      "node-1",
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   true,
					preserveNodeCondition: corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful,
					},
				},
			}),
			Entry("when preserve=now, the machine has Failed, and the preservation is incomplete after step 1 - adding preserveExpiryTime", &testCase{
				setup: setup{
					machinePhase:  machinev1.MachineFailed,
					nodeName:      "node-1",
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   true,
					preserveNodeCondition: corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful,
					},
				},
			}),
			Entry("when preserve=now, the machine has Failed, and the preservation is incomplete at step 2 - adding CA annotations", &testCase{
				setup: setup{
					machinePhase:          machinev1.MachineFailed,
					nodeName:              "node-1",
					preserveValue:         machineutils.PreserveMachineAnnotationValueNow,
					isCAAnnotationPresent: true,
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   true,
					preserveNodeCondition: corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful,
					},
				},
			}),
			Entry("when preserve=now, the machine has Failed, and the preservation is incomplete because of drain failure", &testCase{
				setup: setup{
					machinePhase:          machinev1.MachineFailed,
					nodeName:              "node-1",
					preserveValue:         machineutils.PreserveMachineAnnotationValueNow,
					isCAAnnotationPresent: true,
					preservedNodeCondition: corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionFalse,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainUnsuccessful,
					},
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   true,
					preserveNodeCondition: corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful,
					},
				},
			}),
			Entry("when preserve=when-failed, the machine has Failed, and there is a backing node", &testCase{
				setup: setup{
					machinePhase:  machinev1.MachineFailed,
					nodeName:      "node-1",
					preserveValue: machineutils.PreserveMachineAnnotationValueWhenFailed,
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   true,
					preserveNodeCondition: corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful,
					},
				},
			}),
			Entry("when preserve=auto-preserved, the machine has Failed, and there is a backing node", &testCase{
				setup: setup{
					machinePhase:  machinev1.MachineFailed,
					nodeName:      "node-1",
					preserveValue: machineutils.PreserveMachineAnnotationValuePreservedByMCM,
				},
				expect: expect{
					err:                     nil,
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   true,
					preserveNodeCondition: corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByMCM,
						Message: machinev1.PreservedNodeDrainSuccessful,
					},
				},
			}),
			Entry("when preserve=now, the machine has Failed, and there is an error fetching backing node", &testCase{
				setup: setup{
					machinePhase:  machinev1.MachineFailed,
					nodeName:      "err-backing-node",
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
				},
				expect: expect{
					err:                     fmt.Errorf("node \"err-backing-node\" not found"),
					isPreserveExpiryTimeSet: true,
					isCAAnnotationPresent:   false,
				},
			},
			),
		)
	})
	Describe("#stopMachinePreservationIfPreserved", func() {
		type setup struct {
			nodeName                 string
			removePreserveAnnotation bool
		}
		type expect struct {
			err error
		}
		type testCase struct {
			setup  setup
			expect expect
		}
		DescribeTable("##stopMachinePreservationIfPreserved behaviour scenarios",
			func(tc *testCase) {
				stop := make(chan struct{})
				defer close(stop)

				var controlMachineObjects []runtime.Object
				var targetCoreObjects []runtime.Object

				machine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: testNamespace,
						Labels:    map[string]string{},
					},
					Spec: machinev1.MachineSpec{},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:              machinev1.MachineFailed,
							LastUpdateTime:     metav1.Now(),
							PreserveExpiryTime: &metav1.Time{Time: time.Now().Add(10 * time.Minute)},
						},
					},
				}
				if tc.setup.nodeName != "" && tc.setup.nodeName != "err-backing-node" {
					machine.Labels[machinev1.NodeLabelKey] = tc.setup.nodeName
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
							Annotations: map[string]string{
								machineutils.PreserveMachineAnnotationKey:                  machineutils.PreserveMachineAnnotationValueNow,
								autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey: autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue,
							},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:   machinev1.NodePreserved,
									Status: corev1.ConditionTrue,
									Reason: machinev1.PreservedByUser,
								},
							},
						},
					}
					targetCoreObjects = append(targetCoreObjects, node)

				} else {
					machine.Labels[machinev1.NodeLabelKey] = ""
				}

				controlMachineObjects = append(controlMachineObjects, machine)

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()
				waitForCacheSync(stop, c)
				_, err := c.stopMachinePreservationIfPreserved(context.TODO(), machine, tc.setup.removePreserveAnnotation)
				if tc.expect.err != nil {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(tc.expect.err.Error()))
					return
				}
				Expect(err).To(BeNil())
				updatedMachine, getErr := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), machine.Name, metav1.GetOptions{})
				Expect(getErr).To(BeNil())
				Expect(updatedMachine.Status.CurrentStatus.PreserveExpiryTime.IsZero()).To(BeTrue())

				if machine.Labels[machinev1.NodeLabelKey] == "" || machine.Labels[machinev1.NodeLabelKey] == "no-backing-node" {
					return
				}
				updatedNode, getErr := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), tc.setup.nodeName, metav1.GetOptions{})
				Expect(getErr).To(BeNil())
				updatedNodeCondition := nodeops.GetCondition(updatedNode, machinev1.NodePreserved)
				Expect(updatedNodeCondition).ToNot(BeNil())
				Expect(updatedNodeCondition.Status).To(Equal(corev1.ConditionFalse))
				Expect(updatedNodeCondition.Reason).To(Equal(machinev1.PreservationStopped))
				if tc.setup.removePreserveAnnotation {
					Expect(updatedNode.Annotations).NotTo(HaveKey(machineutils.PreserveMachineAnnotationKey))
				} else {
					Expect(updatedNode.Annotations).To(HaveKey(machineutils.PreserveMachineAnnotationKey))
				}

			},
			Entry("when stopping preservation on a preserved machine with backing node and preserve annotation needs to be removed", &testCase{
				setup: setup{
					nodeName:                 "node-1",
					removePreserveAnnotation: true,
				},
				expect: expect{
					err: nil,
				},
			}),
			Entry("when stopping preservation on a preserved machine with backing node and preserve annotation shouldn't be removed", &testCase{
				setup: setup{
					nodeName:                 "node-1",
					removePreserveAnnotation: false,
				},
				expect: expect{
					err: nil,
				},
			}),
			Entry("when stopping preservation on a preserved machine with no backing node", &testCase{
				setup: setup{
					nodeName: "",
				},
				expect: expect{
					err: nil,
				},
			}),
			Entry("when stopping preservation on a preserved machine, and the backing node is not found", &testCase{
				setup: setup{
					nodeName: "no-backing-node",
				},
				expect: expect{
					err: nil,
				},
			}),
		)
	})
	Describe("#computeNewNodePreservedCondition", func() {
		preserveExpiryTime := &metav1.Time{Time: time.Now().Add(2 * time.Hour)}
		type setup struct {
			currentStatus         machinev1.CurrentStatus
			preserveValue         string
			drainErr              error
			existingNodeCondition *corev1.NodeCondition
		}
		type expect struct {
			newNodeCondition *corev1.NodeCondition
			needsUpdate      bool
		}
		type testCase struct {
			setup  setup
			expect expect
		}
		DescribeTable("##computeNewNodePreservedCondition behaviour scenarios",
			func(tc *testCase) {
				newNodeCondition, needsUpdate := computeNewNodePreservedCondition(
					tc.setup.currentStatus,
					tc.setup.preserveValue,
					tc.setup.drainErr,
					tc.setup.existingNodeCondition,
				)
				if tc.expect.newNodeCondition == nil {
					Expect(newNodeCondition).To(BeNil())
				} else {
					Expect(newNodeCondition.Type).To(Equal(tc.expect.newNodeCondition.Type))
					Expect(newNodeCondition.Status).To(Equal(tc.expect.newNodeCondition.Status))
					Expect(newNodeCondition.Reason).To(Equal(tc.expect.newNodeCondition.Reason))
					Expect(newNodeCondition.Message).To(Equal(tc.expect.newNodeCondition.Message))
				}
				Expect(needsUpdate).To(Equal(tc.expect.needsUpdate))
			},
			Entry("when preserve=now, machine is Running, no existing condition", &testCase{
				setup: setup{
					currentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineRunning,
						LastUpdateTime:     metav1.Now(),
						PreserveExpiryTime: preserveExpiryTime,
					},
					preserveValue:         machineutils.PreserveMachineAnnotationValueNow,
					existingNodeCondition: nil,
				},
				expect: expect{
					newNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: "Machine preserved until " + preserveExpiryTime.String(),
					},
					needsUpdate: true,
				},
			}),
			Entry("when preserve=now, machine is Failed, drain successful, no existing condition", &testCase{
				setup: setup{
					currentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineFailed,
						LastUpdateTime:     metav1.Now(),
						PreserveExpiryTime: preserveExpiryTime,
					},
					preserveValue:         machineutils.PreserveMachineAnnotationValueNow,
					drainErr:              nil,
					existingNodeCondition: nil,
				},
				expect: expect{
					newNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
					needsUpdate: true,
				},
			}),
			Entry("when preserve=now, machine is Failed, drain is unsuccessful, no existing condition", &testCase{
				setup: setup{
					currentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineFailed,
						LastUpdateTime:     metav1.Now(),
						PreserveExpiryTime: preserveExpiryTime,
					},
					preserveValue:         machineutils.PreserveMachineAnnotationValueNow,
					drainErr:              fmt.Errorf("test drain error"),
					existingNodeCondition: nil,
				},
				expect: expect{
					newNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionFalse,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainUnsuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
					needsUpdate: true,
				},
			}),
			Entry("when machine auto-preserved by MCM, machine is Failed, drain is successful, no existing condition", &testCase{
				setup: setup{
					currentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineFailed,
						LastUpdateTime:     metav1.Now(),
						PreserveExpiryTime: preserveExpiryTime,
					},
					preserveValue:         machineutils.PreserveMachineAnnotationValuePreservedByMCM,
					drainErr:              nil,
					existingNodeCondition: nil,
				},
				expect: expect{
					newNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByMCM,
						Message: machinev1.PreservedNodeDrainSuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
					needsUpdate: true,
				},
			}),
			Entry("when preserve=now, machine is Failed, drain is unsuccessful, existing condition present", &testCase{
				setup: setup{
					currentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineFailed,
						LastUpdateTime:     metav1.Now(),
						PreserveExpiryTime: preserveExpiryTime,
					},
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
					drainErr:      fmt.Errorf("test drain error"),
					existingNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionFalse,
						Reason:  machinev1.PreservedByUser,
						Message: "Machine preserved until " + preserveExpiryTime.String(),
					},
				},
				expect: expect{
					newNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionFalse,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainUnsuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
					needsUpdate: true,
				},
			}),
			Entry("when preserve=now, machine is Failed, drain is unsuccessful for the second time, existing condition present", &testCase{
				setup: setup{
					currentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineFailed,
						LastUpdateTime:     metav1.Now(),
						PreserveExpiryTime: &metav1.Time{Time: time.Now().Add(2 * time.Hour)},
					},
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
					drainErr:      fmt.Errorf("test drain error"),
					existingNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionFalse,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainUnsuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
				},
				expect: expect{
					newNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionFalse,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainUnsuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
					needsUpdate: false,
				},
			}),
			Entry("when preserve=now, machine is Failed, drain is successful, existing condition present and status is true", &testCase{
				setup: setup{
					currentStatus: machinev1.CurrentStatus{
						Phase:              machinev1.MachineFailed,
						LastUpdateTime:     metav1.Now(),
						PreserveExpiryTime: &metav1.Time{Time: time.Now().Add(2 * time.Hour)},
					},
					preserveValue: machineutils.PreserveMachineAnnotationValueNow,
					drainErr:      nil,
					existingNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
				},
				expect: expect{
					newNodeCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionTrue,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainSuccessful + ". Machine preserved until " + preserveExpiryTime.String(),
					},
					needsUpdate: false,
				},
			}),
		)
	})
	Describe("#shouldPreservedNodeBeDrained", func() {
		type setup struct {
			machinePhase      machinev1.MachinePhase
			existingCondition *corev1.NodeCondition
		}
		type expect struct {
			shouldDrain bool
		}
		type testCase struct {
			setup  setup
			expect expect
		}

		DescribeTable("##shouldPreservedNodeBeDrained behaviour scenarios",
			func(tc *testCase) {
				shouldDrain := shouldPreservedNodeBeDrained(tc.setup.existingCondition, tc.setup.machinePhase)
				Expect(shouldDrain).To(Equal(tc.expect.shouldDrain))
			},
			Entry("should return false when machine is Running", &testCase{
				setup: setup{
					machinePhase: machinev1.MachineRunning,
				},
				expect: expect{
					shouldDrain: false,
				},
			}),
			Entry("should return true when machine is Failed and there is no existing node condition", &testCase{
				setup: setup{
					machinePhase: machinev1.MachineFailed,
				},
				expect: expect{
					shouldDrain: true,
				},
			}),
			Entry("should return true when machine is Failed and existing node condition message is PreservedNodeDrainUnsuccessful", &testCase{
				setup: setup{
					machinePhase: machinev1.MachineFailed,
					existingCondition: &corev1.NodeCondition{
						Type:    machinev1.NodePreserved,
						Status:  corev1.ConditionFalse,
						Reason:  machinev1.PreservedByUser,
						Message: machinev1.PreservedNodeDrainUnsuccessful,
					},
				},
				expect: expect{
					shouldDrain: true,
				},
			}),
		)
	})
	Describe("#removePreservationRelatedAnnotationsOnNode", func() {
		type setup struct {
			removePreserveAnnotation bool
			CAAnnotationPresent      bool
			CAMCMAnnotationPresent   bool
		}
		type expect struct {
			err                   error
			hasAnnotationKeys     []string
			deletedAnnotationKeys []string
		}
		type testCase struct {
			setup  setup
			expect expect
		}
		DescribeTable("##removePreservationRelatedAnnotationsOnNode behaviour scenarios",
			func(tc *testCase) {
				stop := make(chan struct{})
				defer close(stop)
				var targetCoreObjects []runtime.Object
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Annotations: map[string]string{
							machineutils.PreserveMachineAnnotationKey: machineutils.PreserveMachineAnnotationValueNow,
						},
					},
				}
				if tc.setup.CAAnnotationPresent {
					node.Annotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey] = autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue
				}
				if tc.setup.CAMCMAnnotationPresent {
					node.Annotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey] = autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue
				}
				targetCoreObjects = append(targetCoreObjects, node)
				c, trackers := createController(stop, testNamespace, nil, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()
				waitForCacheSync(stop, c)
				err := c.removePreservationRelatedAnnotationsOnNode(context.TODO(), node, tc.setup.removePreserveAnnotation)
				waitForCacheSync(stop, c)
				updatedNode, getErr := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
				Expect(getErr).To(BeNil())
				if tc.expect.err != nil {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(tc.expect.err.Error()))
				} else {
					Expect(err).To(BeNil())
				}
				for _, key := range tc.expect.hasAnnotationKeys {
					Expect(updatedNode.Annotations).To(HaveKey(key))
				}
				for key := range tc.expect.deletedAnnotationKeys {
					Expect(updatedNode.Annotations).NotTo(HaveKey(key))
				}
			},
			Entry("when removePreserveAnnotation is true and ClusterAutoscalerScaleDownDisabledAnnotationByMCM annotation is present, should delete all preservation related annotations", &testCase{
				setup: setup{
					removePreserveAnnotation: true,
					CAAnnotationPresent:      true,
					CAMCMAnnotationPresent:   true,
				},
				expect: expect{
					err:               nil,
					hasAnnotationKeys: []string{},
					deletedAnnotationKeys: []string{
						machineutils.PreserveMachineAnnotationKey,
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey,
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey,
					},
				},
			}),
			Entry("when removePreserveAnnotation is false and ClusterAutoscalerScaleDownDisabledAnnotationByMCM annotation is present, should delete only CA annotations ", &testCase{
				setup: setup{
					removePreserveAnnotation: false,
					CAAnnotationPresent:      true,
					CAMCMAnnotationPresent:   false,
				},
				expect: expect{
					err: nil,
					hasAnnotationKeys: []string{
						machineutils.PreserveMachineAnnotationKey,
					},
					deletedAnnotationKeys: []string{
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey,
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey,
					},
				},
			}),
			Entry("when removePreserveAnnotation is true and ClusterAutoscalerScaleDownDisabledAnnotationByMCM is not present, should delete only preserve annotation", &testCase{
				setup: setup{
					removePreserveAnnotation: true,
					CAAnnotationPresent:      true,
					CAMCMAnnotationPresent:   false,
				},
				expect: expect{
					err: nil,
					hasAnnotationKeys: []string{
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey,
					},
					deletedAnnotationKeys: []string{
						machineutils.PreserveMachineAnnotationKey,
					},
				},
			}),
			Entry("when removePreserveAnnotation is false and ClusterAutoscalerScaleDownDisabledAnnotationByMCM is not present, should not delete any annotations", &testCase{
				setup: setup{
					removePreserveAnnotation: false,
					CAAnnotationPresent:      true,
					CAMCMAnnotationPresent:   false,
				},
				expect: expect{
					err: nil,
					hasAnnotationKeys: []string{
						autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey,
						machineutils.PreserveMachineAnnotationKey,
					},
				},
			}),
		)
	})
})
