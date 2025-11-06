// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"time"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("safety_logic", func() {

	Describe("#Event Handling functions", func() {
		type setup struct {
			machineObject, newMachineObject *v1alpha1.Machine
		}
		type expect struct {
			expectedQueueSize int
		}
		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##deleteMachineToSafety", func(data *data) {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}

			c, trackers := createController(stop, testNamespace, objects, nil, nil, nil, false)

			defer trackers.Stop()
			//waitForCacheSync(stop, c)
			c.deleteMachineToSafety(data.setup.machineObject)

			//waitForCacheSync(stop, c)
			Expect(c.machineSafetyOrphanVMsQueue.Len()).To(Equal(data.expect.expectedQueueSize))
		},
			Entry("should enqueue the machine key", &data{
				setup: setup{
					machineObject: &v1alpha1.Machine{},
				},
				expect: expect{
					expectedQueueSize: 1,
				},
			}),
		)

		DescribeTable("##updateMachineToSafety", func(data *data) {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}

			c, trackers := createController(stop, testNamespace, objects, nil, nil, nil, false)

			defer trackers.Stop()
			//waitForCacheSync(stop, c)
			c.updateMachineToSafety(data.setup.machineObject, data.setup.newMachineObject)

			//waitForCacheSync(stop, c)
			Expect(c.machineSafetyOrphanVMsQueue.Len()).To(Equal(data.expect.expectedQueueSize))
		},
			Entry("shouldn't enqueue machine key if OutOfRange error not there in last reconciliation", &data{
				setup: setup{
					machineObject:    &v1alpha1.Machine{},
					newMachineObject: &v1alpha1.Machine{},
				},
				expect: expect{
					expectedQueueSize: 0,
				},
			}),
			Entry("shouldn't enqueue machine key if no OutOfRange error in last reconciliation", &data{
				setup: setup{
					machineObject: &v1alpha1.Machine{},
					newMachineObject: &v1alpha1.Machine{
						Status: v1alpha1.MachineStatus{
							LastOperation: v1alpha1.LastOperation{
								Description: "Cloud provider message - machine codes error: code = [NotFound] message = [AWS plugin is returning multiple VM instances backing this machine object. IDs for all backing VMs - [i-1234abcd i-5678efgh]",
							},
						},
					},
				},
				expect: expect{
					expectedQueueSize: 0,
				},
			}),
			Entry("should enqueue machine key if OutOfRange error in last reconciliation", &data{
				setup: setup{
					machineObject: &v1alpha1.Machine{},
					newMachineObject: &v1alpha1.Machine{
						Status: v1alpha1.MachineStatus{
							LastOperation: v1alpha1.LastOperation{
								Description: "Cloud provider message - machine codes error: code = [OutOfRange] message = [AWS plugin is returning multiple VM instances backing this machine object. IDs for all backing VMs - [i-1234abcd i-5678efgh]",
							},
						},
					},
				},
				expect: expect{
					expectedQueueSize: 1,
				},
			}),
			Entry("shouldn't enqueue if new machine obj doesn't have OutOfRange error which old machine obj had", &data{
				setup: setup{
					machineObject: &v1alpha1.Machine{
						Status: v1alpha1.MachineStatus{
							LastOperation: v1alpha1.LastOperation{
								Description: "Cloud provider message - machine codes error: code = [OutOfRange] message = [AWS plugin is returning multiple VM instances backing this machine object. IDs for all backing VMs - [i-1234abcd i-5678efgh]",
							},
						},
					},
					newMachineObject: &v1alpha1.Machine{
						Status: v1alpha1.MachineStatus{
							LastOperation: v1alpha1.LastOperation{
								Description: "Machine abc successfully joined the cluster",
							},
						},
					},
				},
				expect: expect{
					expectedQueueSize: 0,
				},
			}),
		)
	})
	Describe("#machine_safety", func() {

		const (
			zeroDuration        = time.Duration(0)
			fiveSecondsDuration = 5 * time.Second
			fiveMinutesDuration = 5 * time.Minute
		)
		DescribeTable("##reconcileClusterMachineSafetyAPIServer",
			func(
				controlAPIServerIsUp bool,
				targetAPIServerIsUp bool,
				apiServerInactiveDuration time.Duration,
				preMachineControllerIsFrozen bool,
				postMachineControllerFrozen bool,
			) {
				apiServerInactiveStartTime := time.Now().Add(-apiServerInactiveDuration)
				stop := make(chan struct{})
				defer close(stop)

				testMachine := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine1",
						Namespace: testNamespace,
					},
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineUnknown,
						},
					},
				}
				controlMachineObjects := []runtime.Object{}
				controlMachineObjects = append(controlMachineObjects, testMachine)

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, nil, nil, false)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				c.safetyOptions.APIserverInactiveStartTime = apiServerInactiveStartTime
				c.safetyOptions.MachineControllerFrozen = preMachineControllerIsFrozen
				if !controlAPIServerIsUp {
					_ = trackers.ControlMachine.SetError("APIServer is Not Reachable")
					_ = trackers.ControlCore.SetError("APIServer is Not Reachable")
				}
				if !targetAPIServerIsUp {
					_ = trackers.TargetCore.SetError("APIServer is Not Reachable")
				}

				_ = c.reconcileClusterMachineSafetyAPIServer("")

				Expect(c.safetyOptions.MachineControllerFrozen).Should(Equal(postMachineControllerFrozen))
			},

			// Both APIServers are reachable
			Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Inactive, Pre-Frozen: false = Post-Frozen: false",
				true, true, zeroDuration, false, false),
			Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: false",
				true, true, zeroDuration, true, false),
			Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
				true, true, fiveSecondsDuration, false, false),
			Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: false",
				true, true, fiveSecondsDuration, true, false),
			Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: false",
				true, true, fiveMinutesDuration, false, false),
			Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: false",
				true, true, fiveMinutesDuration, true, false),

			// Target APIServer is not reachable
			Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: false = Post-Frozen: false",
				true, false, zeroDuration, false, false),
			Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: true",
				true, false, zeroDuration, true, true),
			Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
				true, false, fiveSecondsDuration, false, false),
			Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: true",
				true, false, fiveSecondsDuration, true, true),
			Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: true",
				true, false, fiveMinutesDuration, false, true),
			Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: true",
				true, false, fiveMinutesDuration, true, true),

			// Control APIServer is not reachable
			Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Inactive, Pre-Frozen: false = Post-Frozen: false",
				false, true, zeroDuration, false, false),
			Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: true",
				false, true, zeroDuration, true, true),
			Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
				false, true, fiveSecondsDuration, false, false),
			Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: true",
				false, true, fiveSecondsDuration, true, true),
			Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: true",
				false, true, fiveMinutesDuration, false, true),
			Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: true",
				false, true, fiveMinutesDuration, true, true),

			// Both APIServers are not reachable
			Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: false = Post-Frozen: false",
				false, false, zeroDuration, false, false),
			Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: true",
				false, false, zeroDuration, true, true),
			Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
				false, false, fiveSecondsDuration, false, false),
			Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: true",
				false, false, fiveSecondsDuration, true, true),
			Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: true",
				false, false, fiveMinutesDuration, false, true),
			Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: true",
				false, false, fiveMinutesDuration, true, true),
		)
	})

	Describe("#Orphan VM collection", func() {
		type setup struct {
			machineObjects     []*v1alpha1.Machine
			machinesOnProvider map[string]string
		}
		type expect struct {
			//machineIds of machines which are expected to be deleted
			toBeDeletedMachines []string
			toBePresentMachines map[string]string
		}
		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##reconcileClusterMachineSafetyOrphanVMs", func(data *data) {
			objMeta := &metav1.ObjectMeta{
				GenerateName: "class",
				Namespace:    testNamespace,
			}

			// classKind := "MachineClass"
			secretData := map[string][]byte{
				"userData":            []byte("dummy-data"),
				"azureClientId":       []byte("dummy-client-id"),
				"azureClientSecret":   []byte("dummy-client-secret"),
				"azureSubscriptionId": []byte("dummy-subcription-id"),
				"azureTenantId":       []byte("dummy-tenant-id"),
			}

			stop := make(chan struct{})
			defer close(stop)

			// Create test secret and add it to controlCoreObject list
			testSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: testNamespace,
				},
				Data: secretData,
			}

			// Create a test secretReference because the method checkMachineClass needs it
			testSecretReference := &corev1.SecretReference{
				Name:      "test-secret",
				Namespace: testNamespace,
			}

			testMachineClass := &v1alpha1.MachineClass{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				SecretRef:  testSecretReference,
			}

			controlCoreObjects := []runtime.Object{}
			controlCoreObjects = append(controlCoreObjects, testSecret)

			controlMachineObjects := []runtime.Object{}
			for _, obj := range data.setup.machineObjects {
				controlMachineObjects = append(controlMachineObjects, obj)
			}

			fakeDriver := driver.NewFakeDriver(false, "", "", "", nil, nil, nil)

			c, trackers := createController(stop, testNamespace, controlMachineObjects, controlCoreObjects, nil, fakeDriver, false)
			defer trackers.Stop()

			fd := fakeDriver.(*driver.FakeDriver)

			listMachinesRequest := &driver.ListMachinesRequest{
				MachineClass: testMachineClass,
				Secret:       testSecret,
			}

			for machineID, machineName := range data.setup.machinesOnProvider {
				_ = fd.AddMachine(machineID, machineName)
			}

			waitForCacheSync(stop, c)

			// call checkMachineClass to delete the orphan VMs
			_ = c.checkMachineClass(context.TODO(), testMachineClass)

			// after this, the testmachine in crashloopbackoff phase
			// should remain and the other one should
			// be deleted because it is an orphan VM
			listMachinesResponse, _ := fd.ListMachines(context.Background(), listMachinesRequest)

			for _, machineID := range data.expect.toBeDeletedMachines {
				Expect(listMachinesResponse.MachineList[machineID]).To(Equal(""))
			}

			for machineID, machineName := range data.expect.toBePresentMachines {
				Expect(listMachinesResponse.MachineList[machineID]).To(Equal(machineName))
			}
		},
			Entry("machine object not found", &data{
				setup: setup{
					machineObjects: nil,
					machinesOnProvider: map[string]string{
						"testmachine-ip1": "testmachine_1",
					},
				},
				expect: expect{
					toBeDeletedMachines: []string{"testmachine-ip1"},
				},
			}),
			Entry("machine object in CrashLoopBackOff state,so machine should NOT be deleted", &data{
				setup: setup{
					machineObjects: []*v1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "testmachine_1",
								Namespace: testNamespace,
							},
							Status: v1alpha1.MachineStatus{
								CurrentStatus: v1alpha1.CurrentStatus{
									Phase: v1alpha1.MachineCrashLoopBackOff,
								},
							},
						},
					},
					machinesOnProvider: map[string]string{
						"testmachine-ip1": "testmachine_1",
					},
				},
				expect: expect{
					toBeDeletedMachines: nil,
					toBePresentMachines: map[string]string{
						"testmachine-ip1": "testmachine_1",
					},
				},
			}),
			Entry("machine object in Running state but doesn't refer to machine, so machine should be deleted", &data{
				setup: setup{
					machineObjects: []*v1alpha1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "testmachine_1",
								Namespace: testNamespace,
							},
							Status: v1alpha1.MachineStatus{
								CurrentStatus: v1alpha1.CurrentStatus{
									Phase: v1alpha1.MachineRunning,
								},
							},
						},
					},
					machinesOnProvider: map[string]string{
						"testmachine-ip1": "testmachine_1",
					},
				},
				expect: expect{
					toBeDeletedMachines: []string{"testmachine-ip1"},
					toBePresentMachines: nil,
				},
			}),
		)
	})

	Describe("#AnnotateNodesUnmanagedByMCM", func() {

		type setup struct {
			node                     *corev1.Node
			associateNodeWithMachine bool
		}
		type action struct {
		}
		type expect struct {
			node0, node1, node2 *corev1.Node
			retry               machineutils.RetryPeriod
			err                 error
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

				targetCoreObjects := []runtime.Object{}
				controlMachineObjects := []runtime.Object{}

				if data.setup.associateNodeWithMachine {
					//optional machine object for test-node-1 ie data.setup.node
					testMachineObject := &v1alpha1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testmachine_0",
							Namespace: testNamespace,
							Labels: map[string]string{
								v1alpha1.NodeLabelKey: data.setup.node.Name,
							},
						},
						Status: v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachineRunning,
							},
						},
					}
					controlMachineObjects = append(controlMachineObjects, testMachineObject)
				}

				//machine object for test-node-1
				testMachineObject := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine_1",
						Namespace: testNamespace,
						Labels: map[string]string{
							v1alpha1.NodeLabelKey: "test-node-1",
						},
					},
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineRunning,
						},
					},
				}
				controlMachineObjects = append(controlMachineObjects, testMachineObject)

				//first machine object for test-node-2
				testMachineObject = &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine_2",
						Namespace: testNamespace,
						Labels: map[string]string{
							v1alpha1.NodeLabelKey: "test-node-2",
						},
					},
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineRunning,
						},
					},
				}
				controlMachineObjects = append(controlMachineObjects, testMachineObject)

				//second machine object for test-node-3
				testMachineObject = &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine_3",
						Namespace: testNamespace,
						Labels: map[string]string{
							v1alpha1.NodeLabelKey: "test-node-2",
						},
					},
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineRunning,
						},
					},
				}
				controlMachineObjects = append(controlMachineObjects, testMachineObject)

				//node without any backing machine object
				nodeObject0 := data.setup.node
				targetCoreObjects = append(targetCoreObjects, nodeObject0)

				//node with 1 backing machine object
				nodeObject1 := &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-1",
						Annotations: map[string]string{
							"anno1": "value1",
						},
					},
				}
				targetCoreObjects = append(targetCoreObjects, nodeObject1)

				//node with 2 backing machine object
				nodeObject2 := &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-2",
						Annotations: map[string]string{
							"anno1": "value1",
						},
					},
				}
				targetCoreObjects = append(targetCoreObjects, nodeObject2)

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				err := c.AnnotateNodesUnmanagedByMCM(context.TODO())

				waitForCacheSync(stop, c)

				// Expect(retry).To(Equal(data.expect.retry))

				if data.expect.err == nil {
					Expect(err).ShouldNot(HaveOccurred())
				} else {
					Expect(err).To(Equal(data.expect.err))
				}
				updatedNodeObject0, _ := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), nodeObject0.Name, metav1.GetOptions{})
				updatedNodeObject1, _ := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), nodeObject1.Name, metav1.GetOptions{})
				updatedNodeObject2, _ := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), nodeObject2.Name, metav1.GetOptions{})

				Expect(updatedNodeObject0.Annotations).Should(Equal(data.expect.node0.Annotations))
				Expect(updatedNodeObject1.Annotations).Should(Equal(data.expect.node1.Annotations))
				Expect(updatedNodeObject2.Annotations).Should(Equal(data.expect.node2.Annotations))
			},

			Entry("Annotate node object when creation time is older than machineCreationTimeout (21mins old)", &data{
				setup: setup{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "value1",
							},
							CreationTimestamp: metav1.NewTime(metav1.Now().Add(-21 * time.Minute)),
						},
					},
				},
				action: action{},
				expect: expect{
					node0: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1":                      "value1",
								machineutils.NotManagedByMCM: "1",
							},
						},
					},
					node1: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-1",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					node2: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-2",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					retry: machineutils.LongRetry,
					err:   nil,
				},
			}),
			Entry("Don't annotate node object when creation time is less than machineCreationTimeout (19mins old)", &data{
				setup: setup{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "value1",
							},
							CreationTimestamp: metav1.NewTime(metav1.Now().Add(-19 * time.Minute)),
						},
					},
				},
				action: action{},
				expect: expect{
					node0: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					node1: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-1",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					node2: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-2",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					retry: machineutils.LongRetry,
					err:   nil,
				},
			}),
			Entry("Node already has NotManagedByMCM annotation", &data{
				setup: setup{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1":                      "value1",
								machineutils.NotManagedByMCM: "1",
							},
						},
					},
				},
				action: action{},
				expect: expect{
					node0: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1":                      "value1",
								machineutils.NotManagedByMCM: "1",
							},
						},
					},
					node1: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-1",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					node2: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-2",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					retry: machineutils.LongRetry,
					err:   nil,
				},
			}),
			Entry("Node incorrectly assigned NotManagedByMCM annotation", &data{
				setup: setup{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1":                      "value1",
								machineutils.NotManagedByMCM: "1",
							},
						},
					},
					associateNodeWithMachine: true,
				},
				action: action{},
				expect: expect{
					node0: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							// node0 no longer has NotManagedByMCM since it has a backing machine object.
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					node1: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-1",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					node2: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-2",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					retry: machineutils.LongRetry,
					err:   nil,
				},
			}),
		)
	})
})
