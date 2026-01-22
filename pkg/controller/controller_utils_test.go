// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const testNamespace = "test"

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
					machineutils.MachinePriority: strconv.Itoa(1),
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
					machineutils.MachinePriority: strconv.Itoa(5),
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

		sortedPreservedAndUnpreservedMachines := []*machinev1.Machine{
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
						Phase: machinev1.MachineFailed,
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
						Phase: machinev1.MachineFailed,
						PreserveExpiryTime: &metav1.Time{
							Time: time.Now().Add(10 * time.Minute),
						},
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
						Phase: machinev1.MachineRunning,
						PreserveExpiryTime: &metav1.Time{
							Time: time.Now().Add(10 * time.Minute),
						},
					},
				},
				nil,
				nil,
				nil,
			),
		}

		unsortedPreservedAndUnpreservedMachines := []*machinev1.Machine{
			sortedPreservedAndUnpreservedMachines[2].DeepCopy(),
			sortedPreservedAndUnpreservedMachines[0].DeepCopy(),
			sortedPreservedAndUnpreservedMachines[1].DeepCopy(),
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
			Entry("sort on preserved and unpreserved", &data{
				inputMachines:  unsortedPreservedAndUnpreservedMachines,
				outputMachines: sortedPreservedAndUnpreservedMachines,
			}),
		)
	})

	Describe("##AddOrUpdateAnnotationOnNode", func() {
		type setup struct {
			node *corev1.Node
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

				err := AddOrUpdateAnnotationOnNode(context.TODO(), c.targetCoreClient, data.action.nodeName, data.action.toBeAppliedAnnotations)

				if data.expect.err {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).To(BeNil())

				nodeObject, _ = c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), data.action.nodeName, metav1.GetOptions{})

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
						nil,
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
						nil,
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
						nil,
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
						nil,
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
						nil,
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
					expectedAnnotations: nil,
					err:                 false,
				},
			}),
		)
	})

	Describe("##RemoveAnnotationsOffNode", func() {
		type setup struct {
			node *corev1.Node
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

				err := RemoveAnnotationsOffNode(context.TODO(), c.targetCoreClient, data.action.nodeName, data.action.toBeRemovedAnnotations)

				if data.expect.err {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).To(BeNil())

				nodeObject, _ = c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), data.action.nodeName, metav1.GetOptions{})

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
						nil,
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
						nil,
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
						nil,
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
						nil,
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
					expectedAnnotations: nil,
					err:                 false,
				},
			}),
		)
	})
	Describe("##FilterActiveMachineSets", func() {
		testMachineSet := &machinev1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "MachineSet-test",
				Namespace: testNamespace,
				Labels: map[string]string{
					"test-label": "test-label",
				},
				Annotations: map[string]string{
					"deployment.kubernetes.io/revision": "1",
				},
				UID: "1234567",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "MachineDeployment",
						Name:       "MachineDeployment-test",
						UID:        "1234567",
						Controller: nil,
					},
				},
				Generation: 5,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "MachineSet",
				APIVersion: "machine.sapcloud.io/v1alpha1",
			},
			Spec: machinev1.MachineSetSpec{
				Replicas: 0,
				Template: machinev1.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test-label": "test-label",
					},
				},
			},
		}
		It("should return an empty list when machine sets have 0 replicas", func() {
			testMachineSet.Spec.Replicas = 0
			testMachineSets := []*machinev1.MachineSet{
				testMachineSet,
			}
			Expect(FilterActiveMachineSets(testMachineSets)).To(HaveLen(0))
		})
		It("should return expected machine sets when replicas are positive", func() {
			testMachineSet.Spec.Replicas = 1
			testMachineSets := []*machinev1.MachineSet{
				testMachineSet,
			}
			Expect(FilterActiveMachineSets(testMachineSets)).To(HaveLen(1))
		})
	})
})
