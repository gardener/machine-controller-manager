/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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
	"time"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("safety_logic", func() {

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

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				c.safetyOptions.APIserverInactiveStartTime = apiServerInactiveStartTime
				c.safetyOptions.MachineControllerFrozen = preMachineControllerIsFrozen
				if !controlAPIServerIsUp {
					trackers.ControlMachine.SetError("APIServer is Not Reachable")
					trackers.ControlCore.SetError("APIServer is Not Reachable")
				}
				if !targetAPIServerIsUp {
					trackers.TargetCore.SetError("APIServer is Not Reachable")
				}

				c.reconcileClusterMachineSafetyAPIServer("")

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

	Describe("machineCrashloopBackoff", func() {
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

		Describe("machineCrashloopBackoff", func() {

			It("Should delete the machine (old code)", func() {
				stop := make(chan struct{})
				defer close(stop)

				// Create test secret and add it to controlCoreObject list
				testSecret := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: testNamespace,
					},
					Data: secretData,
				}

				// Create a test secretReference because the method checkMachineClass needs it
				testSecretReference := &v1.SecretReference{
					Name:      "test-secret",
					Namespace: testNamespace,
				}

				testMachineClass := &v1alpha1.MachineClass{
					ObjectMeta: *newObjectMeta(objMeta, 0),
					SecretRef:  testSecretReference,
				}

				controlCoreObjects := []runtime.Object{}
				controlCoreObjects = append(controlCoreObjects, testSecret)

				// Create test machine object in CrashloopBackoff state
				testMachineObject1 := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine_1",
						Namespace: testNamespace,
					},
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineCrashLoopBackOff,
						},
					},
				}

				// Create another test machine object in Running state
				testMachineObject2 := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine_2",
						Namespace: testNamespace,
					},
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineRunning,
						},
					},
				}

				controlMachineObjects := []runtime.Object{}
				controlMachineObjects = append(controlMachineObjects, testMachineObject1)
				controlMachineObjects = append(controlMachineObjects, testMachineObject2)

				fakeDriver := driver.NewFakeDriver(false, "", "", "", nil, nil)

				c, trackers := createController(stop, testNamespace, controlMachineObjects, controlCoreObjects, nil, fakeDriver)
				defer trackers.Stop()

				fd := fakeDriver.(*driver.FakeDriver)

				listMachinesRequest := &driver.ListMachinesRequest{
					MachineClass: testMachineClass,
					Secret:       testSecret,
				}

				_ = fd.AddMachine("testmachine-ip1", "testmachine_1")
				_ = fd.AddMachine("testmachine-ip2", "testmachine_2")
				waitForCacheSync(stop, c)

				// call checkMachineClass to delete the orphan VMs
				_, _ = c.checkMachineClass(context.TODO(), testMachineClass)

				// after this, the testmachine in crashloopbackoff phase
				// should remain and the other one should
				// be deleted because it is an orphan VM
				listMachinesResponse, _ := fd.ListMachines(context.Background(), listMachinesRequest)

				Expect(listMachinesResponse.MachineList["testmachine-ip1"]).To(Equal("testmachine_1"))
				Expect(listMachinesResponse.MachineList["testmachine-ip2"]).To(Equal(""))
			})
		})
	})

	Describe("#AnnotateNodesUnmanagedByMCM", func() {

		type setup struct {
			node *corev1.Node
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

				//machine object for test-node-1
				testMachineObject := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine_1",
						Namespace: testNamespace,
						Labels: map[string]string{
							"node": "test-node-1",
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
							"node": "test-node-2",
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
							"node": "test-node-2",
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

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				retry, err := c.AnnotateNodesUnmanagedByMCM(context.TODO())

				waitForCacheSync(stop, c)

				Expect(retry).To(Equal(data.expect.retry))

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
		)
	})
})
