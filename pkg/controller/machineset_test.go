/*
Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.

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
	"math/rand"
	"strconv"
	"sync"

	. "github.com/onsi/ginkgo/extensions/table"

	"k8s.io/apimachinery/pkg/types"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

		entryTemplate := `
MachineSet Replicas: %v
Running Machines: %v
Pending Machine: %v
Failed Machine: %v
Expected Active Machines: %v
Expected Failed Machines: %v
FailedMachineDeletionRatio: %v

`

		FDescribeTable("", func(machineSetReplicas, runningMachinesCount, pendingMachinesCount, failedMachinesCount,
			expectedActiveMachinesCount, expectedFailedMachinesCount int, failedMachineDeletionRatio float64) {

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
					Replicas: int32(machineSetReplicas),
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"test-label":     "test-label",
								"failed-machine": "false",
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

			stop := make(chan struct{})
			defer close(stop)

			var objects []runtime.Object
			initialActiveMachines := generateMachines(runningMachinesCount, pendingMachinesCount, 0, map[string]string{
				"test-label":     "test-label",
				"failed-machine": "false",
			})
			initialFailedMachines := generateMachines(0, 0, failedMachinesCount, map[string]string{
				"test-label":     "test-label",
				"failed-machine": "true",
			})
			initialMachines := append(initialActiveMachines, initialFailedMachines...)
			objects = append(objects, testMachineSet)
			for _, m := range initialMachines {
				objects = append(objects, m)
			}
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			c.safetyOptions.FailedMachineDeletionRatio = failedMachineDeletionRatio
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(len(initialMachines)))

			err := c.manageReplicas(initialMachines, testMachineSet)
			Expect(err).To(BeNil())

			waitForCacheSync(stop, c)

			//TODO: Could not use Listers here, need to check more.
			activeMachine, err := c.controlMachineClient.Machines(testNamespace).List(metav1.ListOptions{LabelSelector: "failed-machine=false"})
			Expect(err).To(BeNil())
			Expect(len(activeMachine.Items)).To(Equal(expectedActiveMachinesCount))

			failedMachines, err := c.controlMachineClient.Machines(testNamespace).List(metav1.ListOptions{LabelSelector: "failed-machine=true"})
			Expect(err).To(BeNil())
			Expect(len(failedMachines.Items)).To(Equal(expectedFailedMachinesCount))
		},
			generateEntry("should create a machine"+entryTemplate, 3, 2, 0, 0, 3, 0, 1.0),
			generateEntry("should not create or delete machines"+entryTemplate, 3, 3, 0, 0, 3, 0, 1.0),
			generateEntry("should delete a machine"+entryTemplate, 3, 4, 0, 0, 3, 0, 1.0),

			// let's test FailedMachineDeletionRatio behavior
			generateEntry("should not delete failed machines and not create any new machines"+entryTemplate, 3, 2, 0, 5, 2, 5, 0.0),
			generateEntry("should delete one machine and create one machine"+entryTemplate, 3, 2, 0, 5, 3, 4, 0.2),

			// playing with different FailedMachineDeletionRatios
			generateEntry("should delete one machine"+entryTemplate, 10, 10, 0, 10, 10, 9, 0.1),
			generateEntry("should delete two machines"+entryTemplate, 10, 10, 0, 5, 10, 3, 0.2),
			generateEntry("should delete 4 machines"+entryTemplate, 10, 10, 0, 5, 10, 1, 0.4),
			generateEntry("should delete 3 failed machines (and create new ones)"+entryTemplate, 10, 5, 0, 5, 8, 2, 0.3),
			generateEntry("should delete all failed machines (and create new ones)"+entryTemplate, 10, 2, 0, 5, 10, 0, 1.0),
			generateEntry("should delete two failed machines (and create new ones)"+entryTemplate, 5, 15, 0, 5, 5, 2, 0.5),
		)
	})

	//TODO: This method has dependency on generic-machineclass. Implement later.
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
							// Class: machinev1.ClassSpec{
							// 	Name: "MachineClass-test",
							// 	Kind: "MachineClass",
							// },
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

		//Testcase: It should create new machines.
		It("It should create new machines.", func() {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			objects = append(objects, testMachineSet)
			c, trackers := createController(stop, testNamespace, objects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machines, _ := c.controlMachineClient.Machines(testNamespace).List(metav1.ListOptions{})
			Expect(len(machines.Items)).To(Equal(int(0)))

			Key := testNamespace + "/" + testMachineSet.Name
			Err := c.reconcileClusterMachineSet(Key)

			waitForCacheSync(stop, c)
			machines, _ = c.controlMachineClient.Machines(testNamespace).List(metav1.ListOptions{})
			//Expect(len(machines.Items)).To(Equal(int(testMachineSet.Spec.Replicas)))
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
						Phase: machinev1.MachineRunning,
					},
				},
			}
		})

		//Testcase: It should adopt new machines.
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

		//Testcase: It should release the machine due to not matching machine-labels.
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
			filteredMachines, Err := c.claimMachines(testMachineSet, Selector, filteredMachines)

			waitForCacheSync(stop, c)
			Expect(filteredMachines[0].Name).To(Equal(testActiveMachine1.Name))
			Expect(Err).Should(BeNil())

			testActiveMachine1.Labels = map[string]string{
				"dummy-label": "dummy-label",
			}

			filteredMachines, Err = c.claimMachines(testMachineSet, Selector, filteredMachines)

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
				//Throw Error
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
						Phase: machinev1.MachineRunning,
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
						Phase: machinev1.MachineFailed,
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
						Phase: machinev1.MachineRunning,
					},
				},
			}
		})

		//TestCase: It should delete the target machine.
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

			c.prepareMachineForDeletion(targetMachine, testMachineSet, &wg, errCh)
			waitForCacheSync(stop, c)
			_, err := c.controlMachineClient.Machines(testNamespace).Get(targetMachine.Name, metav1.GetOptions{})

			Expect(k8sError.IsNotFound(err)).Should(BeTrue())
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

			c.addMachineSetFinalizers(testMachineSet)

			waitForCacheSync(stop, c)
			testMachineSet, _ := c.controlMachineClient.MachineSets(testNamespace).Get(testMachineSet.Name, metav1.GetOptions{})

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

			testMachineSet, _ := c.controlMachineClient.MachineSets(testNamespace).Get(testMachineSet.Name, metav1.GetOptions{})
			testMachineSet.Finalizers = finalizers
			Expect(testMachineSet.Finalizers).Should(Not(BeEmpty()))

			c.deleteMachineSetFinalizers(testMachineSet)

			waitForCacheSync(stop, c)
			testMachineSet, _ = c.controlMachineClient.MachineSets(testNamespace).Get(testMachineSet.Name, metav1.GetOptions{})

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

			c.updateMachineSetFinalizers(testMachineSet, finalizers)

			waitForCacheSync(stop, c)
			testMachineSet, _ := c.controlMachineClient.MachineSets(testNamespace).Get(testMachineSet.Name, metav1.GetOptions{})

			Expect(testMachineSet.Finalizers).To(Equal(finalizers))
		})
	})
})

func generateMachines(running, pending, failed int, labels map[string]string) (ret []*machinev1.Machine) {
	for i := 0; i < running; i++ {
		ret = append(ret, &machinev1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "running-machine-" + strconv.Itoa(i),
				Namespace: testNamespace,
				UID:       types.UID(strconv.Itoa(rand.Int())),
				Labels:    labels,
			},
			Status: machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachineRunning,
				},
			},
		})
	}

	for i := 0; i < pending; i++ {
		ret = append(ret, &machinev1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pending-machine-" + strconv.Itoa(i),
				Namespace: testNamespace,
				UID:       types.UID(strconv.Itoa(rand.Int())),
				Labels:    labels,
			},
			Status: machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachinePending,
				},
			},
		})
	}

	for i := 0; i < failed; i++ {
		ret = append(ret, &machinev1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failed-machine-" + strconv.Itoa(i),
				Namespace: testNamespace,
				UID:       types.UID(strconv.Itoa(rand.Int())),
				Labels:    labels,
			},
			Status: machinev1.MachineStatus{
				CurrentStatus: machinev1.CurrentStatus{
					Phase: machinev1.MachineFailed,
				},
			},
		})
	}

	return
}

func generateEntry(template string, parameters ...interface{}) TableEntry {
	return Entry(fmt.Sprintf(template, parameters...), parameters...)
}

func FgenerateEntry(template string, parameters ...interface{}) TableEntry {
	return FEntry(fmt.Sprintf(template, parameters...), parameters...)
}
