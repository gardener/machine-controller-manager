/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

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
	"sort"
	"strconv"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("#controllerUtils", func() {
	Describe("##activeMachines", func() {
		type data struct {
			inputMachines, outputMachines []*machinev1.Machine
		}
		objMeta := &metav1.ObjectMeta{
			GenerateName: "machine-",
			Namespace:    testNamespace,
		}

		sortedMachinesInOrderOfPriorityAnnotation := []*machinev1.Machine{
			newMachine(
				&machinev1.MachineTemplateSpec{
					ObjectMeta: *newObjectMeta(objMeta, 0),
					Spec: machinev1.MachineSpec{
						Class: machinev1.ClassSpec{
							Kind: AWSMachineClass,
							Name: TestMachineClass,
						},
					},
				},
				&machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: machinev1.MachineRunning,
					},
				},
				nil,
				map[string]string{
					MachinePriority: strconv.Itoa(1),
				},
				nil,
			),
			newMachine(
				&machinev1.MachineTemplateSpec{
					ObjectMeta: *newObjectMeta(objMeta, 0),
					Spec: machinev1.MachineSpec{
						Class: machinev1.ClassSpec{
							Kind: AWSMachineClass,
							Name: TestMachineClass,
						},
					},
				},
				&machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: machinev1.MachinePending,
					},
				},
				nil,
				nil,
				nil,
			),
			newMachine(
				&machinev1.MachineTemplateSpec{
					ObjectMeta: *newObjectMeta(objMeta, 0),
					Spec: machinev1.MachineSpec{
						Class: machinev1.ClassSpec{
							Kind: AWSMachineClass,
							Name: TestMachineClass,
						},
					},
				},
				&machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: machinev1.MachineTerminating,
					},
				},
				nil,
				map[string]string{
					MachinePriority: strconv.Itoa(5),
				},
				nil,
			),
		}

		unsortedMachinesInOrderOfPriorityAnnotation := []*machinev1.Machine{
			sortedMachinesInOrderOfPriorityAnnotation[2].DeepCopy(),
			sortedMachinesInOrderOfPriorityAnnotation[0].DeepCopy(),
			sortedMachinesInOrderOfPriorityAnnotation[1].DeepCopy(),
		}

		sortedMachinesInOrderOfPhase := []*machinev1.Machine{
			newMachine(&machinev1.MachineTemplateSpec{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: AWSMachineClass,
						Name: TestMachineClass,
					},
				},
			}, &machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachineTerminating,
				},
			}, nil, nil, nil),
			newMachine(&machinev1.MachineTemplateSpec{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: AWSMachineClass,
						Name: TestMachineClass,
					},
				},
			}, &machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachineFailed,
				},
			}, nil, nil, nil),
			newMachine(&machinev1.MachineTemplateSpec{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: AWSMachineClass,
						Name: TestMachineClass,
					},
				},
			}, &machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachineUnknown,
				},
			}, nil, nil, nil),
			newMachine(&machinev1.MachineTemplateSpec{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: AWSMachineClass,
						Name: TestMachineClass,
					},
				},
			}, &machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachinePending,
				},
			}, nil, nil, nil),
			newMachine(&machinev1.MachineTemplateSpec{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: AWSMachineClass,
						Name: TestMachineClass,
					},
				},
			}, &machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachineAvailable,
				},
			}, nil, nil, nil),
			newMachine(&machinev1.MachineTemplateSpec{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: AWSMachineClass,
						Name: TestMachineClass,
					},
				},
			}, &machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachineRunning,
				},
			}, nil, nil, nil),
		}

		unsortedMachinesInOrderOfPhase := []*machinev1.Machine{
			sortedMachinesInOrderOfPhase[5].DeepCopy(),
			sortedMachinesInOrderOfPhase[4].DeepCopy(),
			sortedMachinesInOrderOfPhase[1].DeepCopy(),
			sortedMachinesInOrderOfPhase[3].DeepCopy(),
			sortedMachinesInOrderOfPhase[2].DeepCopy(),
			sortedMachinesInOrderOfPhase[0].DeepCopy(),
		}

		sortedMachinesInOrderOfCreationTimeStamp := newMachines(3, &machinev1.MachineTemplateSpec{
			ObjectMeta: *newObjectMeta(objMeta, 0),
			Spec: machinev1.MachineSpec{
				Class: machinev1.ClassSpec{
					Kind: AWSMachineClass,
					Name: TestMachineClass,
				},
			},
		}, nil, nil, nil, nil)
		unsortedMachinesInOrderOfCreationTimeStamp := []*machinev1.Machine{
			sortedMachinesInOrderOfCreationTimeStamp[1].DeepCopy(),
			sortedMachinesInOrderOfCreationTimeStamp[0].DeepCopy(),
			sortedMachinesInOrderOfCreationTimeStamp[2].DeepCopy(),
		}

		DescribeTable("###sort",
			func(data *data) {
				sort.Sort(ActiveMachines(data.inputMachines))
				Expect(len(data.inputMachines)).To(Equal(len(data.outputMachines)))
				Expect(data.inputMachines).To(Equal(data.outputMachines))
			},
			Entry("sort on priority annotation", &data{
				inputMachines:  unsortedMachinesInOrderOfPriorityAnnotation,
				outputMachines: sortedMachinesInOrderOfPriorityAnnotation,
			}),
			Entry("sort on phase", &data{
				inputMachines:  unsortedMachinesInOrderOfPhase,
				outputMachines: sortedMachinesInOrderOfPhase,
			}),
			Entry("sort on creation timestamp", &data{
				inputMachines:  unsortedMachinesInOrderOfCreationTimeStamp,
				outputMachines: sortedMachinesInOrderOfCreationTimeStamp,
			}),
		)
	})

	Describe("##AddOrUpdateAnnotationOnNode", func() {
		type setup struct {
			node *v1.Node
		}
		type expect struct {
			expectedAnnotations map[string]string
			err                 bool
		}
		type action struct {
			toBeAppliedAnnotations  map[string]string
			nodeName                string
			nodeExistingAnnotations map[string]string
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
				targetObjects := []runtime.Object{}
				// Name of the node is node-0.
				nodeObject := data.setup.node

				nodeObject.Annotations = data.action.nodeExistingAnnotations
				targetObjects = append(targetObjects, nodeObject)

				c, trackers := createController(stop, testNamespace, controlObjects, nil, targetObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				err := AddOrUpdateAnnotationOnNode(c.targetCoreClient, data.action.nodeName, data.action.toBeAppliedAnnotations)

				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					return
				}

				nodeObject, _ = c.targetCoreClient.CoreV1().Nodes().Get(data.action.nodeName, metav1.GetOptions{})

				// Merge the annotations in the newNodeObject.
				annotationsOnNewNodeObject := make(map[string]string)
				if nodeObject != nil {
					annotationsOnNewNodeObject = nodeObject.Annotations
				}

				Expect(data.expect.expectedAnnotations).To(Equal(annotationsOnNewNodeObject))
			},

			Entry("given annotation should be added when there are certain annotations on node", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName: "node-0",
					nodeExistingAnnotations: map[string]string{
						"anno1": "anno1",
					},
				},
				expect: expect{
					expectedAnnotations: map[string]string{
						"anno0": "anno0",
						"anno1": "anno1",
					},
					err: false,
				},
			}),
			Entry("given annotation should be added when there are no annotations on node", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName:                "node-0",
					nodeExistingAnnotations: map[string]string{},
				},
				expect: expect{
					expectedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					err: false,
				},
			}),
			Entry("given annotation should be added when the same annotations already exists.", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName: "node-0",
					nodeExistingAnnotations: map[string]string{
						"anno0": "anno0",
					},
				},
				expect: expect{
					expectedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					err: false,
				},
			}),
			Entry("given annotation should be updated when the same annotations already exists with different value", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName: "node-0",
					nodeExistingAnnotations: map[string]string{
						"anno0": "annoDummy",
					},
				},
				expect: expect{
					expectedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					err: false,
				},
			}),
			Entry("Error should not be thrown when the node doesnt exist", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName: "node-dummy",
					nodeExistingAnnotations: map[string]string{
						"anno0": "annoDummy",
					},
				},
				expect: expect{
					expectedAnnotations: map[string]string{},
					err:                 false,
				},
			}),
		)
	})

	Describe("##RemoveAnnotationsOffNode", func() {
		type setup struct {
			node *v1.Node
		}
		type expect struct {
			expectedAnnotations map[string]string
			err                 bool
		}
		type action struct {
			toBeRemovedAnnotations  map[string]string
			nodeName                string
			nodeExistingAnnotations map[string]string
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
				targetObjects := []runtime.Object{}
				// Name of the node is node-0.
				nodeObject := data.setup.node

				nodeObject.Annotations = data.action.nodeExistingAnnotations
				targetObjects = append(targetObjects, nodeObject)

				c, trackers := createController(stop, testNamespace, controlObjects, nil, targetObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				err := RemoveAnnotationsOffNode(c.targetCoreClient, data.action.nodeName, data.action.toBeRemovedAnnotations)

				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
					return
				}

				nodeObject, _ = c.targetCoreClient.CoreV1().Nodes().Get(data.action.nodeName, metav1.GetOptions{})

				// Merge the annotations in the newNodeObject.
				annotationsOnNewNodeObject := make(map[string]string)
				if nodeObject != nil {
					annotationsOnNewNodeObject = nodeObject.Annotations
				}

				Expect(data.expect.expectedAnnotations).To(Equal(annotationsOnNewNodeObject))
			},

			Entry("given annotations should be removed when there are same annotations on node", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeRemovedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName: "node-0",
					nodeExistingAnnotations: map[string]string{
						"anno1": "anno1",
						"anno0": "anno0",
					},
				},
				expect: expect{
					expectedAnnotations: map[string]string{
						"anno1": "anno1",
					},
					err: false,
				},
			}),
			Entry("given annotations should be removed when there are no annotations on node", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeRemovedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName:                "node-0",
					nodeExistingAnnotations: map[string]string{},
				},
				expect: expect{
					expectedAnnotations: map[string]string{},
					err:                 false,
				},
			}),
			Entry("given annotations should be removed when there are annotations on node with different value", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeRemovedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName: "node-0",
					nodeExistingAnnotations: map[string]string{
						"anno0": "annodummy",
						"anno1": "anno1",
					},
				},
				expect: expect{
					expectedAnnotations: map[string]string{
						"anno1": "anno1",
					},
					err: false,
				},
			}),
			Entry("error should be thrown when there is no node-object", &data{
				setup: setup{
					node: newNode(
						1,
						&corev1.NodeSpec{},
						nil,
					),
				},
				action: action{
					toBeRemovedAnnotations: map[string]string{
						"anno0": "anno0",
					},
					nodeName: "node-dummy",
					nodeExistingAnnotations: map[string]string{
						"anno0": "annodummy",
						"anno1": "anno1",
					},
				},
				expect: expect{
					expectedAnnotations: map[string]string{},
					err:                 false,
				},
			}),
		)
	})
})
