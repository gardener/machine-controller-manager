// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	faketyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

const (
	MachineRunning = "Running"
	MachineFailed  = "Failed"
)

var _ = Describe("machineset", func() {
	Describe("#getMachineMachineSets", func() {
		// Testcase: It should return error when Machine does not have any labels.
		It("should return error", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			c, trackers := createController(stop, testNamespace, objects, objects, objects)
			defer trackers.Stop()

			testMachine := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Machine-test",
					Namespace: testNamespace,
					Labels:    nil,
				},
			}
			MachineSet, err := c.getMachineMachineSets(testMachine)
			Expect(err).Should(Not(BeNil()))
			Expect(MachineSet).Should(BeNil())
		})

		// Testcase: Return correct MachineSet based on the LabelSelectors.
		It("should not return error", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Spec: machinev1.MachineSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test-label": "test-label",
						},
					},
				},
			}

			testMachine := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Machine-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet, err := c.getMachineMachineSets(testMachine)
			Expect(err).Should(BeNil())
			Expect(MachineSet).Should(Not(BeNil()))
			Expect(MachineSet).Should(HaveLen(1))
			Expect(MachineSet).Should(ContainElement(testMachineSet))
		})
	})

	Describe("#machineSetUpdate", func() {
		var (
			testMachineSet *machinev1.MachineSet
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
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
		})

		It("Should enqueue the machineset", func() {
			stop := make(chan struct{})
			machineSetObj := testMachineSet

			defer close(stop)

			objects := []runtime.Object{}
			c, trackers := createController(stop, testNamespace, objects, nil, nil)

			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machineSetUpdatedObj := machineSetObj.DeepCopy()
			machineSetUpdatedObj.Spec.Replicas = 5
			c.machineSetUpdate(machineSetObj, machineSetUpdatedObj)

			waitForCacheSync(stop, c)
			Expect(c.machineSetQueue.Len()).To(Equal(1))
		})
	})

	Describe("#addMachineToMachineSet", func() {
		var (
			testMachineSet *machinev1.MachineSet
			testMachine    *machinev1.Machine
		)
		BeforeEach(func() {
			testMachine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Machine-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "MachineSet",
							Name:       "MachineSet-test",
							UID:        "1234567",
							Controller: ptr.To(true),
						},
					},
				},
			}

			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
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
		})

		It("Should enqueue the machineset as controllerRef matches", func() {
			stop := make(chan struct{})

			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)

			defer trackers.Stop()
			waitForCacheSync(stop, c)

			c.addMachineToMachineSet(testMachine)

			waitForCacheSync(stop, c)
			Expect(c.machineSetQueue.Len()).To(Equal(1))
		})

		It("Should enqueue the machineset though controllerRef is not set but orphan is created", func() {
			stop := make(chan struct{})

			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)

			defer trackers.Stop()
			waitForCacheSync(stop, c)

			c.addMachineToMachineSet(testMachine)

			waitForCacheSync(stop, c)
			Expect(c.machineSetQueue.Len()).To(Equal(1))
		})

		It("Should enqueue the machineset while machine is being deleted", func() {
			stop := make(chan struct{})

			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)

			defer trackers.Stop()
			waitForCacheSync(stop, c)

			testMachine.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			c.addMachineToMachineSet(testMachine)

			waitForCacheSync(stop, c)
			Expect(c.machineSetQueue.Len()).To(Equal(1))
		})

		It("Shouldn't enqueue the machineset if machineset is not found via cotrollerRef", func() {
			stop := make(chan struct{})

			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)

			defer trackers.Stop()
			waitForCacheSync(stop, c)

			testMachine.OwnerReferences[0].Name = "dummy-one"
			c.addMachineToMachineSet(testMachine)

			waitForCacheSync(stop, c)
			Expect(c.machineSetQueue.Len()).To(Equal(0))
		})
	})

	Describe("#updateMachineToMachineSet", func() {
		var (
			testMachineSet *machinev1.MachineSet
			testMachine    *machinev1.Machine
		)
		BeforeEach(func() {
			testMachine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Machine-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					ResourceVersion: "123",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "MachineSet",
							Name:       "MachineSet-test",
							UID:        "1234567",
							Controller: ptr.To(true),
						},
					},
				},
			}

			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
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
		})
		Describe("Shouldn't enqueue the machineset", func() {
			It("Shouldn't enqueue the machineset if resource version matches", func() {
				stop := make(chan struct{})
				testMachineUpdated := testMachine.DeepCopy()

				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineSet)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)

				testMachineUpdated.ResourceVersion = testMachine.ResourceVersion
				c.updateMachineToMachineSet(testMachine, testMachineUpdated)

				waitForCacheSync(stop, c)
				Expect(c.machineSetQueue.Len()).To(Equal(0))
			})
		})

		DescribeTable("Should enqueue the machineset",
			func(preset func(oldMachine *machinev1.Machine, newMachine *machinev1.Machine)) {
				machine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Machine-test",
						Namespace: testNamespace,
						Labels: map[string]string{
							"test-label": "test-label",
						},
						ResourceVersion: "123",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "MachineSet",
								Name:       "MachineSet-test",
								UID:        "1234567",
								Controller: ptr.To(true),
							},
						},
					},
				}

				newMachine := machine.DeepCopy()
				newMachine.ResourceVersion = "345"

				stop := make(chan struct{})
				preset(machine, newMachine)
				defer close(stop)

				objects := []runtime.Object{}
				objects = append(objects, testMachineSet)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)

				defer trackers.Stop()
				waitForCacheSync(stop, c)
				c.updateMachineToMachineSet(machine, newMachine)

				waitForCacheSync(stop, c)
				Expect(c.machineSetQueue.Len()).To(Equal(1))
			},
			Entry("ResourceVersion is different for new machine",
				func(_ *machinev1.Machine, newMachine *machinev1.Machine) {
					newMachine.ResourceVersion = "3456"
				},
			),
			Entry("newMachine is being deleted",
				func(oldMachine *machinev1.Machine, _ *machinev1.Machine) {
					oldMachine.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				},
			),
			Entry("labels on newMachine has changed",
				func(_ *machinev1.Machine, newMachine *machinev1.Machine) {
					newMachine.Labels = map[string]string{
						"dummy": "dummy",
					}
				},
			),
			Entry("if controllerRef has changed and new ref is nil",
				func(_ *machinev1.Machine, newMachine *machinev1.Machine) {
					newMachine.OwnerReferences = nil
				},
			),
			Entry("if controllerRef has changed and new ref points to valid machineSet",
				func(_ *machinev1.Machine, newMachine *machinev1.Machine) {
					newMachine.OwnerReferences = []metav1.OwnerReference{
						{
							Kind:       "MachineSet",
							Name:       "MachineSet-test-dummy",
							UID:        "1234567",
							Controller: ptr.To(true),
						},
					}
				},
			),
		)

	})

	Describe("#resolveMachineSetControllerRef", func() {

		// Testcase: It should not return MachineSet if only name matches but UID does not.
		It("should not return MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234", // Dummy UID
				},
			}

			testControllerRef := &metav1.OwnerReference{
				Name: "MachineSet-test",
				Kind: "MachineSet",
				UID:  "1234567", // Actual UID
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet := c.resolveMachineSetControllerRef(testNamespace, testControllerRef)
			Expect(MachineSet).Should(BeNil())
		})

		// Testcase: It should return MachineSet if name and UID matches.
		It("should return MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
			}

			testControllerRef := &metav1.OwnerReference{
				Name: "MachineSet-test",
				Kind: "MachineSet",
				UID:  "1234567",
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet := c.resolveMachineSetControllerRef(testNamespace, testControllerRef)
			Expect(MachineSet).Should(Not(BeNil()))
			Expect(MachineSet).Should(Equal(testMachineSet))
		})

		// Testcase: It should return MachineSet if name and UID matches.
		It("should not return MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
			}

			testControllerRef := &metav1.OwnerReference{
				Name: "MachineSet-test",
				Kind: "'MachineDeployment'", // Incorrect Kind
				UID:  "1234567",
			}

			objects = append(objects, testMachineSet)

			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			MachineSet := c.resolveMachineSetControllerRef(testNamespace, testControllerRef)
			Expect(MachineSet).Should(BeNil())
		})
	})

	Describe("#manageReplicas", func() {

		var (
			testMachineSet     *machinev1.MachineSet
			testActiveMachine1 *machinev1.Machine
			testActiveMachine2 *machinev1.Machine
			testActiveMachine3 *machinev1.Machine
			testActiveMachine4 *machinev1.Machine
			testActiveMachine5 *machinev1.Machine
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 3,
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

			testActiveMachine1 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
					UID:       "1234568",
					Labels: map[string]string{
						"test-label": "test-label",
					},
					Annotations: map[string]string{},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}

			testActiveMachine2 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-2",
					Namespace: testNamespace,
					UID:       "1234569",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}

			testActiveMachine3 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-3",
					Namespace: testNamespace,
					UID:       "12345610",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}

			testActiveMachine4 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-4",
					Namespace: testNamespace,
					UID:       "12345611",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineFailed,
					},
				},
			}
		})

		// Testcase: ActiveMachines < DesiredMachines
		// It should create new machines and should not return erros.
		It("should create new machines and should not return errors.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas) - 1))

			activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2}
			Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
			waitForCacheSync(stop, c)
			// TODO: Could not use Listers here, need to check more.
			machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
			Expect(Err).Should(BeNil())
		})

		// It should return nil error if typemeta is missing in machine-set, to avoid constant reconciliations.
		It("should return nil on buggy machineset and avoid reconciliations", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas) - 1))

			activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2}

			testMachineSet.TypeMeta = metav1.TypeMeta{}
			Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
			waitForCacheSync(stop, c)

			Expect(Err).Should(BeNil())
		})

		// Testcase: diff > burstReplicas
		// Create number of machines equal to the burst-replicas.
		It("should create new machines only equal to burstReplicas and should not return errors.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			testMachineSet.Spec.Replicas = 200
			objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2}
			Expect(c.manageReplicas(context.TODO(), activeMachines, testMachineSet)).NotTo(HaveOccurred())
			waitForCacheSync(stop, c)
			// TODO: Could not use Listers here, need to check more.
			machines, err := c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(machines.Items)).To(Equal(int(BurstReplicas + len(activeMachines))))
		})

		// TestCase: ActiveMachines = DesiredMachines
		// Testcase: It should not return error.
		It("should not create or delete machines and should not return error", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2, testActiveMachine3)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))

			activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2, testActiveMachine3}
			Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
			waitForCacheSync(stop, c)
			machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
			Expect(Err).Should(BeNil())
		})

		// TestCase: ActiveMachines > DesiredMachines
		// Testcase: It should not return error and delete extra failed machine.
		It("should not return error and should delete extra failed machine.", func() {
			stop := make(chan struct{})
			defer close(stop)

			testActiveMachine3 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-3",
					Namespace: testNamespace,
					UID:       "12345610",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2, testActiveMachine3, testActiveMachine4)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas + 1)))

			activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2, testActiveMachine3, testActiveMachine4}
			Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
			waitForCacheSync(stop, c)
			machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
			Expect(Err).Should(BeNil())
		})

		// TestCase: ActiveMachines > DesiredMachines
		// Testcase: It should not return error and delete extra running machine.
		It("should not return error and should delete extra running machine.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2, testActiveMachine3, testActiveMachine4)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas + 1)))

			activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2, testActiveMachine3, testActiveMachine4}
			Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
			waitForCacheSync(stop, c)
			machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
			Expect(Err).Should(BeNil())
		})

		It("should delete MachinePriority=1 machines", func() {
			stop := make(chan struct{})
			defer close(stop)
			objects := []runtime.Object{}

			staleMachine := testActiveMachine1.DeepCopy()
			staleMachine.Annotations[machineutils.MachinePriority] = "1"
			testActiveMachine4.Status.CurrentStatus.Phase = MachineRunning

			objects = append(objects, testMachineSet, staleMachine, testActiveMachine2, testActiveMachine3, testActiveMachine4)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas + 1)))

			beforeMachines := []*machinev1.Machine{staleMachine, testActiveMachine2, testActiveMachine3, testActiveMachine4}
			err := c.manageReplicas(context.TODO(), beforeMachines, testMachineSet)
			Expect(err).Should(BeNil())
			waitForCacheSync(stop, c)

			_, err = c.controlMachineClient.Machines(testNamespace).Get(context.Background(), staleMachine.Name, metav1.GetOptions{})
			Expect(err).ShouldNot(BeNil())
			Expect(err).To(Satisfy(func(e error) bool {
				return k8sError.IsNotFound(e)
			}))
			afterMachines, err := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			// replica count is still maintained.
			Expect(len(afterMachines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
			Expect(err).Should(BeNil())
		})

		Describe("machine with update-result label", func() {
			// Testcase: ActiveMachines + MachinesWithUpdateSuccessfulLabel < DesiredMachines
			It("should create new machines and should not return errors.", func() {
				stop := make(chan struct{})
				defer close(stop)

				testActiveMachine3 = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-3",
						Namespace: testNamespace,
						UID:       "12345610",
						Labels: map[string]string{
							"test-label":                             "test-label",
							"node.machine.sapcloud.io/update-result": "successful",
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "Machine",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase: MachineRunning,
						},
					},
				}

				objects := []runtime.Object{}
				objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine3)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas) - 1))

				activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine3}
				Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
				waitForCacheSync(stop, c)
				machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
				Expect(Err).Should(BeNil())
			})

			// Testcase: ActiveMachines + MachinesWithUpdateSuccessfulLabel = DesiredMachines
			It("should not create or delete machines and should not return error", func() {
				stop := make(chan struct{})
				defer close(stop)

				testActiveMachine3 = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-3",
						Namespace: testNamespace,
						UID:       "12345610",
						Labels: map[string]string{
							"test-label":                             "test-label",
							"node.machine.sapcloud.io/update-result": "successful",
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "Machine",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase: MachineRunning,
						},
					},
				}

				objects := []runtime.Object{}
				objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2, testActiveMachine3)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))

				activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2, testActiveMachine3}
				Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
				waitForCacheSync(stop, c)
				machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
				Expect(Err).Should(BeNil())
			})

			// TestCase: ActiveMachines + MachinesWithUpdateSuccessfulLabel > DesiredMachines
			It("should not return error and delete extra running machine", func() {
				stop := make(chan struct{})
				defer close(stop)

				testActiveMachine3 = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-3",
						Namespace: testNamespace,
						UID:       "12345610",
						Labels: map[string]string{
							"test-label":                             "test-label",
							"node.machine.sapcloud.io/update-result": "successful",
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "Machine",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase: MachineRunning,
						},
					},
				}

				testActiveMachine4 = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-4",
						Namespace: testNamespace,
						UID:       "12345611",
						Labels: map[string]string{
							"test-label": "test-label",
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "Machine",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase: MachineRunning,
						},
					},
				}

				testActiveMachine5 = &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-5",
						Namespace: testNamespace,
						UID:       "12345612",
						Labels: map[string]string{
							"test-label": "test-label",
						},
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "Machine",
						APIVersion: "machine.sapcloud.io/v1alpha1",
					},
					Status: machinev1.MachineStatus{
						CurrentStatus: machinev1.CurrentStatus{
							Phase: MachineRunning,
						},
					},
				}

				objects := []runtime.Object{}
				objects = append(objects, testMachineSet, testActiveMachine1, testActiveMachine2, testActiveMachine3, testActiveMachine4, testActiveMachine5)
				c, trackers := createController(stop, testNamespace, objects, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas) + 2))

				activeMachines := []*machinev1.Machine{testActiveMachine1, testActiveMachine2, testActiveMachine3, testActiveMachine4, testActiveMachine5}
				Err := c.manageReplicas(context.TODO(), activeMachines, testMachineSet)
				waitForCacheSync(stop, c)
				machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
				Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas) + 1))
				Expect(Err).Should(BeNil())
			})
		})
	})

	Describe("#isFailedMachineCandidateForPreservation", func() {

		type setup struct {
			autoPreserveFailedMachineMax   int32
			machineIsPreserved             bool
			machinePreserveAnnotationValue string
			backingNode                    *corev1.Node
		}
		type expect struct {
			result bool
			err    error
		}
		type testCase struct {
			setup  setup
			expect expect
		}

		DescribeTable("isFailedMachineCandidateForPreservation test cases", func(tc testCase) {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}

			testMachineSet := &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 2,
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
					AutoPreserveFailedMachineMax: tc.setup.autoPreserveFailedMachineMax,
				},
				Status: machinev1.MachineSetStatus{
					AutoPreserveFailedMachineCount: int32(0),
				},
			}

			testMachine := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
					UID:       "1234568",
					Labels: map[string]string{
						"test-label": "test-label",
					},
					Annotations: map[string]string{
						machineutils.PreserveMachineAnnotationKey: tc.setup.machinePreserveAnnotationValue,
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Machine",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineFailed,
					},
				},
			}
			if tc.setup.backingNode != nil {
				testMachine.Labels[machinev1.NodeLabelKey] = "node-1"
			}
			if tc.setup.machineIsPreserved {
				testMachine.Status.CurrentStatus.PreserveExpiryTime = &metav1.Time{
					Time: time.Now().Add(1 * time.Hour),
				}
			}

			objects = append(objects, testMachineSet, testMachine)
			var targetCoreObjects []runtime.Object
			if tc.setup.backingNode != nil {
				targetCoreObjects = append(targetCoreObjects, tc.setup.backingNode)
			}
			c, trackers := createController(stop, testNamespace, objects, nil, targetCoreObjects)
			defer trackers.Stop()
			waitForCacheSync(stop, c)
			result, err := c.isFailedMachineCandidateForPreservation(context.TODO(), testMachineSet, testMachine)
			Expect(result).To(Equal(tc.expect.result))
			if tc.expect.err != nil {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(tc.expect.err.Error()))
				return
			}
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("should return false for un-preserved machine, when autoPreserveFailedMachineMax is 0", testCase{
				setup: setup{
					autoPreserveFailedMachineMax: 0,
					machineIsPreserved:           false,
					backingNode:                  nil,
				},
				expect: expect{
					result: false,
					err:    nil,
				},
			}),
			Entry("should return true for un-preserved machine, when autoPreserveFailedMachineMax is 1", testCase{
				setup: setup{
					autoPreserveFailedMachineMax: 1,
					machineIsPreserved:           false,
					backingNode:                  nil,
				},
				expect: expect{
					result: true,
					err:    nil,
				},
			}),
			Entry("should return true for machine annotated with preserve=now", testCase{
				setup: setup{
					autoPreserveFailedMachineMax:   1,
					machineIsPreserved:             true,
					machinePreserveAnnotationValue: machineutils.PreserveMachineAnnotationValueNow,
					backingNode:                    nil,
				},
				expect: expect{
					result: true,
					err:    nil,
				},
			}),
			Entry("should return true for machine annotated with preserve=when-failed, but not yet preserved", testCase{
				setup: setup{
					autoPreserveFailedMachineMax:   1,
					machineIsPreserved:             false,
					machinePreserveAnnotationValue: machineutils.PreserveMachineAnnotationValueWhenFailed,
					backingNode:                    nil,
				},
				expect: expect{
					result: true,
					err:    nil,
				},
			}),
			Entry("should return false for machine annotated with preserve=false", testCase{
				setup: setup{
					autoPreserveFailedMachineMax:   1,
					machineIsPreserved:             false,
					machinePreserveAnnotationValue: machineutils.PreserveMachineAnnotationValueFalse,
					backingNode:                    nil,
				},
				expect: expect{
					result: false,
					err:    nil,
				},
			}),
			Entry("should return false when backing node is annotated with preserve=false", testCase{
				setup: setup{
					autoPreserveFailedMachineMax: 1,
					machineIsPreserved:           false,
					backingNode: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
							Annotations: map[string]string{
								machineutils.PreserveMachineAnnotationKey: machineutils.PreserveMachineAnnotationValueFalse,
							},
						},
					},
				},
				expect: expect{
					result: false,
					err:    nil,
				},
			}),
			Entry("should return true when backing node has no preserve-annotation, and autoPreserveFailedMachineMax is 1", testCase{
				setup: setup{
					autoPreserveFailedMachineMax: 1,
					machineIsPreserved:           false,
					backingNode: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "node-1",
							Annotations: map[string]string{},
						},
					},
				},
				expect: expect{
					result: true,
					err:    nil,
				},
			}),
			Entry("should return error when backing node is not found", testCase{
				setup: setup{
					autoPreserveFailedMachineMax: 1,
					machineIsPreserved:           false,
					backingNode: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "invalid",
							Annotations: map[string]string{},
						},
					},
				},
				expect: expect{
					result: false,
					err:    errors.New("node \"node-1\" not found"),
				},
			}),
		)
	})
	// TODO: This method has dependency on generic-machineclass. Implement later.
	Describe("#reconcileClusterMachineSet", func() {
		var (
			testMachineSet *machinev1.MachineSet
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MachineSet",
					APIVersion: "machine.sapcloud.io/v1alpha1",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 3,
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label": "test-label",
							},
						},
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Name: "MachineClass-test",
								Kind: "MachineClass",
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
		})

		// Testcase: It should create new machines.
		It("It should create new machines.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(0)))

			Key := testNamespace + "/" + testMachineSet.Name
			Err := c.reconcileClusterMachineSet(Key)

			waitForCacheSync(stop, c)
			machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.TODO(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
			Expect(Err).Should(BeNil())
		})

		// Testcase: Should return nil if the machineset doesnt exist, to avoid constant reconciliations.
		It("It should return nil if machineset doesnt exist.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(0)))

			Key := testNamespace + "/" + testMachineSet.Name
			Err := c.reconcileClusterMachineSet(Key)

			waitForCacheSync(stop, c)
			Expect(Err).Should(BeNil())
		})

		// Testcase: It should return nil if the machineset validation fails.
		It("It should return nil if machineset validation fails", func() {
			stop := make(chan struct{})
			defer close(stop)

			testMachineSet.Spec.Template.Spec.Class = machinev1.ClassSpec{}
			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(0)))

			Key := testNamespace + "/" + testMachineSet.Name
			Err := c.reconcileClusterMachineSet(Key)

			waitForCacheSync(stop, c)
			Expect(Err).Should(BeNil())
		})

		// Testcase: It should delete all the machines as DeletionTimestamp is set.
		It("It should delete all the machines as DeletionTimestamp is set on MachineSet", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			testMachineSet.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			testMachineSet.Finalizers = []string{DeleteFinalizerName}
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(0)))

			Key := testNamespace + "/" + testMachineSet.Name
			Err := c.reconcileClusterMachineSet(Key)

			waitForCacheSync(stop, c)
			machines, _ = c.controlMachineClient.Machines(testNamespace).List(context.Background(), metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(0)))
			Expect(Err).Should(BeNil())
		})
	})

	Describe("#claimMachines", func() {
		var (
			testMachineSet     *machinev1.MachineSet
			testActiveMachine1 *machinev1.Machine
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 1,
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

			testActiveMachine1 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
					UID:       "1234568",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}
		})

		// Testcase: It should adopt new machines.
		/* TBD: Looks like an patch issue with fake clients in 1.16 need to fix it.
		FIt("should adopt new machines.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			filteredMachines := []*machinev1.Machine{testActiveMachine1}
			Selector, _ := metav1.LabelSelectorAsSelector(testMachineSet.Spec.Selector)
			filteredMachines, Err := c.claimMachines(testMachineSet, Selector, filteredMachines)

			waitForCacheSync(stop, c)
			Expect(len(filteredMachines)).To(Equal(int(testMachineSet.Spec.Replicas)))
			Expect(Err).Should(BeNil())
		})
		*/

		// Testcase: It should release the machine due to not matching machine-labels.
		It("should release the machine due to not matching machine-labels.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testActiveMachine1)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			filteredMachines := []*machinev1.Machine{testActiveMachine1}
			Selector, _ := metav1.LabelSelectorAsSelector(testMachineSet.Spec.Selector)
			filteredMachines, Err := c.claimMachines(context.TODO(), testMachineSet, Selector, filteredMachines)

			waitForCacheSync(stop, c)
			Expect(filteredMachines[0].Name).To(Equal(testActiveMachine1.Name))
			Expect(Err).Should(BeNil())

			testActiveMachine1.Labels = map[string]string{
				"dummy-label": "dummy-label",
			}

			filteredMachines, Err = c.claimMachines(context.TODO(), testMachineSet, Selector, filteredMachines)

			waitForCacheSync(stop, c)
			Expect(len(filteredMachines)).To(Equal(0))
			Expect(Err).Should(BeNil())
		})
	})

	Describe("#slowStartBatch", func() {
		var (
			count            int
			initialBatchSize int
			f                func() error
			fError           func() error
		)

		BeforeEach(func() {
			f = func() error {
				// Do nothing
				return nil
			}
			fError = func() error {
				// Throw Error
				err := errors.New("some error")
				return err
			}
		})

		// It should return number of success call which should be equal to count.
		It("should return number of success call which should be equal to count.", func() {
			count = 10
			initialBatchSize = 2
			successes, Err := slowStartBatch(count, initialBatchSize, f)
			Expect(successes).To(Equal(count))
			Expect(Err).Should(BeNil())
		})

		// It should fail initialBatchSize is 0
		It("should fail initialBatchSize is 0", func() {
			count = 10
			initialBatchSize = 0
			successes, Err := slowStartBatch(count, initialBatchSize, f)
			Expect(successes).Should(Equal(0))
			Expect(Err).Should(BeNil())
		})

		// It should return errors as fError throws one.
		It("should return error", func() {
			count = 10
			initialBatchSize = 2
			successes, Err := slowStartBatch(count, initialBatchSize, fError)
			Expect(successes).Should(Equal(0))
			Expect(Err).Should(Not(BeNil()))
		})
	})

	Describe("#getMachinesToDelete", func() {
		var (
			testActiveMachine1 *machinev1.Machine
			testFailedMachine1 *machinev1.Machine
			diff               int
		)

		BeforeEach(func() {

			testActiveMachine1 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
					UID:       "1234568",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}

			testFailedMachine1 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-2",
					Namespace: testNamespace,
					UID:       "1234569",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineFailed,
					},
				},
			}
		})

		// Testcase: It should return the Failed machines first.
		It("should return the Failed machines first.", func() {
			stop := make(chan struct{})
			defer close(stop)
			diff = 1
			filteredMachines := []*machinev1.Machine{testActiveMachine1, testFailedMachine1}
			machinesToDelete := getMachinesToDelete(filteredMachines, diff)

			Expect(len(machinesToDelete)).To(Equal(len(filteredMachines) - diff))
			Expect(machinesToDelete[0].Name).To(Equal(testFailedMachine1.Name))
		})
	})

	Describe("#getMachineKeys", func() {
		var (
			testMachine1 *machinev1.Machine
			testMachine2 *machinev1.Machine
		)

		BeforeEach(func() {

			testMachine1 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
				},
			}

			testMachine2 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-2",
					Namespace: testNamespace,
				},
			}
		})

		// It should return number of success call which should be equal to count.
		It("should return number of success call which should be equal to count.", func() {
			filteredMachines := []*machinev1.Machine{testMachine1, testMachine2}
			Keys := getMachineKeys(filteredMachines)
			Expect(Keys).To(HaveLen(len(filteredMachines)))
			for k := range Keys {
				Expect(Keys[k]).To(Equal(filteredMachines[k].Name))
			}
		})
	})

	Describe("#prepareMachineForDeletion", func() {
		var (
			testMachineSet *machinev1.MachineSet
			targetMachine  *machinev1.Machine
			wg             sync.WaitGroup
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 1,
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

			targetMachine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
					UID:       "1234568",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}
		})

		// TestCase: It should delete the target machine.
		It("should delete the target machine.", func() {
			var errCh chan error
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, targetMachine)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)
			wg.Add(1)

			c.prepareMachineForDeletion(context.TODO(), targetMachine, testMachineSet, &wg, errCh)
			waitForCacheSync(stop, c)
			_, err := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), targetMachine.Name, metav1.GetOptions{})

			Expect(k8sError.IsNotFound(err)).Should(BeTrue())
		})
	})

	Describe("#terminateMachines", func() {
		var (
			testMachineSet     *machinev1.MachineSet
			testFailedMachine2 *machinev1.Machine
			testFailedMachine1 *machinev1.Machine
			testRunningMachine *machinev1.Machine
		)

		BeforeEach(func() {

			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 2,
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

			testFailedMachine1 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testNamespace,
					UID:       "1234568",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineFailed,
					},
				},
			}

			testFailedMachine2 = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-2",
					Namespace: testNamespace,
					UID:       "1234569",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineFailed,
					},
				},
			}

			testRunningMachine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-t",
					Namespace: testNamespace,
					UID:       "1234560",
					Labels: map[string]string{
						"test-label": "test-label",
					},
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: MachineRunning,
					},
				},
			}
		})

		// Testcase: It should delete the inactive machines.
		It("It should delete the inactive machines.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testFailedMachine1, testFailedMachine2)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			inactiveMachines := []*machinev1.Machine{testFailedMachine1, testFailedMachine2}
			err := c.terminateMachines(context.TODO(), inactiveMachines, testMachineSet)

			waitForCacheSync(stop, c)
			_, Err1 := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), inactiveMachines[0].Name, metav1.GetOptions{})
			_, Err2 := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), inactiveMachines[1].Name, metav1.GetOptions{})

			Expect(err).Should(BeNil())
			Expect(Err1).Should(Not(BeNil()))
			Expect(Err2).Should(Not(BeNil()))
		})

		It("It should not mark a machine as terminating when deletion fails.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet, testFailedMachine1, testRunningMachine)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			// Ref: https://estebangarcia.io/unit-testing-k8s-golang/
			// Add a reactor that intercepts machine delete call and returns an error
			// to simulate error when processing deletion request for a machine
			machineDeletionError := "forced machine deletion error"
			c.controlMachineClient.(*faketyped.FakeMachineV1alpha1).Fake.PrependReactor("delete", "machines", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &machinev1.Machine{}, errors.New(machineDeletionError)
			})
			targetMachines := []*machinev1.Machine{testFailedMachine1, testRunningMachine}
			err := c.terminateMachines(context.TODO(), targetMachines, testMachineSet)

			waitForCacheSync(stop, c)

			mFailed, Err1 := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), targetMachines[0].Name, metav1.GetOptions{})
			mRunning, Err2 := c.controlMachineClient.Machines(testNamespace).Get(context.TODO(), targetMachines[1].Name, metav1.GetOptions{})

			Expect(err).To(Equal(fmt.Errorf("unable to delete machines: %s", machineDeletionError)))
			Expect(Err1).Should(BeNil())
			Expect(mFailed.ObjectMeta.DeletionTimestamp).Should(BeNil())
			Expect(mFailed.Status.CurrentStatus.Phase).NotTo(Equal(machinev1.MachineTerminating))
			Expect(Err2).Should(BeNil())
			Expect(mRunning.ObjectMeta.DeletionTimestamp).Should(BeNil())
			Expect(mRunning.Status.CurrentStatus.Phase).NotTo(Equal(machinev1.MachineTerminating))
		})
	})

	Describe("#addMachineSetFinalizers", func() {
		var (
			testMachineSet *machinev1.MachineSet
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 2,
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

		})

		// Testcase: It should add finalizer on MachineSet.
		It("should add finalizer on MachineSet.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			err := c.addMachineSetFinalizers(context.TODO(), testMachineSet)

			waitForCacheSync(stop, c)
			testMachineSet, _ := c.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), testMachineSet.Name, metav1.GetOptions{})

			Expect(err).ToNot(HaveOccurred())
			Expect(testMachineSet.Finalizers).To(HaveLen(1))
			Expect(testMachineSet.Finalizers).To(ContainElement(DeleteFinalizerName))
		})
	})

	Describe("#deleteMachineSetFinalizers", func() {
		var (
			testMachineSet *machinev1.MachineSet
			finalizers     []string
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 2,
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
			finalizers = []string{"finalizer1"}

		})

		// Testcase: It should delete the finalizer from MachineSet.
		It("should delete the finalizer from MachineSet.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			testMachineSet, _ := c.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), testMachineSet.Name, metav1.GetOptions{})
			testMachineSet.Finalizers = finalizers
			Expect(testMachineSet.Finalizers).Should(Not(BeEmpty()))

			err := c.deleteMachineSetFinalizers(context.TODO(), testMachineSet)

			waitForCacheSync(stop, c)
			testMachineSet, _ = c.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), testMachineSet.Name, metav1.GetOptions{})

			Expect(err).ToNot(HaveOccurred())
			Expect(testMachineSet.Finalizers).Should(BeNil())
		})
	})

	Describe("#updateMachineSetFinalizers", func() {
		var (
			testMachineSet *machinev1.MachineSet
			finalizers     []string
		)

		BeforeEach(func() {
			testMachineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "MachineSet-test",
					Namespace: testNamespace,
					Labels: map[string]string{
						"test-label": "test-label",
					},
					UID: "1234567",
				},
				Spec: machinev1.MachineSetSpec{
					Replicas: 2,
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

			finalizers = []string{"finalizer1", "finalizer2"}

		})

		// Testcase: It should update the finalizer on MachineSet.
		It("should update the finalizer on MachineSet.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			err := c.updateMachineSetFinalizers(context.TODO(), testMachineSet, finalizers)

			waitForCacheSync(stop, c)
			testMachineSet, _ := c.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), testMachineSet.Name, metav1.GetOptions{})

			Expect(err).ToNot(HaveOccurred())
			Expect(testMachineSet.Finalizers).To(Equal(finalizers))
		})
	})
})
