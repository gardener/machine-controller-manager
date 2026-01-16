// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	k8stesting "k8s.io/client-go/testing"
	"math"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	fakemachineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

const testNamespace = "test"

var _ = Describe("machine", func() {
	var (
		fakeMachineClient *fakemachineapi.FakeMachineV1alpha1
		c                 *controller
		testMachine       v1alpha1.Machine
		testNode          corev1.Node
	)

	Describe("#isHealthy", func() {
		BeforeEach(func() {
			fakeMachineClient = &fakemachineapi.FakeMachineV1alpha1{
				Fake: &k8stesting.Fake{},
			}
			c = &controller{
				controlMachineClient: fakeMachineClient,
				nodeConditions:       "ReadonlyFilesystem,KernelDeadlock,DiskPressure,NetworkUnavailable",
			}
			testMachine = v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testmachine",
					Namespace: testNamespace,
				},
				Status: v1alpha1.MachineStatus{
					Conditions: []corev1.NodeCondition{
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
					},
				},
			}
		})

		DescribeTable("Checking health of the machine",
			func(conditionType corev1.NodeConditionType, conditionStatus corev1.ConditionStatus, expected bool) {
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
			Entry("with NodeMemoryPressure is False", corev1.NodeMemoryPressure, corev1.ConditionFalse, true),
			Entry("with NodeMemoryPressure is Unknown", corev1.NodeMemoryPressure, corev1.ConditionUnknown, true),

			Entry("with NodeNetworkUnavailable is True", corev1.NodeNetworkUnavailable, corev1.ConditionTrue, false),
			Entry("with NodeNetworkUnavailable is False", corev1.NodeNetworkUnavailable, corev1.ConditionFalse, true),
			Entry("with NodeNetworkUnavailable is Unknown", corev1.NodeNetworkUnavailable, corev1.ConditionUnknown, false),

			Entry("with NodeReady is True", corev1.NodeReady, corev1.ConditionTrue, true),
			Entry("with NodeReady is False", corev1.NodeReady, corev1.ConditionFalse, false),
			Entry("with NodeReady is Unknown", corev1.NodeReady, corev1.ConditionUnknown, false),
		)
	})

	Describe("#criticalComponentsNotReadyTaintPresent", func() {
		BeforeEach(func() {
			c = &controller{
				controlMachineClient: fakeMachineClient,
				nodeConditions:       "ReadonlyFilesystem,KernelDeadlock,DiskPressure,NetworkUnavailable",
			}
			testNode = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testnode",
				},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{},
				},
			}
		})

		DescribeTable("Checking readiness of the node",
			func(nodeTaints []corev1.Taint, expected bool) {
				testNode.Spec.Taints = nodeTaints
				Expect(criticalComponentsNotReadyTaintPresent(&testNode)).Should(BeIdenticalTo(expected))
			},
			Entry("with no taints is False", nil, false),
			Entry("with empty taints is False", []corev1.Taint{}, false),
			Entry("with unrelated taints is False", []corev1.Taint{{Key: "unrelated", Effect: corev1.TaintEffectNoSchedule}}, false),
			Entry("with critical-components-not-ready taint is True", []corev1.Taint{{Key: "node.gardener.cloud/critical-components-not-ready", Effect: corev1.TaintEffectNoSchedule}}, true),
		)
	})

	Describe("addedInPlaceUpdateLabels", func() {
		type testCase struct {
			oldNode  *corev1.Node
			node     *corev1.Node
			expected bool
		}

		DescribeTable("##table",
			func(tc testCase) {
				result := inPlaceUpdateLabelsChanged(tc.oldNode, tc.node)
				Expect(result).To(Equal(tc.expected))
			},
			Entry("both nodes are nil", testCase{
				oldNode:  nil,
				node:     nil,
				expected: false,
			}),
			Entry("oldNode is nil", testCase{
				oldNode:  nil,
				node:     &corev1.Node{},
				expected: false,
			}),
			Entry("node is nil", testCase{
				oldNode:  &corev1.Node{},
				node:     nil,
				expected: false,
			}),
			Entry("no labels added or changed", testCase{
				oldNode:  &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"someLabel": "someValue"}}},
				node:     &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"someLabel": "someValue"}}},
				expected: false,
			}),
			Entry("LabelKeyNodeCandidateForUpdate added", testCase{
				oldNode:  &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}},
				node:     &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha1.LabelKeyNodeCandidateForUpdate: "true"}}},
				expected: true,
			}),
			Entry("LabelKeyNodeSelectedForUpdate added", testCase{
				oldNode:  &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}},
				node:     &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha1.LabelKeyNodeSelectedForUpdate: "true"}}},
				expected: true,
			}),
			Entry("LabelKeyNodeUpdateResult added", testCase{
				oldNode:  &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}},
				node:     &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha1.LabelKeyNodeUpdateResult: "success"}}},
				expected: true,
			}),
			Entry("LabelKeyNodeUpdateResult changed", testCase{
				oldNode:  &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha1.LabelKeyNodeUpdateResult: "failure"}}},
				node:     &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha1.LabelKeyNodeUpdateResult: "success"}}},
				expected: true,
			}),
			Entry("LabelKeyNodeUpdateResult unchanged", testCase{
				oldNode:  &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha1.LabelKeyNodeUpdateResult: "success"}}},
				node:     &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha1.LabelKeyNodeUpdateResult: "success"}}},
				expected: false,
			}),
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
							Kind: "MachineClass",
							Name: "aws",
						},
					},
				},
				expect: field.ErrorList{},
			}),
		)
		DescribeTable("#machine validation fails with no class name",
			func(data *data) {
				errList := validation.ValidateMachine(&data.action)
				Expect(errList).To(Equal(data.expect))
			},
			Entry("aws", &data{
				action: machineapi.Machine{
					Spec: machineapi.MachineSpec{
						Class: machineapi.ClassSpec{
							Kind: "MachineClass",
							Name: "",
						},
					},
				},
				expect: field.ErrorList{
					{
						Type:     "FieldValueRequired",
						Field:    "spec.class.name",
						BadValue: "",
						Detail:   "Name is required",
					},
				},
			}),
		)
	})

	Describe("#ValidateMachineClass", func() {
		type setup struct {
			machineClass []*v1alpha1.MachineClass
			secrets      []*corev1.Secret
		}
		type expect struct {
			machineClass any
			secretData   map[string][]byte
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
			Finalizers:   []string{MCMFinalizerName},
		}

		objMetaWithoutFinalizer := &metav1.ObjectMeta{
			GenerateName: "class",
			Namespace:    testNamespace,
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlObjects := []runtime.Object{}
				for _, machineClass := range data.setup.machineClass {
					controlObjects = append(controlObjects, machineClass)
				}

				coreObjects := []runtime.Object{}
				for _, o := range data.setup.secrets {
					coreObjects = append(coreObjects, o)
				}

				controller, trackers := createController(stop, objMeta.Namespace, controlObjects, coreObjects, nil, nil, false)
				defer trackers.Stop()

				waitForCacheSync(stop, controller)
				machineClass, secretData, _, err := controller.ValidateMachineClass(context.TODO(), data.action)

				if data.expect.machineClass == nil {
					Expect(machineClass).To(BeNil())
				} else {
					Expect(machineClass).To(Equal(data.expect.machineClass))
				}
				if data.expect.secretData == nil {
					Expect(secretData).To(BeNil())
				} else {
					Expect(secretData).To(Equal(data.expect.secretData))
				}
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("non-existing machine class", &data{
				setup: setup{
					machineClass: []*v1alpha1.MachineClass{
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
					machineClass: []*v1alpha1.MachineClass{
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
							Data:       map[string][]byte{"foo": []byte("bar")},
						},
					},
					machineClass: []*v1alpha1.MachineClass{
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
					secretData: map[string][]byte{"foo": []byte("bar")},
					err:        false,
				},
			}),
			Entry("machineClass without Finalizer", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"foo": []byte("bar")},
						},
					},
					machineClass: []*v1alpha1.MachineClass{
						{
							ObjectMeta: *newObjectMeta(objMetaWithoutFinalizer, 0),
							SecretRef:  newSecretReference(objMeta, 0),
						},
					},
				},
				action: &v1alpha1.ClassSpec{
					Kind: "MachineClass",
					Name: "class-0",
				},
				expect: expect{
					err: true,
				},
			}),
		)
	})

	Describe("#triggerCreationFlow", func() {
		type setup struct {
			machineClasses      []*v1alpha1.MachineClass
			machines            []*v1alpha1.Machine
			secrets             []*corev1.Secret
			nodes               []*corev1.Node
			fakeResourceActions *customfake.ResourceActions
			noTargetCluster     bool
		}
		type action struct {
			machine    string
			fakeDriver *driver.FakeDriver
		}
		type expect struct {
			machine *v1alpha1.Machine
			err     error
			retry   machineutils.RetryPeriod
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

				controlCoreObjects := []runtime.Object{}
				for _, o := range data.setup.secrets {
					controlCoreObjects = append(controlCoreObjects, o)
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range data.setup.nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				fakedriver := driver.NewFakeDriver(
					data.action.fakeDriver.VMExists,
					data.action.fakeDriver.ProviderID,
					data.action.fakeDriver.NodeName,
					data.action.fakeDriver.LastKnownState,
					data.action.fakeDriver.Addresses,
					data.action.fakeDriver.Err,
					nil,
				)

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, controlCoreObjects, targetCoreObjects, fakedriver, data.setup.noTargetCluster)

				defer trackers.Stop()

				waitForCacheSync(stop, controller)

				action := data.action
				machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(context.TODO(), action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				machineClass, err := controller.controlMachineClient.MachineClasses(objMeta.Namespace).Get(context.TODO(), machine.Spec.Class.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				secret, err := controller.controlCoreClient.CoreV1().Secrets(objMeta.Namespace).Get(context.TODO(), machineClass.SecretRef.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				retry, err := controller.triggerCreationFlow(
					context.TODO(),
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

				actual, err := controller.controlMachineClient.Machines(machine.Namespace).Get(context.TODO(), machine.Name, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(actual.Spec.ProviderID).To(Equal(data.expect.machine.Spec.ProviderID))
				Expect(actual.Finalizers).To(Equal(data.expect.machine.Finalizers))
				Expect(retry).To(Equal(data.expect.retry))
				Expect(actual.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
				Expect(actual.Status.Addresses).To(Equal(data.expect.machine.Status.Addresses))

				if data.expect.machine.Labels == nil {
					Expect(actual.Labels).To(BeNil())
				} else {
					Expect(actual.Labels).To(Equal(data.expect.machine.Labels))
				}
				if data.expect.machine.Status.LastOperation.ErrorCode != "" {
					Expect(actual.Status.LastOperation.ErrorCode).To(Equal(data.expect.machine.Status.LastOperation.ErrorCode))
				} else {
					Expect(actual.Status.LastOperation.ErrorCode).To(Equal(""))
				}
				if data.expect.machine.Status.LastOperation.Description != "" {
					Expect(actual.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
				}
			},

			Entry("Machine creation succeeds with object UPDATE", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.Now()),
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
					}, nil, nil, nil, map[string]string{v1alpha1.NodeLabelKey: "fakeNode-0"}, true, metav1.Now()),
					err:   fmt.Errorf("machine creation in process. Machine initialization (if required) is successful"),
					retry: machineutils.ShortRetry,
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
						nil,
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachinePending,
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
					err:   fmt.Errorf("machine creation in process. Machine/Status UPDATE successful"),
					retry: machineutils.ShortRetry,
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
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
					err:   nil,
					retry: machineutils.LongRetry,
				},
			}),
			Entry("Machine creation fails with CrashLoopBackOff due to Internal error", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.Now()),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists: false,
						Err:      status.Error(codes.Internal, "Provider is returning error on create call"),
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
					}, &v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineCrashLoopBackOff,
						},
						LastOperation: v1alpha1.LastOperation{
							ErrorCode: codes.Internal.String(),
						},
					}, nil, nil, nil, true, metav1.Now()),
					err:   status.Error(codes.Internal, "Provider is returning error on create call"),
					retry: machineutils.MediumRetry,
				},
			}),
			Entry("Machine creation fails with CrashLoopBackOff due to resource exhaustion", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.Now()),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists: false,
						Err:      status.Error(codes.ResourceExhausted, "Provider does not have capacity to create VM"),
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
					}, &v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineCrashLoopBackOff,
						},
						LastOperation: v1alpha1.LastOperation{
							ErrorCode: codes.ResourceExhausted.String(),
						},
					}, nil, nil, nil, true, metav1.Now()),
					err:   status.Error(codes.ResourceExhausted, "Provider does not have capacity to create VM"),
					retry: machineutils.LongRetry,
				},
			}),
			Entry("Machine creation fails with Failure due to timeout", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.NewTime(metav1.Now().Add(-time.Hour))),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists: false,
						Err:      status.Error(codes.Internal, "Provider is returning error on create call"),
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
					}, &v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineFailed,
						},
						LastOperation: v1alpha1.LastOperation{
							ErrorCode: codes.Internal.String(),
						},
					}, nil, nil, nil, true, metav1.NewTime(metav1.Now().Add(-time.Hour))),
					err:   status.Error(codes.Internal, "Provider is returning error on create call"),
					retry: machineutils.MediumRetry,
				},
			}),
			Entry("Machine creation fails with Failure due to VM using stale node obj (case is for Provider AWS only)", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.Now()),
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
					}, &v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineFailed,
						},
					}, nil, nil, nil, true, metav1.Now()),
					err:   nil,
					retry: machineutils.ShortRetry,
				},
			}),
			Entry("CLBF machine turns to Pending if VM is present", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachineCrashLoopBackOff,
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachinePending,
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
					err:   fmt.Errorf("machine creation in process. Machine/Status UPDATE successful"),
					retry: machineutils.ShortRetry,
				},
			}),
			Entry("Machine initialization failed due to VM instance initialization error", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.Now()),
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
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        status.Error(codes.Uninitialized, "VM instance could not be initialized"),
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
					}, &v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase: v1alpha1.MachineCrashLoopBackOff,
						},
						LastOperation: v1alpha1.LastOperation{
							Description: fmt.Sprintf("Provider error: %s. %s", status.Error(codes.Uninitialized, "VM instance could not be initialized").Error(), machineutils.InstanceInitialization),
							ErrorCode:   codes.Uninitialized.String(),
							State:       v1alpha1.MachineStateFailed,
							Type:        v1alpha1.MachineOperationCreate,
						},
					}, nil, nil, map[string]string{v1alpha1.NodeLabelKey: "fakeNode-0"}, true, metav1.Now()),
					err:   status.Error(codes.Uninitialized, "VM instance could not be initialized"),
					retry: machineutils.ShortRetry,
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
			Entry("without target cluster: machine creation and initialization succeeds", &data{
				setup: setup{
					noTargetCluster: true,
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.Now()),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   false,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Addresses: []corev1.NodeAddress{{
							Type:    corev1.NodeInternalIP,
							Address: "10.0.0.1",
						}},
						Err: nil,
					},
				},
				expect: expect{
					machine: newMachine(
						&v1alpha1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: v1alpha1.MachineSpec{
								Class: v1alpha1.ClassSpec{
									Kind: "MachineClass",
									Name: "machineClass",
								},
								ProviderID: "fakeID",
							},
						},
						&v1alpha1.MachineStatus{
							Addresses: []corev1.NodeAddress{
								{
									Type:    corev1.NodeHostName,
									Address: "fakeNode-0",
								},
								{
									Type:    corev1.NodeInternalIP,
									Address: "10.0.0.1",
								},
							},
						},
						nil, nil, map[string]string{}, true, metav1.Now(),
					),
					err:   fmt.Errorf("machine creation in process. Machine initialization (if required) is successful"),
					retry: machineutils.ShortRetry,
				},
			}),
			Entry("without target cluster: machine transitions to Available", &data{
				setup: setup{
					noTargetCluster: true,
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
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
						nil,
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{},
						true,
						metav1.Now(),
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachineAvailable,
							},
							LastOperation: v1alpha1.LastOperation{
								State:       v1alpha1.MachineStateSuccessful,
								Description: "Created machine on cloud provider",
							},
							Addresses: []corev1.NodeAddress{{
								Type:    corev1.NodeHostName,
								Address: "fakeNode-0",
							}},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{}, // expect empty labels – no node label
						true,
						metav1.Now(),
					),
					err:   fmt.Errorf("machine creation in process. Machine/Status UPDATE successful"),
					retry: machineutils.ShortRetry,
				},
			}),
			Entry("without target cluster: Machine is already Available, so no update", &data{
				setup: setup{
					noTargetCluster: true,
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineAvailable,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Created machine on cloud provider",
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
							Addresses: []corev1.NodeAddress{{
								Type:    corev1.NodeHostName,
								Address: "fakeNode-0",
							}},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{},
						true,
						metav1.Now(),
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase: v1alpha1.MachineAvailable,

								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Created machine on cloud provider",
								State:          v1alpha1.MachineStateSuccessful,
								Type:           v1alpha1.MachineOperationCreate,
								LastUpdateTime: metav1.Now(),
							},
							Addresses: []corev1.NodeAddress{{
								Type:    corev1.NodeHostName,
								Address: "fakeNode-0",
							}},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{},
						true,
						metav1.Now(),
					),
					err:   nil,
					retry: machineutils.LongRetry,
				},
			}),
		)
	})

	Describe("#triggerDeletionFlow", func() {
		type setup struct {
			secrets             []*corev1.Secret
			machineClasses      []*v1alpha1.MachineClass
			machines            []*v1alpha1.Machine
			nodes               []*corev1.Node
			fakeResourceActions *customfake.ResourceActions
			noTargetCluster     bool
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
			retry                         machineutils.RetryPeriod
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			GenerateName:      "machine",
			Namespace:         "test",
			CreationTimestamp: metav1.Now(),
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

				controlCoreObjects := []runtime.Object{}
				targetCoreObjects := []runtime.Object{}

				for _, o := range data.setup.secrets {
					controlCoreObjects = append(controlCoreObjects, o)
				}
				for _, o := range data.setup.nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				fakeDriver := driver.NewFakeDriver(
					data.action.fakeDriver.VMExists,
					data.action.fakeDriver.ProviderID,
					data.action.fakeDriver.NodeName,
					data.action.fakeDriver.LastKnownState,
					data.action.fakeDriver.Addresses,
					data.action.fakeDriver.Err,
					nil,
				)

				controller, trackers := createController(stop, objMeta.Namespace, machineObjects, controlCoreObjects, targetCoreObjects, fakeDriver, data.setup.noTargetCluster)

				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				action := data.action
				machine, err := controller.controlMachineClient.Machines(objMeta.Namespace).Get(context.TODO(), action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				machineClass, err := controller.controlMachineClient.MachineClasses(objMeta.Namespace).Get(context.TODO(), machine.Spec.Class.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				secret, err := controller.controlCoreClient.CoreV1().Secrets(objMeta.Namespace).Get(context.TODO(), machineClass.SecretRef.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if data.setup.fakeResourceActions != nil {
					_ = trackers.TargetCore.SetFakeResourceActions(data.setup.fakeResourceActions, math.MaxInt32)
				}

				// Deletion of machine is triggered
				retry, err := controller.triggerDeletionFlow(context.TODO(), &driver.DeleteMachineRequest{
					Machine:      machine,
					MachineClass: machineClass,
					Secret:       secret,
				})
				if err != nil || data.expect.err != nil {
					Expect(err).To(Equal(data.expect.err))
				}
				Expect(retry).To(Equal(data.expect.retry))

				machine, err = controller.controlMachineClient.Machines(objMeta.Namespace).Get(context.TODO(), action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(machine.Spec).To(Equal(data.expect.machine.Spec))
				Expect(machine.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
				Expect(machine.Status.LastOperation.State).To(Equal(data.expect.machine.Status.LastOperation.State))
				Expect(machine.Status.LastOperation.Type).To(Equal(data.expect.machine.Status.LastOperation.Type))
				Expect(machine.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
				Expect(machine.Finalizers).To(Equal(data.expect.machine.Finalizers))

				if data.expect.nodeDeleted {
					_, nodeErr := controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), machine.Labels[v1alpha1.NodeLabelKey], metav1.GetOptions{})
					Expect(nodeErr).To(HaveOccurred())
				}
				if data.expect.nodeTerminationConditionIsSet {
					node, nodeErr := controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), machine.Labels[v1alpha1.NodeLabelKey], metav1.GetOptions{})
					Expect(nodeErr).To(Not(HaveOccurred()))
					Expect(len(node.Status.Conditions)).To(Equal(1))
					Expect(node.Status.Conditions[0].Type).To(Equal(machineutils.NodeTerminationCondition))
					Expect(node.Status.Conditions[0].Status).To(Equal(corev1.ConditionTrue))
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						false,
						metav1.Now(),
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
					retry: machineutils.LongRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						false,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					retry: machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("machine deletion in process. VM with matching ID found"),
					retry: machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeNode-0",
						},
						true,
						metav1.Now(),
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
					err:                           fmt.Errorf("Drain successful. %s", machineutils.InitiateVMDeletion),
					retry:                         machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "",
						},
						true,
						metav1.Now(),
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
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Skipping drain as nodeName is not a valid one for machine. Initiate VM deletion",
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
							v1alpha1.NodeLabelKey: "",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("Force Drain as machine is NotReady for a long time (5 minutes)", &data{
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("%s", fmt.Sprintf("Force Drain successful. %s", machineutils.DelVolumesAttachments)),
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Force Drain successful. %s", machineutils.DelVolumesAttachments),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("Force Drain as machine is in ReadonlyFilesystem for a long time (5 minutes)", &data{
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
									Type:               "ReadonlyFilesystem",
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
								},
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("%s", fmt.Sprintf("Force Drain successful. %s", machineutils.DelVolumesAttachments)),
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Force Drain successful. %s", machineutils.DelVolumesAttachments),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("Force Drain as machine is NotReady for a long time(5 min) ,also ReadonlyFilesystem is true for a long time (5 minutes)", &data{
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
									Type:               "ReadonlyFilesystem",
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
								},
								{
									Type:               corev1.NodeReady,
									Status:             corev1.ConditionFalse,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
								},
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("%s", fmt.Sprintf("Force Drain successful. %s", machineutils.DelVolumesAttachments)),
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Force Drain successful. %s", machineutils.DelVolumesAttachments),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("No Drain skipping as ReadonlyFilesystem is true for a short time(<5min)", &data{
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
									Type:               "ReadonlyFilesystem",
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
								},
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("Drain successful. %s", machineutils.InitiateVMDeletion),
					retry: machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("No Drain skipping as ReadonlyFilesystem is false", &data{
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
									Type:               "ReadonlyFilesystem",
									Status:             corev1.ConditionFalse,
									LastTransitionTime: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
								},
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("Drain successful. %s", machineutils.InitiateVMDeletion),
					retry: machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
							"force-deletion":      "True",
						},
						true,
						metav1.Now(),
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
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain failed due to - Failed to update node. However, since it's a force deletion shall continue deletion of VM. %s", machineutils.DelVolumesAttachments),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("Drain machine failure before drain timeout, hence deletion fails", &data{
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateDrain,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.NewTime(time.Now().Add(-3*time.Minute)),
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
					err:   fmt.Errorf("failed to create/update conditions on node \"fakeID-0\": Failed to update node"),
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain failed due to failure in update of node conditions - %s. Will retry in next sync. %s", "failed to create/update conditions on node \"fakeID-0\": Failed to update node", machineutils.InitiateDrain),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.NewTime(time.Now().Add(-3*time.Hour)),
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
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain failed due to - Failed to update node. However, since it's a force deletion shall continue deletion of VM. %s", machineutils.DelVolumesAttachments),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeNode-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("failed to create/update conditions on node \"fakeNode-0\": Failed to update node"),
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Drain failed due to failure in update of node conditions - %s. Will retry in next sync. %s", "failed to create/update conditions on node \"fakeNode-0\": Failed to update node", machineutils.InitiateDrain),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:   fmt.Errorf("Machine deletion in process. VM deletion was successful. " + machineutils.RemoveNodeFinalizers),
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("VM deletion was successful. %s", machineutils.RemoveNodeFinalizers),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("Delete node finalizers successfully", &data{
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("VM deletion was successful. %s", machineutils.RemoveNodeFinalizers),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
					nodes: []*corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fakeID-0",
								Finalizers: []string{
									NodeFinalizerName,
								},
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
					err:   fmt.Errorf("machine deletion in process. Removal of finalizers from Node Object \"fakeID-0\" is successful. " + machineutils.InitiateNodeDeletion),
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Removal of finalizers from Node Object %q is successful. %s", "fakeID-0", machineutils.InitiateNodeDeletion),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Removal of finalizers from Node Object %q is successful. %s", "fakeID-0", machineutils.InitiateNodeDeletion),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					err:         fmt.Errorf("Machine deletion in process. Deletion of node object was successful"),
					retry:       machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					retry: machineutils.LongRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						false,
						metav1.Now(),
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
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
					retry: machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("without target cluster: checking existence of VM, then jump to VM deletion", &data{
				setup: setup{
					noTargetCluster: true,
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
						map[string]string{}, // no node label
						true,
						metav1.Now(),
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
					retry: machineutils.ShortRetry,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    "Running without target cluster, skipping node drain and volume attachment deletion. " + machineutils.InitiateVMDeletion,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{}, // no node label
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("without target cluster: Delete node object successfully", &data{
				setup: setup{
					noTargetCluster: true,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    machineutils.InitiateNodeDeletion,
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{}, // no node label
						true,
						metav1.Now(),
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
					err:         fmt.Errorf("Machine deletion in process. No node object found"),
					retry:       machineutils.ShortRetry,
					nodeDeleted: false,
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
							CurrentStatus: v1alpha1.CurrentStatus{
								Phase:          v1alpha1.MachineTerminating,
								LastUpdateTime: metav1.Now(),
							},
							LastOperation: v1alpha1.LastOperation{
								Description:    fmt.Sprintf("Label %q not present on machine %q or no associated node object found, continuing deletion flow. %s", v1alpha1.NodeLabelKey, "machine-0", machineutils.InitiateFinalizerRemoval),
								State:          v1alpha1.MachineStateProcessing,
								Type:           v1alpha1.MachineOperationDelete,
								LastUpdateTime: metav1.Now(),
							},
						},
						nil,
						map[string]string{
							machineutils.MachinePriority: "3",
						},
						map[string]string{}, // no node label
						true,
						metav1.Now(),
					),
				},
			}),
		)
	})

	Describe("#pendingMachineCreationMap tests", func() {
		type setup struct {
			secrets             []*corev1.Secret
			machineClasses      []*v1alpha1.MachineClass
			machines            []*v1alpha1.Machine
			nodes               []*corev1.Node
			fakeResourceActions *customfake.ResourceActions
			controller          *controller
			noTargetCluster     bool
		}
		type machineActionRequest struct {
			machine      *v1alpha1.Machine
			machineClass *v1alpha1.MachineClass
			secret       *corev1.Secret
		}
		type action struct {
			machine                 string
			forceDeleteLabelPresent bool
			fakeMachineStatus       *v1alpha1.MachineStatus
			fakeDriver              *driver.FakeDriver
			// These fields are used to change the test scenario (add to pending map, call creation/deletion flow)
			isCreation bool
			testFunc   func(setup, machineActionRequest) (machineutils.RetryPeriod, error)
		}
		type expect struct {
			machine                       *v1alpha1.Machine
			err                           error
			nodeTerminationConditionIsSet bool
			nodeDeleted                   bool
			retry                         machineutils.RetryPeriod
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			GenerateName:      "machine",
			Namespace:         "test",
			CreationTimestamp: metav1.Now(),
		}
		commonSetup := setup{
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
					v1alpha1.NodeLabelKey: "fakeID-0",
				},
				true,
				metav1.Now(),
			),
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

				controlCoreObjects := []runtime.Object{}
				targetCoreObjects := []runtime.Object{}

				for _, o := range data.setup.secrets {
					controlCoreObjects = append(controlCoreObjects, o)
				}
				for _, o := range data.setup.nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				fakeDriver := driver.NewFakeDriver(
					data.action.fakeDriver.VMExists,
					data.action.fakeDriver.ProviderID,
					data.action.fakeDriver.NodeName,
					data.action.fakeDriver.LastKnownState,
					data.action.fakeDriver.Addresses,
					data.action.fakeDriver.Err,
					nil,
				)

				var trackers *customfake.FakeObjectTrackers
				data.setup.controller, trackers = createController(stop, objMeta.Namespace, machineObjects, controlCoreObjects, targetCoreObjects, fakeDriver, data.setup.noTargetCluster)

				defer trackers.Stop()
				waitForCacheSync(stop, data.setup.controller)

				action := data.action
				machine, err := data.setup.controller.controlMachineClient.Machines(objMeta.Namespace).Get(context.TODO(), action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				machineClass, err := data.setup.controller.controlMachineClient.MachineClasses(objMeta.Namespace).Get(context.TODO(), machine.Spec.Class.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				secret, err := data.setup.controller.controlCoreClient.CoreV1().Secrets(objMeta.Namespace).Get(context.TODO(), machineClass.SecretRef.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if data.setup.fakeResourceActions != nil {
					_ = trackers.TargetCore.SetFakeResourceActions(data.setup.fakeResourceActions, math.MaxInt32)
				}

				// ******************************************************
				// This is changing the setup in accordance with the test
				retry, err := data.action.testFunc(data.setup, machineActionRequest{
					machine:      machine,
					machineClass: machineClass,
					secret:       secret,
				})
				// ******************************************************

				if err != nil || data.expect.err != nil {
					Expect(err).To(Equal(data.expect.err))
				}
				Expect(retry).To(Equal(data.expect.retry))

				if data.action.isCreation {
					_, found := data.setup.controller.pendingMachineCreationMap.Load(data.expect.machine.Name)
					Expect(found).To(Equal(false))
				}

				machine, err = data.setup.controller.controlMachineClient.Machines(objMeta.Namespace).Get(context.TODO(), action.machine, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(machine.Spec).To(Equal(data.expect.machine.Spec))
				Expect(machine.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
				Expect(machine.Status.LastOperation.State).To(Equal(data.expect.machine.Status.LastOperation.State))
				Expect(machine.Status.LastOperation.Type).To(Equal(data.expect.machine.Status.LastOperation.Type))
				Expect(machine.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
				Expect(machine.Finalizers).To(Equal(data.expect.machine.Finalizers))

				if data.expect.nodeDeleted {
					_, nodeErr := data.setup.controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), machine.Labels[v1alpha1.NodeLabelKey], metav1.GetOptions{})
					Expect(nodeErr).To(HaveOccurred())
				}
				if data.expect.nodeTerminationConditionIsSet {
					node, nodeErr := data.setup.controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), machine.Labels[v1alpha1.NodeLabelKey], metav1.GetOptions{})
					Expect(nodeErr).To(Not(HaveOccurred()))
					Expect(len(node.Status.Conditions)).To(Equal(1))
					Expect(node.Status.Conditions[0].Type).To(Equal(machineutils.NodeTerminationCondition))
					Expect(node.Status.Conditions[0].Status).To(Equal(corev1.ConditionTrue))
				}
			},
			Entry("Remove machine from pendingMachineCreationMap after it exits creation flow", &data{
				setup: setup{
					secrets: []*corev1.Secret{
						{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Data:       map[string][]byte{"userData": []byte("test")},
						},
					},
					machineClasses: []*v1alpha1.MachineClass{
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
					}, nil, nil, nil, nil, true, metav1.Now()),
				},
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
					isCreation: true,
					testFunc: func(setUp setup, req machineActionRequest) (machineutils.RetryPeriod, error) {
						return setUp.controller.triggerCreationFlow(context.TODO(), &driver.CreateMachineRequest{
							Machine:      req.machine,
							MachineClass: req.machineClass,
							Secret:       req.secret,
						})
					},
				},
				expect: expect{
					machine: newMachine(&v1alpha1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: v1alpha1.MachineSpec{
							Class: v1alpha1.ClassSpec{
								Kind: "MachineClass",
								Name: "machine-0",
							},
							ProviderID: "fakeID",
						},
					}, nil, nil, nil, map[string]string{v1alpha1.NodeLabelKey: "fakeNode-0"}, true, metav1.Now()),
					err:   fmt.Errorf("machine creation in process. Machine labels/annotations update is successful"),
					retry: machineutils.ShortRetry,
				},
			}),
			Entry("Do not process machine deletion for machine present in pendingMachineCreationMap", &data{
				setup: commonSetup,
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
					isCreation: false,
					testFunc: func(setUp setup, req machineActionRequest) (machineutils.RetryPeriod, error) {
						setUp.controller.pendingMachineCreationMap.Store("test/machine-0", "")
						return setUp.controller.triggerDeletionFlow(context.TODO(), &driver.DeleteMachineRequest{
							Machine:      req.machine,
							MachineClass: req.machineClass,
							Secret:       req.secret,
						})
					},
				},
				expect: expect{
					err:   fmt.Errorf("machine \"machine-0\" is in creation flow. Deletion cannot proceed"),
					retry: machineutils.MediumRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
			Entry("Proceed with machine deletion for machine not present in pendingMachineCreationMap", &data{
				setup: commonSetup,
				action: action{
					machine: "machine-0",
					fakeDriver: &driver.FakeDriver{
						VMExists:   true,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
					isCreation: false,
					testFunc: func(setUp setup, req machineActionRequest) (machineutils.RetryPeriod, error) {
						setUp.controller.pendingMachineCreationMap.Store("machine-xyz", "")
						return setUp.controller.triggerDeletionFlow(context.TODO(), &driver.DeleteMachineRequest{
							Machine:      req.machine,
							MachineClass: req.machineClass,
							Secret:       req.secret,
						})
					},
				},
				expect: expect{
					err:   fmt.Errorf("Machine deletion in process. Phase set to termination"),
					retry: machineutils.ShortRetry,
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
							v1alpha1.NodeLabelKey: "fakeID-0",
						},
						true,
						metav1.Now(),
					),
				},
			}),
		)
	})

	Describe("#computeEffectivePreserveAnnotationValue", func() {
		type setup struct {
			machinePreserveAnnotation string
			nodePreserveAnnotation    string
			nodeName                  string
		}
		type expect struct {
			preserveValue string
			exists        bool
			err           error
		}
		type testCase struct {
			setup  setup
			expect expect
		}

		DescribeTable("computeEffectivePreserveAnnotationValue behavior",
			func(tc testCase) {

				stop := make(chan struct{})
				defer close(stop)

				var controlMachineObjects []runtime.Object
				var targetCoreObjects []runtime.Object

				// Build machine
				machine := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "m1",
						Labels: map[string]string{
							v1alpha1.NodeLabelKey: tc.setup.nodeName,
						},
						Annotations: map[string]string{},
					},
				}
				if tc.setup.machinePreserveAnnotation != "" {
					machine.Annotations[machineutils.PreserveMachineAnnotationKey] = tc.setup.machinePreserveAnnotation
				}

				controlMachineObjects = append(controlMachineObjects, machine)
				// Build node
				if tc.setup.nodeName != "" && tc.setup.nodeName != "invalid" {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:        tc.setup.nodeName,
							Annotations: map[string]string{},
						},
					}
					if tc.setup.nodePreserveAnnotation != "" {
						node.Annotations[machineutils.PreserveMachineAnnotationKey] = tc.setup.nodePreserveAnnotation
					}
					targetCoreObjects = append(targetCoreObjects, node)
				}

				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()

				waitForCacheSync(stop, c)
				value, exists, err := c.computeEffectivePreserveAnnotationValue(machine)

				if tc.expect.err != nil {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(tc.expect.err.Error()))
					return
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(Equal(tc.expect.exists))
				Expect(value).To(Equal(tc.expect.preserveValue))
			},
			Entry("neither machine nor node has preserve annotation", testCase{
				setup: setup{
					nodeName: "node-1",
				},
				expect: expect{
					preserveValue: "",
					exists:        false,
					err:           nil,
				},
			}),
			Entry("only machine has preserve annotation", testCase{
				setup: setup{
					machinePreserveAnnotation: "machineValue",
					nodeName:                  "node-1",
				},
				expect: expect{
					preserveValue: "machineValue",
					exists:        true,
					err:           nil,
				},
			}),
			Entry("only node has preserve annotation", testCase{
				setup: setup{
					nodePreserveAnnotation: "nodeValue",
					nodeName:               "node-1",
				},
				expect: expect{
					preserveValue: "nodeValue",
					exists:        true,
					err:           nil,
				},
			}),
			Entry("both machine and node have preserve annotation - node takes precedence", testCase{
				setup: setup{
					machinePreserveAnnotation: "machineValue",
					nodePreserveAnnotation:    "nodeValue",
					nodeName:                  "node-1",
				},
				expect: expect{
					preserveValue: "nodeValue",
					exists:        true,
					err:           nil,
				},
			}),
			Entry("machine has node label but node object is not found", testCase{
				setup: setup{
					machinePreserveAnnotation: "machineValue",
					nodeName:                  "invalid",
				},
				expect: expect{
					preserveValue: "",
					exists:        false,
					err:           fmt.Errorf("node %q not found", "invalid"),
				},
			}),
			Entry("machine does not have node label", testCase{
				setup: setup{
					machinePreserveAnnotation: "machineValue",
				},
				expect: expect{
					preserveValue: "machineValue",
					exists:        true,
					err:           nil,
				},
			}),
		)
	})

	Describe("#manageMachinePreservation", func() {
		type setup struct {
			machineAnnotationValue string
			nodeAnnotationValue    string
			nodeName               string
			machinePhase           v1alpha1.MachinePhase
			preserveExpiryTime     *metav1.Time
		}
		type expect struct {
			retry                   machineutils.RetryPeriod
			preserveExpiryTimeIsSet bool
			err                     error
			nodeCondition           *corev1.NodeCondition
		}
		type testCase struct {
			setup  setup
			expect expect
		}

		DescribeTable("manageMachinePreservation behavior scenarios",
			func(tc testCase) {

				stop := make(chan struct{})
				defer close(stop)

				var controlMachineObjects []runtime.Object
				var targetCoreObjects []runtime.Object

				// Build machine
				machine := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "m1",
						Labels: map[string]string{
							v1alpha1.NodeLabelKey: tc.setup.nodeName,
						},
						Annotations: map[string]string{},
					}, Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							Phase:              tc.setup.machinePhase,
							LastUpdateTime:     metav1.Now(),
							PreserveExpiryTime: tc.setup.preserveExpiryTime,
						},
					},
				}
				if tc.setup.machineAnnotationValue != "" {
					machine.Annotations[machineutils.PreserveMachineAnnotationKey] = tc.setup.machineAnnotationValue
				}
				controlMachineObjects = append(controlMachineObjects, machine)
				if tc.setup.nodeName != "" && tc.setup.nodeName != "invalid" {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:        tc.setup.nodeName,
							Annotations: map[string]string{},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{},
						},
					}
					if tc.setup.nodeAnnotationValue != "" {
						node.Annotations[machineutils.PreserveMachineAnnotationKey] = tc.setup.nodeAnnotationValue
					}
					targetCoreObjects = append(targetCoreObjects, node)
				}
				c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects, nil, false)
				defer trackers.Stop()
				waitForCacheSync(stop, c)
				retry, err := c.manageMachinePreservation(context.TODO(), machine)

				Expect(retry).To(Equal(tc.expect.retry))
				if tc.expect.err != nil {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(tc.expect.err.Error()))
					return
				}
				Expect(err).ToNot(HaveOccurred())
				updatedMachine, err := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), machine.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if tc.expect.preserveExpiryTimeIsSet {
					Expect(updatedMachine.Status.CurrentStatus.PreserveExpiryTime.IsZero()).To(BeFalse())
				} else {
					Expect(updatedMachine.Status.CurrentStatus.PreserveExpiryTime.IsZero()).To(BeTrue())
				}
				if tc.setup.nodeName != "" {
					updatedNode, err := c.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), tc.setup.nodeName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					found := false
					if tc.expect.nodeCondition != nil {
						for _, cond := range updatedNode.Status.Conditions {
							if cond.Type == tc.expect.nodeCondition.Type {
								found = true
								Expect(cond.Status).To(Equal(tc.expect.nodeCondition.Status))
								break
							}
						}
					}
					if tc.expect.nodeCondition != nil {
						Expect(found).To(BeTrue())
					} else {
						Expect(found).To(BeFalse())
					}
				}
			},
			Entry("no preserve annotation on machine and node", testCase{
				setup: setup{
					nodeName: "node-1",
				},
				expect: expect{
					preserveExpiryTimeIsSet: false,
					nodeCondition:           nil,
					retry:                   machineutils.LongRetry,
				},
			}),
			Entry("preserve annotation 'now' added on Running machine", testCase{
				setup: setup{
					machineAnnotationValue: machineutils.PreserveMachineAnnotationValueNow,
					nodeName:               "node-1",
					machinePhase:           v1alpha1.MachineRunning,
				},
				expect: expect{
					preserveExpiryTimeIsSet: true,
					nodeCondition: &corev1.NodeCondition{
						Type:   v1alpha1.NodePreserved,
						Status: corev1.ConditionTrue},
					retry: machineutils.LongRetry,
				},
			}),
			Entry("preserve annotation 'when-failed' added on Running machine", testCase{
				setup: setup{
					machineAnnotationValue: machineutils.PreserveMachineAnnotationValueWhenFailed,
					nodeName:               "node-1",
					machinePhase:           v1alpha1.MachineRunning,
				},
				expect: expect{
					preserveExpiryTimeIsSet: false,
					nodeCondition:           nil,
					retry:                   machineutils.LongRetry,
				},
			}),
			Entry("Failed machine annotated with when-failed", testCase{
				setup: setup{
					machineAnnotationValue: machineutils.PreserveMachineAnnotationValueWhenFailed,
					nodeName:               "node-1",
					machinePhase:           v1alpha1.MachineFailed,
				},
				expect: expect{
					preserveExpiryTimeIsSet: true,
					nodeCondition: &corev1.NodeCondition{
						Type:   v1alpha1.NodePreserved,
						Status: corev1.ConditionTrue},
					retry: machineutils.LongRetry,
				},
			}),
			Entry("preserve annotation 'now' added on Healthy node ", testCase{
				setup: setup{
					nodeAnnotationValue: machineutils.PreserveMachineAnnotationValueNow,
					nodeName:            "node-1",
					machinePhase:        v1alpha1.MachineRunning,
				},
				expect: expect{
					preserveExpiryTimeIsSet: true,
					nodeCondition: &corev1.NodeCondition{
						Type:   v1alpha1.NodePreserved,
						Status: corev1.ConditionTrue},
					retry: machineutils.LongRetry,
				},
			}),
			Entry("preserve annotation 'when-failed' added on Healthy node ", testCase{
				setup: setup{
					nodeAnnotationValue: machineutils.PreserveMachineAnnotationValueWhenFailed,
					nodeName:            "node-1",
					machinePhase:        v1alpha1.MachineRunning,
				},
				expect: expect{
					preserveExpiryTimeIsSet: false,
					nodeCondition:           nil,
					retry:                   machineutils.LongRetry,
				}}),
			Entry("preserve annotation 'false' added on backing node of preserved machine", testCase{
				setup: setup{
					nodeAnnotationValue: "false",
					nodeName:            "node-1",
					machinePhase:        v1alpha1.MachineRunning,
					preserveExpiryTime:  &metav1.Time{Time: metav1.Now().Add(1 * time.Hour)},
				},
				expect: expect{
					preserveExpiryTimeIsSet: false,
					nodeCondition:           nil,
					retry:                   machineutils.LongRetry,
				},
			}),
			Entry("machine auto-preserved by MCM", testCase{
				setup: setup{
					machineAnnotationValue: machineutils.PreserveMachineAnnotationValuePreservedByMCM,
					nodeAnnotationValue:    "",
					nodeName:               "node-1",
					machinePhase:           v1alpha1.MachineRunning,
				},
				expect: expect{
					preserveExpiryTimeIsSet: true,
					nodeCondition: &corev1.NodeCondition{
						Type:   v1alpha1.NodePreserved,
						Status: corev1.ConditionTrue},
					retry: machineutils.LongRetry,
				},
			}),
			Entry("preservation timed out", testCase{
				setup: setup{
					machineAnnotationValue: machineutils.PreserveMachineAnnotationValueNow,
					nodeAnnotationValue:    machineutils.PreserveMachineAnnotationValueNow,
					nodeName:               "node-1",
					machinePhase:           v1alpha1.MachineRunning,
					preserveExpiryTime:     &metav1.Time{Time: metav1.Now().Add(-1 * time.Minute)},
				},
				expect: expect{
					preserveExpiryTimeIsSet: false,
					nodeCondition:           &corev1.NodeCondition{Type: v1alpha1.NodePreserved, Status: corev1.ConditionFalse},
					retry:                   machineutils.LongRetry,
				},
			}),
			Entry("invalid preserve annotation on node of unpreserved machine", testCase{
				setup: setup{
					machineAnnotationValue: "",
					nodeAnnotationValue:    "invalidValue",
					nodeName:               "node-1",
					machinePhase:           v1alpha1.MachineRunning,
				},
				expect: expect{
					preserveExpiryTimeIsSet: false,
					nodeCondition:           nil,
					retry:                   machineutils.LongRetry,
					err:                     nil,
				},
			}),
			Entry("machine annotated with preserve=now, but has no backing node", testCase{
				setup: setup{
					machineAnnotationValue: machineutils.PreserveMachineAnnotationValueNow,
					nodeAnnotationValue:    "",
					nodeName:               "",
					machinePhase:           v1alpha1.MachineUnknown,
				},
				expect: expect{
					preserveExpiryTimeIsSet: true,
					nodeCondition:           nil,
					retry:                   machineutils.LongRetry,
					err:                     nil,
				},
			}),
			Entry("machine with backing node, but node retrieval fails", testCase{
				setup: setup{
					machineAnnotationValue: machineutils.PreserveMachineAnnotationValueNow,
					nodeAnnotationValue:    "",
					nodeName:               "invalid",
					machinePhase:           v1alpha1.MachineUnknown,
				},
				expect: expect{
					preserveExpiryTimeIsSet: false,
					nodeCondition:           nil,
					retry:                   machineutils.ShortRetry,
					err:                     fmt.Errorf("node %q not found", "invalid"),
				},
			}),
		)
	})
})
