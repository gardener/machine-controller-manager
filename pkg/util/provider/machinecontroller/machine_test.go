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
	"fmt"
	"math"
	"time"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	fakemachineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8stesting "k8s.io/client-go/testing"
)

const testNamespace = "test"

var _ = Describe("machine", func() {
	var (
		fakeMachineClient *fakemachineapi.FakeMachineV1alpha1
		c                 *controller
	)

	Describe("#isHealthy", func() {
		BeforeEach(func() {
			fakeMachineClient = &fakemachineapi.FakeMachineV1alpha1{
				Fake: &k8stesting.Fake{},
			}
			c = &controller{
				controlMachineClient: fakeMachineClient,
				nodeConditions:       "ReadonlyFilesystem,KernelDeadlock,DiskPressure",
			}
		})

		testMachine := v1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testmachine",
				Namespace: testNamespace,
			},
			Status: v1alpha1.MachineStatus{
				Conditions: []corev1.NodeCondition{},
			},
		}
		DescribeTable("Checking health of the machine",
			func(conditionType corev1.NodeConditionType, conditionStatus corev1.ConditionStatus, expected bool) {
				testMachine.Status.Conditions = []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   corev1.NodeDiskPressure,
						Status: corev1.ConditionFalse,
					},
					{
						Type:   corev1.NodeMemoryPressure,
						Status: corev1.ConditionFalse,
					},
					{
						Type:   corev1.NodeNetworkUnavailable,
						Status: corev1.ConditionFalse,
					},
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				}
				for i, condition := range testMachine.Status.Conditions {
					if condition.Type == conditionType {
						testMachine.Status.Conditions[i].Status = conditionStatus
						break
					}
				}
				Expect(c.isHealthy(&testMachine)).Should(BeIdenticalTo(expected))
			},
			Entry("with NodeReady is True", corev1.NodeReady, corev1.ConditionTrue, true),
			Entry("with NodeReady is False", corev1.NodeReady, corev1.ConditionFalse, false),
			Entry("with NodeReady is Unknown", corev1.NodeReady, corev1.ConditionUnknown, false),

			Entry("with NodeDiskPressure is True", corev1.NodeDiskPressure, corev1.ConditionTrue, false),
			Entry("with NodeDiskPressure is False", corev1.NodeDiskPressure, corev1.ConditionFalse, true),
			Entry("with NodeDiskPressure is Unknown", corev1.NodeDiskPressure, corev1.ConditionUnknown, false),

			Entry("with NodeMemoryPressure is True", corev1.NodeMemoryPressure, corev1.ConditionTrue, true),
			Entry("with NodeMemoryPressure is Unknown", corev1.NodeMemoryPressure, corev1.ConditionUnknown, true),
			Entry("with NodeMemoryPressure is False", corev1.NodeMemoryPressure, corev1.ConditionFalse, true),

			Entry("with NodeNetworkUnavailable is True", corev1.NodeNetworkUnavailable, corev1.ConditionTrue, true),
			Entry("with NodeNetworkUnavailable is Unknown", corev1.NodeNetworkUnavailable, corev1.ConditionUnknown, true),
			Entry("with NodeNetworkUnavailable is False", corev1.NodeNetworkUnavailable, corev1.ConditionFalse, true),

			Entry("with NodeReady is True", corev1.NodeReady, corev1.ConditionTrue, true),
			Entry("with NodeReady is Unknown", corev1.NodeReady, corev1.ConditionUnknown, false),
			Entry("with NodeReady is False", corev1.NodeReady, corev1.ConditionFalse, false),
		)
	})

	/*
		Describe("##updateMachineConditions", func() {
			Describe("Update conditions of a non-existing machine", func() {
				It("should return error", func() {
					stop := make(chan struct{})
					defer close(stop)

					objects := []runtime.Object{}
					c, trackers := createController(stop, testNamespace, objects, nil, nil)
					defer trackers.Stop()

					testMachine := &v1alpha1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testmachine",
							Namespace: testNamespace,
						},
						Status: v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachineTerminating,
							},
						},
					}
					conditions := []corev1.NodeCondition{}
					var _, err = c.updateMachineConditions(testMachine, conditions)
					Expect(err).Should(Not(BeNil()))
				})
			})
			DescribeTable("Update conditions of an existing machine",
				func(phase v1alpha1.MachinePhase, conditions []corev1.NodeCondition, expectedPhase v1alpha1.MachinePhase) {
					stop := make(chan struct{})
					defer close(stop)

					testMachine := &v1alpha1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testmachine",
							Namespace: testNamespace,
						},
						Status: v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: phase,
							},
						},
					}
					objects := []runtime.Object{}
					objects = append(objects, testMachine)

					c, trackers := createController(stop, testNamespace, objects, nil, nil)
					defer trackers.Stop()

					var updatedMachine, err = c.updateMachineConditions(testMachine, conditions)
					Expect(updatedMachine.Status.Conditions).Should(BeEquivalentTo(conditions))
					Expect(updatedMachine.Status.CurrentStatus.Phase).Should(BeIdenticalTo(expectedPhase))
					Expect(err).Should(BeNil())
				},
				Entry("healthy status but machine terminating", v1alpha1.MachineTerminating, []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				}, v1alpha1.MachineTerminating),
				Entry("unhealthy status but machine running", v1alpha1.MachineRunning, []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionFalse,
					},
				}, v1alpha1.MachineUnknown),
				Entry("healthy status but machine not running", v1alpha1.MachineAvailable, []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				}, v1alpha1.MachineRunning),
			)
		})
	*/

	Describe("#ValidateMachine", func() {
		type data struct {
			action machineapi.Machine
			expect field.ErrorList
		}
		DescribeTable("#happy path",
			func(data *data) {
				errList := validation.ValidateMachine(&data.action)
				Expect(errList).To(Equal(data.expect))
			},
			Entry("aws", &data{
				action: machineapi.Machine{
					Spec: machineapi.MachineSpec{
						Class: machineapi.ClassSpec{
							Kind: "AWSMachineClass",
							Name: "aws",
						},
					},
				},
				expect: field.ErrorList{},
			}),
		)
	})

	Describe("#ValidateMachineClass", func() {
		type setup struct {
			aws     []*v1alpha1.MachineClass
			secrets []*corev1.Secret
		}
		type expect struct {
			machineClass interface{}
			secret       *corev1.Secret
			err          bool
		}
		type data struct {
			setup  setup
			action *v1alpha1.ClassSpec
			expect expect
		}

		objMeta := &metav1.ObjectMeta{
			GenerateName: "class",
			Namespace:    testNamespace,
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				machineObjects := []runtime.Object{}
				for _, o := range data.setup.aws {
					machineObjects = append(machineObjects, o)
				}

				coreObjects := []runtime.Object{}
				for _, o := range data.setup.secrets {
					coreObjects = append(coreObjects, o)
				}

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects, nil)
				defer trackers.Stop()

				waitForCacheSync(stop, controller)
				machineClass, secret, _, err := controller.ValidateMachineClass(data.action)

				if data.expect.machineClass == nil {
					Expect(machineClass).To(BeNil())
				} else {
					Expect(machineClass).To(Equal(data.expect.machineClass))
				}
				if data.expect.secret == nil {
					Expect(secret).To(BeNil())
				} else {
					Expect(secret).To(Equal(data.expect.secret))
				}
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("non-existing machine class", &data{
				setup: setup{
					aws: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
				},
				action: &v1alpha1.ClassSpec{
					Kind: "MachineClass",
					Name: "non-existing",
				},
				expect: expect{
					err: true,
				},
			}),
			Entry("non-existing secret", &data{
				setup: setup{
					secrets: []*corev1.Secret{},
					aws: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
				},
				action: &v1alpha1.ClassSpec{
					Kind: "MachineClass",
					Name: "class-0",
				},
				expect: expect{
					machineClass: &v1alpha1.MachineClass{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						SecretRef:  newSecretReference(objMeta, 0),
					},
					err: false, //TODO Why? Create issue
				},
			}),
			Entry("valid machineClass", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
				},
				action: &v1alpha1.ClassSpec{
					Kind: "MachineClass",
					Name: "class-0",
				},
				expect: expect{
					machineClass: &v1alpha1.MachineClass{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						SecretRef:  newSecretReference(objMeta, 0),
					},
					secret: &corev1.Secret{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					},
					err: false,
				},
			}),
		)
	})

	Describe("#triggerCreationFlow", func() {
		type setup struct {
			machineClaasses     []*v1alpha1.MachineClass
			machines            []*v1alpha1.Machine
			secrets             []*corev1.Secret
			fakeResourceActions *customfake.ResourceActions
		}
		type action struct {
			machine    string
			fakeDriver *driver.FakeDriver
		}
		type expect struct {
			machine *v1alpha1.Machine
			err     error
			retry   machineutils.Retry
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			GenerateName: "machine",
			Namespace:    "test",
		}
		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				machineObjects := []runtime.Object{}
				for _, o := range data.setup.machineClaasses {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range data.setup.machines {
					machineObjects = append(machineObjects, o)
				}

				coreObjects := []runtime.Object{}
				for _, o := range data.setup.secrets {
					coreObjects = append(coreObjects, o)
				}

				fakedriver := driver.NewFakeDriver(data.action.fakeDriver)

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects, fakedriver)
				defer trackers.Stop()

				waitForCacheSync(stop, controller)

				action := data.action
				machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				machineClass, err := controller.controlMachineClient.MachineClasses(objMeta.Namespace).Get(machine.Spec.Class.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				secret, err := controller.targetCoreClient.CoreV1().Secrets(objMeta.Namespace).Get(machineClass.SecretRef.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				retry, err := controller.triggerCreationFlow(
					&driver.CreateMachineRequest{
						Machine:      machine,
						MachineClass: machineClass,
						Secret:       secret,
					},
				)

				if data.expect.err != nil || err != nil {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(data.expect.err))
				}

				actual, err := controller.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(actual.Spec.ProviderID).To(Equal(data.expect.machine.Spec.ProviderID))
				Expect(actual.Status.Node).To(Equal(data.expect.machine.Status.Node))
				Expect(actual.Finalizers).To(Equal(data.expect.machine.Finalizers))
				Expect(retry).To(Equal(data.expect.retry))
			},

			Entry("Machine creation in process. Machine finalizers are UPDATED", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClaasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: v1alpha1.MachineSpec{
							Class: v1alpha1.ClassSpec{
								Kind: "MachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil, false),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   false,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					machine: newMachine(&v1alpha1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: v1alpha1.MachineSpec{
							Class: v1alpha1.ClassSpec{
								Kind: "MachineClass",
								Name: "machineClass",
							},
						},
					}, nil, nil, nil, nil, true),
					err:   fmt.Errorf("Machine creation in process. Machine finalizers are UPDATED"),
					retry: true,
				},
			}),
			Entry("Machine creation succeeds with object UPDATE", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClaasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: v1alpha1.MachineSpec{
							Class: v1alpha1.ClassSpec{
								Kind: "MachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil, true),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   false,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					machine: newMachine(&v1alpha1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: v1alpha1.MachineSpec{
							Class: v1alpha1.ClassSpec{
								Kind: "MachineClass",
								Name: "machineClass",
							},
							ProviderID: "fakeID",
						},
					}, nil, nil, nil, nil, true),
					err:   fmt.Errorf("Machine creation in process. Machine UPDATE successful"),
					retry: true,
				},
			}),
			Entry("Machine creation succeeds with status UPDATE", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClaasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						nil,
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
					err:   fmt.Errorf("Machine creation in process. Machine/Status UPDATE successful"),
					retry: true,
				},
			}),
			Entry("Machine creation has already succeeded, so no update", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClaasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachinePending,

								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Creating machine on cloud provider",
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachinePending,

								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Creating machine on cloud provider",
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
					err:   nil,
					retry: false,
				},
			}),

			/*
				Entry("Machine creation success even on temporary APIServer disruption", &data{
					setup: setup{
						secrets: []*corev1.Secret{
							{
								ObjectMeta: *newObjectMeta(objMeta, 0),
							},
						},
						aws: []*v1alpha1.MachineClass{
							{
								ObjectMeta: *newObjectMeta(objMeta, 0),
								SecretRef:  newSecretReference(objMeta, 0),
							},
						},
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "AWSMachineClass",
									Name: "machine-0",
								},
							},
						}, nil, nil, nil, nil),
						fakeResourceActions: &customfake.ResourceActions{
							Machine: customfake.Actions{
								Get: "Failed to GET machine",
							},
						},
					},
					action: action{
						machine:        "machine-0",
						fakeProviderID: "fakeID-0",
						fakeNodeName:   "fakeNode-0",
						fakeError:      nil,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "AWSMachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						}, &v1alpha1.MachineStatus{
							Node: "fakeNode",
							//TODO conditions
						}, nil, nil, nil),
						err: false,
					},
				}),
			*/
		)
	})

	Describe("#triggerDeletionFlow", func() {
		type setup struct {
			secrets             []*corev1.Secret
			machineClasses      []*v1alpha1.MachineClass
			machines            []*v1alpha1.Machine
			nodes               []*corev1.Node
			fakeResourceActions *customfake.ResourceActions
		}
		type action struct {
			machine                 string
			forceDeleteLabelPresent bool
			fakeMachineStatus       *v1alpha1.MachineStatus
			fakeDriver              *driver.FakeDriver
		}
		type expect struct {
			machine                       *v1alpha1.Machine
			err                           error
			nodeTerminationConditionIsSet bool
			nodeDeleted                   bool
			retry                         machineutils.Retry
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			GenerateName: "machine",
			Namespace:    "test",
		}
		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				machineObjects := []runtime.Object{}
				for _, o := range data.setup.machineClasses {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range data.setup.machines {
					machineObjects = append(machineObjects, o)
				}

				coreObjects := []runtime.Object{}
				for _, o := range data.setup.secrets {
					coreObjects = append(coreObjects, o)
				}
				for _, o := range data.setup.nodes {
					coreObjects = append(coreObjects, o)
				}

				fakeDriver := driver.NewFakeDriver(
					data.action.fakeDriver,
				)

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects, fakeDriver)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				action := data.action
				machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				machineClass, err := controller.controlMachineClient.MachineClasses(objMeta.Namespace).Get(machine.Spec.Class.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				secret, err := controller.targetCoreClient.CoreV1().Secrets(objMeta.Namespace).Get(machineClass.SecretRef.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if data.setup.fakeResourceActions != nil {
					trackers.TargetCore.SetFakeResourceActions(data.setup.fakeResourceActions, math.MaxInt32)
				}

				// Deletion of machine is triggered
				retry, err := controller.triggerDeletionFlow(&driver.DeleteMachineRequest{
					Machine:      machine,
					MachineClass: machineClass,
					Secret:       secret,
				})
				if err != nil || data.expect.err != nil {
					Expect(err).To(Equal(data.expect.err))
				}
				Expect(retry).To(Equal(data.expect.retry))

				machine, err = controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(machine.Spec).To(Equal(data.expect.machine.Spec))
				Expect(machine.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
				Expect(machine.Status.LastOperation.State).To(Equal(data.expect.machine.Status.LastOperation.State))
				Expect(machine.Status.LastOperation.Type).To(Equal(data.expect.machine.Status.LastOperation.Type))
				Expect(machine.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
				Expect(machine.Finalizers).To(Equal(data.expect.machine.Finalizers))

				if data.expect.nodeDeleted {
					_, nodeErr := controller.targetCoreClient.CoreV1().Nodes().Get(machine.Status.Node, metav1.GetOptions{})
					Expect(nodeErr).To(HaveOccurred())
				}
				if data.expect.nodeTerminationConditionIsSet {
					node, nodeErr := controller.targetCoreClient.CoreV1().Nodes().Get(machine.Status.Node, metav1.GetOptions{})
					Expect(nodeErr).To(Not(HaveOccurred()))
					Expect(len(node.Status.Conditions)).To(Equal(1))
					Expect(node.Status.Conditions[0].Type).To(Equal(machineutils.NodeTerminationCondition))
					Expect(node.Status.Conditions[0].Status).To(Equal(v1.ConditionTrue))
				}

			},
			Entry("Do not process machine deletion for object without finalizer", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineRunning,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Machine machine-0 successfully joined the cluster",
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						false,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Machine \"machine-0\" is missing finalizers. Deletion cannot proceed"),
					retry: machineutils.DoNotRetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineRunning,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Machine machine-0 successfully joined the cluster",
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						false,
					),
				},
			}),
			Entry("Change machine phase to termination successfully", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineRunning,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Machine machine-0 successfully joined the cluster",
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Machine deletion in process. Phase set to termination"),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.GetVMStatus,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Checking existance of VM at provider successfully", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.GetVMStatus,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Machine deletion in process. VM with matching ID found"),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Drain machine successfully", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeNode-0",
						},
						true,
					),
					nodes: []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fakeNode-0",
							},
						},
					},
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:                           fmt.Errorf("Machine deletion in process. Drain successful. %s", machineutils.InitiateVMDeletion),
					retry:                         machineutils.RetryOp,
					nodeTerminationConditionIsSet: true,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain successful. %s", machineutils.InitiateVMDeletion),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Drain skipping as nodeName is not valid", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
							Conditions: []corev1.NodeCondition{
								{
									Type:               corev1.NodeReady,
									Status:             corev1.ConditionUnknown,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
								},
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Skipping drain as nodeName is not a valid one for machine. Initiate VM deletion"),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Skipping drain as nodeName is not a valid one for machine. Initiate VM deletion"),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "",
						},
						true,
					),
				},
			}),
			Entry("Drain skipping as machine is NotReady for a long time (5 minutes)", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
							Conditions: []corev1.NodeCondition{
								{
									Type:               corev1.NodeReady,
									Status:             corev1.ConditionUnknown,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
								},
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Skipping drain as machine is NotReady for over 5minutes. %s", machineutils.InitiateVMDeletion),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Skipping drain as machine is NotReady for over 5minutes. %s", machineutils.InitiateVMDeletion),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Drain machine failure, but since force deletion label is present deletion continues", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node":           "fakeID-0",
							"force-deletion": "True",
						},
						true,
					),
					nodes: []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fakeID-0",
							},
						},
					},
					fakeResourceActions: &customfake.ResourceActions{
						Node: customfake.Actions{
							Update: "Failed to update node",
						},
					},
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Failed to update node"),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain failed due to - Failed to update node. However, since it's a force deletion shall continue deletion of VM. %s", machineutils.InitiateVMDeletion),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Drain machine failure after drain timeout, hence deletion continues", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
					nodes: []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fakeID-0",
							},
						},
					},
					fakeResourceActions: &customfake.ResourceActions{
						Node: customfake.Actions{
							Update: "Failed to update node",
						},
					},
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Failed to update node"),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain failed due to - Failed to update node. However, since it's a force deletion shall continue deletion of VM. %s", machineutils.InitiateVMDeletion),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Drain machine failure due to node update failure", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeNode-0",
						},
						true,
					),
					nodes: []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fakeNode-0",
							},
						},
					},
					fakeResourceActions: &customfake.ResourceActions{
						Node: customfake.Actions{
							Update: "Failed to update node",
						},
					},
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("failed to create update conditions for node \"fakeNode-0\": Failed to update node"),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain failed due to failure in update of node conditions - %s. Will retry in next sync. %s", "failed to create update conditions for node \"fakeNode-0\": Failed to update node", machineutils.InitiateDrain),
								State:          v1alpha1.MachineStateFailed,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Delete VM successfully", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain successful. %s", machineutils.InitiateVMDeletion),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Machine deletion in process. VM deletion was successful. " + machineutils.InitiateNodeDeletion),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("VM deletion was successful. %s", machineutils.InitiateNodeDeletion),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Delete node object successfully", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeID",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("VM deletion was successful. %s", machineutils.InitiateNodeDeletion),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
					nodes: []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fakeID-0",
							},
						},
					},
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:         fmt.Errorf("Machine deletion in process. Deletion of node object was succesful"),
					retry:       machineutils.RetryOp,
					nodeDeleted: true,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Deletion of Node Object %q is successful. %s", "fakeID-0", machineutils.InitiateFinalizerRemoval),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
			Entry("Delete machine finalizer successfully", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Deletion of Node Object %q is successful. %s", "fakeID-0", machineutils.InitiateFinalizerRemoval),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					retry: machineutils.DoNotRetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Deletion of Node Object %q is successful. %s", "fakeID-0", machineutils.InitiateFinalizerRemoval),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						false,
					),
				},
			}),
			Entry("Unable to decode deletion flow state for machine", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
					machines: newMachines(
						1,
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Some random last op description",
								State:          v1alpha1.MachineStateFailed,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					err:   fmt.Errorf("Machine deletion in process. Phase set to termination"),
					retry: machineutils.RetryOp,
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machine-0",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Node: "fakeNode",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.GetVMStatus,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							"node": "fakeID-0",
						},
						true,
					),
				},
			}),
		)
	})

	/*
		Describe("#checkMachineTimeout", func() {
			type setup struct {
				machines []*v1alpha1.Machine
			}
			type action struct {
				machine string
			}
			type expect struct {
				machine *v1alpha1.Machine
				err     bool
			}
			type data struct {
				setup  setup
				action action
				expect expect
			}
			objMeta := &metav1.ObjectMeta{
				GenerateName: "machine",
				Namespace:    "test",
			}

			machineName := "machine-0"
			timeOutOccurred := -21 * time.Minute
			timeOutNotOccurred := -5 * time.Minute
			creationTimeOut := 20 * time.Minute
			healthTimeOut := 10 * time.Minute

			DescribeTable("##Machine Timeout Scenarios",
				func(data *data) {
					stop := make(chan struct{})
					defer close(stop)

					machineObjects := []runtime.Object{}
					for _, o := range data.setup.machines {
						machineObjects = append(machineObjects, o)
					}

					coreObjects := []runtime.Object{}

					controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects)
					defer trackers.Stop()
					waitForCacheSync(stop, controller)

					action := data.action
					machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
					//Expect(err).ToNot(HaveOccurred())

					controller.checkMachineTimeout(machine)

					actual, err := controller.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(actual.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
					Expect(actual.Status.CurrentStatus.//TimeoutActive).To(Equal(data.expect.machine.Status.CurrentStatus.//TimeoutActive))
					Expect(actual.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
					Expect(actual.Status.LastOperation.State).To(Equal(data.expect.machine.Status.LastOperation.State))
					Expect(actual.Status.LastOperation.Type).To(Equal(data.expect.machine.Status.LastOperation.Type))
				},
				Entry("Machine is still running", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineRunning,
								//TimeoutActive:  false,
								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Machine % successfully joined the cluster", machineName),
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
							},
						}, nil, nil, nil),
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:         v1alpha1.MachineRunning,
							},
							LastOperation: v1alpha1.LastOperation{
								Description: fmt.Sprintf("Machine % successfully joined the cluster", machineName),
								State:       v1alpha1.MachineStateSuccessful,
								Type:        v1alpha1.MachineOperationCreate,
							},
						}, nil, nil, nil),
					},
				}),
				Entry("Machine creation has still not timed out", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineUnknown,
								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", machineName),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
							},
						}, nil, nil, nil),
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:         v1alpha1.MachineUnknown,
							},
							LastOperation: v1alpha1.LastOperation{
								Description: fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", machineName),
								State:       v1alpha1.MachineStateProcessing,
								Type:        v1alpha1.MachineOperationCreate,
							},
						}, nil, nil, nil),
					},
				}),
				Entry("Machine creation has timed out", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachinePending,
								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Creating machine on cloud provider",
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
							},
						}, nil, nil, nil),
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:         v1alpha1.MachineFailed,

							},
							LastOperation: v1alpha1.LastOperation{
								Description: fmt.Sprintf(
									"Machine %s failed to join the cluster in %s minutes.",
									machineName,
									creationTimeOut,
								),
								State: v1alpha1.MachineStateFailed,
								Type:  v1alpha1.MachineOperationCreate,
							},
						}, nil, nil, nil),
					},
				}),
				Entry("Machine health has timed out", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineUnknown,

								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", machineName),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationHealthCheck,
								LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
							},
						}, nil, nil, nil),
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:         v1alpha1.MachineFailed,

							},
							LastOperation: v1alpha1.LastOperation{
								Description: fmt.Sprintf(
									"Machine %s is not healthy since %s minutes. Changing status to failed. Node Conditions: %+v",
									machineName,
									healthTimeOut,
									[]corev1.NodeCondition{},
								),
								State: v1alpha1.MachineStateFailed,
								Type:  v1alpha1.MachineOperationHealthCheck,
							},
						}, nil, nil, nil),
					},
				}),
			)
		})

		Describe("#updateMachineState", func() {
			type setup struct {
				machines []*v1alpha1.Machine
				nodes    []*corev1.Node
			}
			type action struct {
				machine string
			}
			type expect struct {
				machine *v1alpha1.Machine
				err     bool
			}
			type data struct {
				setup  setup
				action action
				expect expect
			}
			objMeta := &metav1.ObjectMeta{
				GenerateName: "machine",
				// using default namespace for non-namespaced objects
				// as our current fake client is with the assumption
				// that all objects are namespaced
				Namespace: "",
			}

			machineName := "machine-0"

			DescribeTable("##Different machine state update scenrios",
				func(data *data) {
					stop := make(chan struct{})
					defer close(stop)

					machineObjects := []runtime.Object{}
					for _, o := range data.setup.machines {
						machineObjects = append(machineObjects, o)
					}

					coreObjects := []runtime.Object{}
					for _, o := range data.setup.nodes {
						coreObjects = append(coreObjects, o)
					}

					controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects)
					defer trackers.Stop()
					waitForCacheSync(stop, controller)

					action := data.action
					machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())

					controller.updateMachineState(machine)

					actual, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(actual.Name).To(Equal(data.expect.machine.Name))
					Expect(actual.Status.Node).To(Equal(data.expect.machine.Status.Node))
					Expect(actual.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
					Expect(actual.Status.CurrentStatus.//TimeoutActive).To(Equal(data.expect.machine.Status.CurrentStatus.//TimeoutActive))
					Expect(actual.Status.LastOperation.State).To(Equal(data.expect.machine.Status.LastOperation.State))
					Expect(actual.Status.LastOperation.Type).To(Equal(data.expect.machine.Status.LastOperation.Type))
					Expect(actual.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))

					if data.expect.machine.Labels != nil {
						if _, ok := data.expect.machine.Labels["node"]; ok {
							Expect(actual.Labels["node"]).To(Equal(data.expect.machine.Labels["node"]))
						}
					}

					for i := range actual.Status.Conditions {
						Expect(actual.Status.Conditions[i].Type).To(Equal(data.expect.machine.Status.Conditions[i].Type))
						Expect(actual.Status.Conditions[i].Status).To(Equal(data.expect.machine.Status.Conditions[i].Status))
						Expect(actual.Status.Conditions[i].Reason).To(Equal(data.expect.machine.Status.Conditions[i].Reason))
						Expect(actual.Status.Conditions[i].Message).To(Equal(data.expect.machine.Status.Conditions[i].Message))
					}
				},
				Entry("Machine does not have a node backing", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{}, nil, nil, nil),
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{}, nil, nil, nil),
					},
				}),
				Entry("Node object backing machine not found and machine conditions are empty", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							Node: "dummy-node",
						}, nil, nil, nil),
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							Node: "dummy-node",
						}, nil, nil, nil),
					},
				}),
				Entry("Machine is running but node object is lost", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							Node: "dummy-node",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineRunning,
								//TimeoutActive:  false,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Machine % successfully joined the cluster", machineName),
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
							Conditions: []corev1.NodeCondition{
								{
									Message: "kubelet is posting ready status",
									Reason:  "KubeletReady",
									Status:  "True",
									Type:    "Ready",
								},
							},
						}, nil, nil, nil),
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							Node: "dummy-node",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineUnknown,

								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description: fmt.Sprintf(
									"Node object went missing. Machine %s is unhealthy - changing MachineState to Unknown",
									machineName,
								),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationHealthCheck,
								LastUpdateTime: metav1.Now(),
							},
							Conditions: []corev1.NodeCondition{
								{
									Message: "kubelet is posting ready status",
									Reason:  "KubeletReady",
									Status:  "True",
									Type:    "Ready",
								},
							},
						}, nil, nil, nil),
					},
				}),
				Entry("Machine and node both are present and kubelet ready status is updated", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							Node: "machine",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachinePending,

								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Creating machine on cloud provider",
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
							Conditions: []corev1.NodeCondition{
								{
									Message: "kubelet is not ready",
									Reason:  "KubeletReady",
									Status:  "False",
									Type:    "Ready",
								},
							},
						}, nil, nil, nil),
						nodes: []*corev1.Node{
							{
								ObjectMeta: *newObjectMeta(objMeta, 0),
								Status: corev1.NodeStatus{
									Conditions: []corev1.NodeCondition{
										{
											Message: "kubelet is posting ready status",
											Reason:  "KubeletReady",
											Status:  "True",
											Type:    "Ready",
										},
									},
								},
							},
						},
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							Node: "machine",
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineRunning,
								//TimeoutActive:  false,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Machine machine-0 successfully joined the cluster",
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
							Conditions: []corev1.NodeCondition{
								{
									Message: "kubelet is posting ready status",
									Reason:  "KubeletReady",
									Status:  "True",
									Type:    "Ready",
								},
							},
						}, nil, nil, nil),
					},
				}),
				Entry("Machine object does not have node-label and node exists", &data{
					setup: setup{
						machines: newMachines(1, &v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						}, &v1alpha1.MachineStatus{
							Node: "node",
						}, nil, nil, nil),
						nodes: []*corev1.Node{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "node-0",
								},
							},
						},
					},
					action: action{
						machine: machineName,
					},
					expect: expect{
						machine: newMachine(&v1alpha1.MachineTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Name: "machine-0",
							},
						}, &v1alpha1.MachineStatus{
							Node: "node",
						}, nil, nil,
							map[string]string{
								"node": "node-0",
							},
						),
					},
				}),
			)
		})
	*/

})
