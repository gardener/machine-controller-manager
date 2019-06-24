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
	"errors"
	"fmt"
	"time"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	fakemachineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	"github.com/gardener/machine-controller-manager/pkg/driver"
	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
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

	Describe("#updateMachineStatus", func() {
		var (
			machine       *machinev1.Machine
			lastOperation machinev1.LastOperation
			currentStatus machinev1.CurrentStatus
		)

		BeforeEach(func() {
			fakeMachineClient = &fakemachineapi.FakeMachineV1alpha1{
				Fake: &k8stesting.Fake{},
			}
			c = &controller{
				controlMachineClient: fakeMachineClient,
			}
			machine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
				},
			}
			lastOperation = machinev1.LastOperation{
				Description:    "test operation",
				LastUpdateTime: metav1.Now(),
				State:          machinev1.MachineStateProcessing,
				Type:           "Create",
			}
			currentStatus = machinev1.CurrentStatus{
				LastUpdateTime: lastOperation.LastUpdateTime,
				Phase:          machinev1.MachinePending,
				TimeoutActive:  true,
			}
		})

		It("should return error", func() {
			err := errors.New("test error")

			fakeMachineClient.AddReactor("get", "machines", func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, err
			})

			machineRet, errRet := c.updateMachineStatus(machine, lastOperation, currentStatus)
			Expect(errRet).Should(Not(BeNil()))
			Expect(errRet).Should(BeIdenticalTo(err))
			Expect(machineRet).Should(Not(BeNil()))
			Expect(machineRet).Should(BeIdenticalTo(machine))
		})

		It("should return success", func() {
			fakeMachineClient.AddReactor("get", "machines", func(action k8stesting.Action) (bool, runtime.Object, error) {
				if action.(k8stesting.GetAction).GetName() == machine.GetName() {
					return true, machine, nil
				}
				return false, nil, nil
			})

			var machineUpdated *machinev1.Machine
			fakeMachineClient.AddReactor("update", "machines", func(action k8stesting.Action) (bool, runtime.Object, error) {
				o := action.(k8stesting.UpdateAction).GetObject()
				if o == nil {
					return false, nil, nil
				}

				m := o.(*machinev1.Machine)
				if m.GetName() == machine.GetName() {
					machineUpdated = m
					return true, m, nil
				}

				return false, nil, nil

			})

			machineRet, errRet := c.updateMachineStatus(machine, lastOperation, currentStatus)
			Expect(errRet).Should(BeNil())
			Expect(machineRet).Should(Not(BeNil()))
			Expect(machineUpdated).Should(Not(BeNil()))

			Expect(machineUpdated).Should(Not(BeIdenticalTo(machine)))
			Expect(machineRet).Should(Not(BeIdenticalTo(machine)))
			Expect(machineRet).Should(BeIdenticalTo(machineUpdated))
			Expect(machineRet.Status.CurrentStatus).Should(BeIdenticalTo(currentStatus))
			Expect(machineRet.Status.LastOperation).Should(BeIdenticalTo(lastOperation))
		})
	})

	Describe("#isHealthy", func() {
		BeforeEach(func() {
			fakeMachineClient = &fakemachineapi.FakeMachineV1alpha1{
				Fake: &k8stesting.Fake{},
			}
			c = &controller{
				controlMachineClient: fakeMachineClient,
			}
		})

		testMachine := machinev1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testmachine",
				Namespace: testNamespace,
			},
			Status: machinev1.MachineStatus{
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
						Type:   corev1.NodeOutOfDisk,
						Status: corev1.ConditionFalse,
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

			Entry("with NodeOutOfDisk is True", corev1.NodeOutOfDisk, corev1.ConditionTrue, true),
			Entry("with NodeOutOfDisk is Unknown", corev1.NodeOutOfDisk, corev1.ConditionUnknown, true),
			Entry("with NodeOutOfDisk is False", corev1.NodeOutOfDisk, corev1.ConditionFalse, true),

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

	Describe("##updateMachineConditions", func() {
		Describe("Update conditions of a non-existing machine", func() {
			It("should return error", func() {
				stop := make(chan struct{})
				defer close(stop)

				objects := []runtime.Object{}
				c, trackers := createController(stop, testNamespace, objects, nil, nil)
				defer trackers.Stop()

				testMachine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine",
						Namespace: testNamespace,
					},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase: machinev1.MachineTerminating,
						},
					},
				}
				conditions := []corev1.NodeCondition{}
				var _, err = c.updateMachineConditions(testMachine, conditions)
				Expect(err).Should(Not(BeNil()))
			})
		})
		DescribeTable("Update conditions of an existing machine",
			func(phase machinev1.MachinePhase, conditions []corev1.NodeCondition, expectedPhase machinev1.MachinePhase) {
				stop := make(chan struct{})
				defer close(stop)

				testMachine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testmachine",
						Namespace: testNamespace,
					},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
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
			Entry("healthy status but machine terminating", machinev1.MachineTerminating, []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			}, machinev1.MachineTerminating),
			Entry("unhealthy status but machine running", machinev1.MachineRunning, []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			}, machinev1.MachineUnknown),
			Entry("healthy status but machine not running", machinev1.MachineAvailable, []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			}, machinev1.MachineRunning),
		)
	})

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

	Describe("#validateMachineClass", func() {
		type setup struct {
			aws     []*machinev1.AWSMachineClass
			secrets []*corev1.Secret
		}
		type expect struct {
			machineClass interface{}
			secret       *corev1.Secret
			err          bool
		}
		type data struct {
			setup  setup
			action *machinev1.ClassSpec
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

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects)
				defer trackers.Stop()

				waitForCacheSync(stop, controller)
				machineClass, secret, err := controller.validateMachineClass(data.action)

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
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
				},
				action: &machinev1.ClassSpec{
					Kind: "AWSMachineClass",
					Name: "non-existing",
				},
				expect: expect{
					err: true,
				},
			}),
			Entry("non-existing secret", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.AWSMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
				},
				action: &machinev1.ClassSpec{
					Kind: "AWSMachineClass",
					Name: "class-0",
				},
				expect: expect{
					machineClass: &machinev1.AWSMachineClass{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.AWSMachineClassSpec{
							SecretRef: newSecretReference(objMeta, 0),
						},
					},
					err: false, //TODO Why? Create issue
				},
			}),
			Entry("valid", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.AWSMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
				},
				action: &machinev1.ClassSpec{
					Kind: "AWSMachineClass",
					Name: "class-0",
				},
				expect: expect{
					machineClass: &machinev1.AWSMachineClass{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.AWSMachineClassSpec{
							SecretRef: newSecretReference(objMeta, 0),
						},
					},
					err: false,
				},
			}),
		)
	})

	Describe("#machineCreate", func() {
		type setup struct {
			secrets   []*corev1.Secret
			aws       []*machinev1.AWSMachineClass
			openstack []*machinev1.OpenStackMachineClass
			machines  []*machinev1.Machine
		}
		type action struct {
			machine        string
			fakeProviderID string
			fakeNodeName   string
			fakeError      error
		}
		type expect struct {
			machine *machinev1.Machine
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
		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				machineObjects := []runtime.Object{}
				for _, o := range data.setup.aws {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range data.setup.openstack {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range data.setup.machines {
					machineObjects = append(machineObjects, o)
				}

				coreObjects := []runtime.Object{}
				for _, o := range data.setup.secrets {
					coreObjects = append(coreObjects, o)
				}

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects)
				defer trackers.Stop()

				waitForCacheSync(stop, controller)

				action := data.action
				machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				err = controller.machineCreate(machine, driver.NewFakeDriver(
					func() (string, string, error) {
						return action.fakeProviderID, action.fakeNodeName, action.fakeError
					},
					nil, nil))

				if data.expect.err {
					Expect(err).To(HaveOccurred())
					actual, err := controller.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
					Expect(err).To(BeNil())
					Expect(actual.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
					Expect(actual.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
					return
				}

				Expect(err).To(BeNil())
				actual, err := controller.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(actual.Spec).To(Equal(data.expect.machine.Spec))
				Expect(actual.Status.Node).To(Equal(data.expect.machine.Status.Node))
				//TODO Conditions
			},
			Entry("OpenStackSimple", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					openstack: []*machinev1.OpenStackMachineClass{
						&machinev1.OpenStackMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.OpenStackMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "OpenStackMachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil),
				},
				action: action{
					machine:        "machine-0",
					fakeProviderID: "fakeID-0",
					fakeNodeName:   "fakeNode-0",
					fakeError:      fmt.Errorf("Test Error"),
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "OpenStackMachineClass",
								Name: "machine-0",
							},
						},
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase: "Failed",
						},
						LastOperation: machinev1.LastOperation{
							Description: "Cloud provider message - Test Error",
						},
					}, nil, nil, nil),
					err: true,
				},
			}),
			Entry("AWSSimple", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.AWSMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil),
				},
				action: action{
					machine:        "machine-0",
					fakeProviderID: "fakeID-0",
					fakeNodeName:   "fakeNode-0",
					fakeError:      nil,
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "machine-0",
							},
							ProviderID: "fakeID",
						},
					}, &machinev1.MachineStatus{
						Node: "fakeNode",
						//TODO conditions
					}, nil, nil, nil),
					err: false,
				},
			}),
		)
	})

	Describe("#machineDelete", func() {
		type setup struct {
			secrets             []*corev1.Secret
			aws                 []*machinev1.AWSMachineClass
			machines            []*machinev1.Machine
			fakeResourceActions *customfake.ResourceActions
		}
		type action struct {
			machine        string
			fakeProviderID string
			fakeNodeName   string
			fakeError      error
			forceDelete    bool
		}
		type expect struct {
			machine        *machinev1.Machine
			errOccurred    bool
			machineDeleted bool
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
				for _, o := range data.setup.aws {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range data.setup.machines {
					machineObjects = append(machineObjects, o)
				}

				coreObjects := []runtime.Object{}
				for _, o := range data.setup.secrets {
					coreObjects = append(coreObjects, o)
				}

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, nil, coreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				action := data.action
				machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				fakeDriver := driver.NewFakeDriver(
					func() (string, string, error) {
						_, err := controller.targetCoreClient.Core().Nodes().Create(&v1.Node{
							ObjectMeta: metav1.ObjectMeta{
								Name: action.fakeNodeName,
							},
						})
						if err != nil {
							return "", "", err
						}
						return action.fakeProviderID, action.fakeNodeName, action.fakeError
					},
					func() error {
						return nil
					},
					func() (string, error) {
						return action.fakeProviderID, action.fakeError
					},
				)

				// Create a machine that is to be deleted later
				err = controller.machineCreate(machine, fakeDriver)
				Expect(err).ToNot(HaveOccurred())

				// Add finalizers
				controller.addMachineFinalizers(machine)

				// Fetch the latest machine version
				machine, err = controller.controlMachineClient.Machines(objMeta.Namespace).Get(action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if data.action.forceDelete {
					// Add labels for force deletion
					clone := machine.DeepCopy()
					clone.Labels["force-deletion"] = "True"
					machine, err = controller.controlMachineClient.Machines(objMeta.Namespace).Update(clone)
					Expect(err).ToNot(HaveOccurred())
				}

				if data.setup.fakeResourceActions != nil {
					trackers.TargetCore.SetFakeResourceActions(data.setup.fakeResourceActions)
				}

				// Deletion of machine is triggered
				err = controller.machineDelete(machine, fakeDriver)
				if data.expect.errOccurred {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}

				machine, err = controller.controlMachineClient.Machines(machine.Namespace).Get(machine.Name, metav1.GetOptions{})
				if data.expect.machineDeleted {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
					Expect(machine).ToNot(BeNil())
				}
			},
			Entry("Machine deletion", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.AWSMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil),
				},
				action: action{
					machine:        "machine-0",
					fakeProviderID: "fakeID-0",
					fakeNodeName:   "fakeNode-0",
					fakeError:      nil,
				},
				expect: expect{
					errOccurred:    false,
					machineDeleted: true,
				},
			}),
			Entry("Machine deletion when drain fails", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.AWSMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil),
					fakeResourceActions: &customfake.ResourceActions{
						Node: customfake.Actions{
							Update: "Failed to update nodes",
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
					errOccurred:    true,
					machineDeleted: false,
				},
			}),
			Entry("Machine force deletion", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.AWSMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil),
				},
				action: action{
					machine:        "machine-0",
					fakeProviderID: "fakeID-0",
					fakeNodeName:   "fakeNode-0",
					fakeError:      nil,
					forceDelete:    true,
				},
				expect: expect{
					errOccurred:    false,
					machineDeleted: true,
				},
			}),
			Entry("Machine force deletion when drain call fails (APIServer call fails)", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						&corev1.Secret{
							ObjectMeta: *newObjectMeta(objMeta, 0),
						},
					},
					aws: []*machinev1.AWSMachineClass{
						&machinev1.AWSMachineClass{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.AWSMachineClassSpec{
								SecretRef: newSecretReference(objMeta, 0),
							},
						},
					},
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "AWSMachineClass",
								Name: "machine-0",
							},
						},
					}, nil, nil, nil, nil),
					fakeResourceActions: &customfake.ResourceActions{
						Node: customfake.Actions{
							Update: "Failed to update nodes",
						},
					},
				},
				action: action{
					machine:        "machine-0",
					fakeProviderID: "fakeID-0",
					fakeNodeName:   "fakeNode-0",
					fakeError:      nil,
					forceDelete:    true,
				},
				expect: expect{
					errOccurred:    false,
					machineDeleted: true,
				},
			}),
		)
	})

	Describe("#checkMachineTimeout", func() {
		type setup struct {
			machines []*machinev1.Machine
		}
		type action struct {
			machine string
		}
		type expect struct {
			machine *machinev1.Machine
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
				Expect(actual.Status.CurrentStatus.TimeoutActive).To(Equal(data.expect.machine.Status.CurrentStatus.TimeoutActive))
				Expect(actual.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
				Expect(actual.Status.LastOperation.State).To(Equal(data.expect.machine.Status.LastOperation.State))
				Expect(actual.Status.LastOperation.Type).To(Equal(data.expect.machine.Status.LastOperation.Type))
			},
			Entry("Machine is still running", &data{
				setup: setup{
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachineRunning,
							TimeoutActive:  false,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
						},
						LastOperation: machinev1.LastOperation{
							Description:    fmt.Sprintf("Machine % successfully joined the cluster", machineName),
							State:          machinev1.MachineStateSuccessful,
							Type:           machinev1.MachineOperationCreate,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
						},
					}, nil, nil, nil),
				},
				action: action{
					machine: machineName,
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:         machinev1.MachineRunning,
							TimeoutActive: false,
						},
						LastOperation: machinev1.LastOperation{
							Description: fmt.Sprintf("Machine % successfully joined the cluster", machineName),
							State:       machinev1.MachineStateSuccessful,
							Type:        machinev1.MachineOperationCreate,
						},
					}, nil, nil, nil),
				},
			}),
			Entry("Machine creation has still not timed out", &data{
				setup: setup{
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachineUnknown,
							TimeoutActive:  true,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
						},
						LastOperation: machinev1.LastOperation{
							Description:    fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", machineName),
							State:          machinev1.MachineStateProcessing,
							Type:           machinev1.MachineOperationCreate,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutNotOccurred)),
						},
					}, nil, nil, nil),
				},
				action: action{
					machine: machineName,
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:         machinev1.MachineUnknown,
							TimeoutActive: true,
						},
						LastOperation: machinev1.LastOperation{
							Description: fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", machineName),
							State:       machinev1.MachineStateProcessing,
							Type:        machinev1.MachineOperationCreate,
						},
					}, nil, nil, nil),
				},
			}),
			Entry("Machine creation has timed out", &data{
				setup: setup{
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachinePending,
							TimeoutActive:  true,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
						},
						LastOperation: machinev1.LastOperation{
							Description:    "Creating machine on cloud provider",
							State:          machinev1.MachineStateProcessing,
							Type:           machinev1.MachineOperationCreate,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
						},
					}, nil, nil, nil),
				},
				action: action{
					machine: machineName,
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:         machinev1.MachineFailed,
							TimeoutActive: false,
						},
						LastOperation: machinev1.LastOperation{
							Description: fmt.Sprintf(
								"Machine %s failed to join the cluster in %s minutes.",
								machineName,
								creationTimeOut,
							),
							State: machinev1.MachineStateFailed,
							Type:  machinev1.MachineOperationCreate,
						},
					}, nil, nil, nil),
				},
			}),
			Entry("Machine health has timed out", &data{
				setup: setup{
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachineUnknown,
							TimeoutActive:  true,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
						},
						LastOperation: machinev1.LastOperation{
							Description:    fmt.Sprintf("Machine %s is unhealthy - changing MachineState to Unknown", machineName),
							State:          machinev1.MachineStateProcessing,
							Type:           machinev1.MachineOperationHealthCheck,
							LastUpdateTime: metav1.NewTime(time.Now().Add(timeOutOccurred)),
						},
					}, nil, nil, nil),
				},
				action: action{
					machine: machineName,
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase:         machinev1.MachineFailed,
							TimeoutActive: false,
						},
						LastOperation: machinev1.LastOperation{
							Description: fmt.Sprintf(
								"Machine %s is not healthy since %s minutes. Changing status to failed. Node Conditions: %+v",
								machineName,
								healthTimeOut,
								[]corev1.NodeCondition{},
							),
							State: machinev1.MachineStateFailed,
							Type:  machinev1.MachineOperationHealthCheck,
						},
					}, nil, nil, nil),
				},
			}),
		)
	})

	Describe("#updateMachineState", func() {
		type setup struct {
			machines []*machinev1.Machine
			nodes    []*corev1.Node
		}
		type action struct {
			machine string
		}
		type expect struct {
			machine *machinev1.Machine
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
				Expect(actual.Status.CurrentStatus.TimeoutActive).To(Equal(data.expect.machine.Status.CurrentStatus.TimeoutActive))
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
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{}, nil, nil, nil),
				},
				action: action{
					machine: machineName,
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{}, nil, nil, nil),
				},
			}),
			Entry("Node object backing machine not found and machine conditions are empty", &data{
				setup: setup{
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						Node: "dummy-node",
					}, nil, nil, nil),
				},
				action: action{
					machine: machineName,
				},
				expect: expect{
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						Node: "dummy-node",
					}, nil, nil, nil),
				},
			}),
			Entry("Machine is running but node object is lost", &data{
				setup: setup{
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						Node: "dummy-node",
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachineRunning,
							TimeoutActive:  false,
							LastUpdateTime: metav1.Now(),
						},
						LastOperation: machinev1.LastOperation{
							Description:    fmt.Sprintf("Machine % successfully joined the cluster", machineName),
							State:          machinev1.MachineStateSuccessful,
							Type:           machinev1.MachineOperationCreate,
							LastUpdateTime: metav1.Now(),
						},
						Conditions: []corev1.NodeCondition{
							corev1.NodeCondition{
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
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						Node: "dummy-node",
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachineUnknown,
							TimeoutActive:  true,
							LastUpdateTime: metav1.Now(),
						},
						LastOperation: machinev1.LastOperation{
							Description: fmt.Sprintf(
								"Node object went missing. Machine %s is unhealthy - changing MachineState to Unknown",
								machineName,
							),
							State:          machinev1.MachineStateProcessing,
							Type:           machinev1.MachineOperationHealthCheck,
							LastUpdateTime: metav1.Now(),
						},
						Conditions: []corev1.NodeCondition{
							corev1.NodeCondition{
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
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						Node: "machine",
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachinePending,
							TimeoutActive:  true,
							LastUpdateTime: metav1.Now(),
						},
						LastOperation: machinev1.LastOperation{
							Description:    "Creating machine on cloud provider",
							State:          machinev1.MachineStateProcessing,
							Type:           machinev1.MachineOperationCreate,
							LastUpdateTime: metav1.Now(),
						},
						Conditions: []corev1.NodeCondition{
							corev1.NodeCondition{
								Message: "kubelet is not ready",
								Reason:  "KubeletReady",
								Status:  "False",
								Type:    "Ready",
							},
						},
					}, nil, nil, nil),
					nodes: []*corev1.Node{
						&corev1.Node{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Status: corev1.NodeStatus{
								Conditions: []corev1.NodeCondition{
									corev1.NodeCondition{
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
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						Node: "machine",
						CurrentStatus: machinev1.CurrentStatus{
							Phase:          machinev1.MachineRunning,
							TimeoutActive:  false,
							LastUpdateTime: metav1.Now(),
						},
						LastOperation: machinev1.LastOperation{
							Description:    "Machine machine-0 successfully joined the cluster",
							State:          machinev1.MachineStateSuccessful,
							Type:           machinev1.MachineOperationCreate,
							LastUpdateTime: metav1.Now(),
						},
						Conditions: []corev1.NodeCondition{
							corev1.NodeCondition{
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
					machines: newMachines(1, &machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
					}, &machinev1.MachineStatus{
						Node: "node",
					}, nil, nil, nil),
					nodes: []*corev1.Node{
						&corev1.Node{
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
					machine: newMachine(&machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: "machine-0",
						},
					}, &machinev1.MachineStatus{
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

})
