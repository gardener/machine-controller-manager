// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

var _ = Describe("deployment_inplace", func() {
	Describe("syncMachineSets", func() {
		type setup struct {
			oldMachineSetReplicas                  int32
			oldMSMachinesMovedToNewMS              int32
			newMachineSetReplicas                  int32
			newMSMachinesWithUpdateSuccessfulLabel int32
		}
		type expect struct {
			oldMachineSetReplicas int32
			newMachineSetReplicas int32
		}
		type data struct {
			setup  setup
			expect expect
		}

		machineSets := newMachineSets(
			2,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-machine-class",
					},
				},
			}, 1, 500, &machinev1.MachineSetStatus{AvailableReplicas: 1}, nil, nil, map[string]string{"machineset": "old"})

		oldMachineSet := machineSets[0]
		newMachineSet := machineSets[1]
		oldMachineSet.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"machineset": "old"}}
		newMachineSet.Labels = map[string]string{"machineset": "new"}
		newMachineSet.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"machineset": "new"}}

		deployment := &machinev1.MachineDeployment{
			Spec: machinev1.MachineDeploymentSpec{
				Replicas: int32(3),
				Strategy: machinev1.MachineDeploymentStrategy{
					Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxUnavailable: ptr.To(intstr.FromInt32(1)),
							MaxSurge:       ptr.To(intstr.FromInt32(0)),
						},
					},
				},
			},
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				oldMachineSet.Spec.Replicas = data.setup.oldMachineSetReplicas
				newMachineSet.Spec.Replicas = data.setup.newMachineSetReplicas

				controlMachineObjects := []runtime.Object{}
				controlMachineObjects = append(controlMachineObjects, oldMachineSet, newMachineSet)

				machines := []*machinev1.Machine{}
				machines = append(machines, newMachinesFromMachineSet(int(data.setup.oldMachineSetReplicas-data.setup.oldMSMachinesMovedToNewMS), oldMachineSet, &machinev1.MachineStatus{}, nil, map[string]string{"machineset": "old"})...)

				newMachines := newMachinesFromMachineSet(int(data.setup.newMachineSetReplicas+data.setup.newMSMachinesWithUpdateSuccessfulLabel), newMachineSet, &machinev1.MachineStatus{}, nil, map[string]string{})
				machinesWithUpdateSuccessful := 0
				for i := range newMachines {
					newMachines[i].Labels = map[string]string{
						"machineset":           "new",
						machinev1.NodeLabelKey: fmt.Sprintf("node-%d", i),
					}

					if machinesWithUpdateSuccessful < int(data.setup.newMSMachinesWithUpdateSuccessfulLabel) {
						newMachines[i].Labels[machinev1.LabelKeyNodeUpdateResult] = machinev1.LabelValueNodeUpdateSuccessful

						newMachines[i].Status.Conditions = []corev1.NodeCondition{
							{
								Type:   machinev1.NodeInPlaceUpdate,
								Reason: machinev1.UpdateSuccessful,
							},
						}

						machinesWithUpdateSuccessful++
					}
				}

				machines = append(machines, newMachines...)
				for _, o := range machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				nodes := newNodes(int(data.setup.newMachineSetReplicas+data.setup.newMSMachinesWithUpdateSuccessfulLabel), map[string]string{}, &corev1.NodeSpec{}, nil)
				nodesWithUpdateSuccessful := 0
				for i := range nodes {
					if nodesWithUpdateSuccessful < int(data.setup.newMSMachinesWithUpdateSuccessfulLabel) {
						nodes[i].Labels = map[string]string{
							machinev1.LabelKeyNodeCandidateForUpdate: "true",
							machinev1.LabelKeyNodeSelectedForUpdate:  "true",
							machinev1.LabelKeyNodeUpdateResult:       machinev1.LabelValueNodeUpdateSuccessful,
						}
						nodes[i].Spec.Unschedulable = true
						nodesWithUpdateSuccessful++

						nodes[i].Status.Conditions = []corev1.NodeCondition{
							{
								Type:   machinev1.NodeInPlaceUpdate,
								Reason: machinev1.UpdateSuccessful,
							},
						}
					}
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				err := controller.syncMachineSets(context.TODO(), []*machinev1.MachineSet{oldMachineSet}, newMachineSet, deployment)
				Expect(err).ToNot(HaveOccurred())

				actualOldMachineSet, err := controller.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), oldMachineSet.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(actualOldMachineSet.Spec.Replicas).To(Equal(data.expect.oldMachineSetReplicas))

				actualNewMachineSet, err := controller.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), newMachineSet.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(actualNewMachineSet.Spec.Replicas).To(Equal(data.expect.newMachineSetReplicas))

				for _, expectedMachine := range newMachines {
					actualMachine, err := controller.controlMachineClient.Machines(testNamespace).Get(context.TODO(), expectedMachine.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualMachine.Labels).ToNot(HaveKey(machinev1.LabelKeyNodeUpdateResult))
				}

				actualNodes, err := controller.targetCoreClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				for i := range actualNodes.Items {
					node := actualNodes.Items[i]
					Expect(node.Spec.Unschedulable).To(Equal(false))
					Expect(node.Labels).ToNot(HaveKey(machinev1.LabelKeyNodeCandidateForUpdate))
					Expect(node.Labels).ToNot(HaveKey(machinev1.LabelKeyNodeSelectedForUpdate))
					Expect(node.Labels).ToNot(HaveKey(machinev1.LabelKeyNodeUpdateResult))
				}
			},

			Entry("no scaling required as there are no extra machines affiliated to new machine set with update successful label", &data{
				setup: setup{
					oldMachineSetReplicas:                  2,
					oldMSMachinesMovedToNewMS:              0,
					newMachineSetReplicas:                  3,
					newMSMachinesWithUpdateSuccessfulLabel: 0,
				},
				expect: expect{
					oldMachineSetReplicas: 2,
					newMachineSetReplicas: 3,
				},
			}),
			Entry("scale up new machine set because there are machines with update successful condition", &data{
				setup: setup{
					oldMachineSetReplicas:                  2,
					oldMSMachinesMovedToNewMS:              0,
					newMachineSetReplicas:                  2,
					newMSMachinesWithUpdateSuccessfulLabel: 1,
				},
				expect: expect{
					oldMachineSetReplicas: 2,
					newMachineSetReplicas: 3,
				},
			}),
			Entry("scale down old machine set because there are less machines than the replicas count", &data{
				setup: setup{
					oldMachineSetReplicas:                  2,
					oldMSMachinesMovedToNewMS:              1,
					newMachineSetReplicas:                  3,
					newMSMachinesWithUpdateSuccessfulLabel: 0,
				},
				expect: expect{
					oldMachineSetReplicas: 1,
					newMachineSetReplicas: 3,
				},
			}),
		)
	})

	Describe("reconcileNewMachineSetInPlace", func() {
		type setup struct {
			oldMachineSetReplicas     int32
			newMachineSetReplicas     int32
			nodesWithUpdateSuccessful int
		}
		type expect struct {
			scaled bool
		}
		type data struct {
			setup  setup
			expect expect
		}

		machineSets := newMachineSets(
			2,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-machine-class",
					},
				},
			}, 1, 500, &machinev1.MachineSetStatus{AvailableReplicas: 1}, nil, nil, map[string]string{"key": "value"})

		oldMachineSet := machineSets[0]
		newMachineSet := machineSets[1]

		deployment := &machinev1.MachineDeployment{
			Spec: machinev1.MachineDeploymentSpec{
				Replicas: int32(3),
				Strategy: machinev1.MachineDeploymentStrategy{
					Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxUnavailable: ptr.To(intstr.FromInt32(1)),
							MaxSurge:       ptr.To(intstr.FromInt32(0)),
						},
					},
				},
			},
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				oldMachineSet.Spec.Replicas = data.setup.oldMachineSetReplicas
				newMachineSet.Spec.Replicas = data.setup.newMachineSetReplicas

				controlMachineObjects := []runtime.Object{}
				controlMachineObjects = append(controlMachineObjects, oldMachineSet, newMachineSet)

				machines := []*machinev1.Machine{}
				machines = append(machines, newMachinesFromMachineSet(int(data.setup.oldMachineSetReplicas), oldMachineSet, &machinev1.MachineStatus{}, nil, map[string]string{"key": "value"})...)
				machines = append(machines, newMachinesFromMachineSet(int(data.setup.newMachineSetReplicas), newMachineSet, &machinev1.MachineStatus{}, nil, nil)...)
				machinesWithUpdateSuccessful := 0
				for i := range machines {
					machines[i].Labels = map[string]string{
						machinev1.NodeLabelKey: fmt.Sprintf("node-%d", i),
					}
					if machinesWithUpdateSuccessful < data.setup.nodesWithUpdateSuccessful {
						machines[i].Status.Conditions = []corev1.NodeCondition{
							{
								Type:   machinev1.NodeInPlaceUpdate,
								Reason: machinev1.UpdateSuccessful,
							},
						}
						machinesWithUpdateSuccessful++
					}
				}

				for _, o := range machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				nodes := newNodes(int(data.setup.oldMachineSetReplicas), map[string]string{}, &corev1.NodeSpec{}, nil)
				nodesWithUpdateSuccessful := 0
				for i := range nodes {
					if nodesWithUpdateSuccessful < data.setup.nodesWithUpdateSuccessful {
						nodes[i].Labels = map[string]string{machinev1.LabelKeyNodeUpdateResult: machinev1.LabelValueNodeUpdateSuccessful}
						nodes[i].Status.Conditions = []corev1.NodeCondition{
							{
								Type:   machinev1.NodeInPlaceUpdate,
								Reason: machinev1.UpdateSuccessful,
							},
						}
						nodesWithUpdateSuccessful++
					}
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				scaled, err := controller.reconcileNewMachineSetInPlace(context.TODO(), []*machinev1.MachineSet{oldMachineSet}, newMachineSet, deployment)
				Expect(err).ToNot(HaveOccurred())
				Expect(scaled).To(Equal(data.expect.scaled))

				machinesWithUpdateSuccessful = 0
				for i := range machines {
					if machinesWithUpdateSuccessful < data.setup.nodesWithUpdateSuccessful {
						actualMachine, err := controller.controlMachineClient.Machines(testNamespace).Get(context.TODO(), machines[i].Name, metav1.GetOptions{})
						Expect(err).ToNot(HaveOccurred())
						Expect(actualMachine.Labels).To(HaveKey(machinev1.LabelKeyNodeUpdateResult))
						machinesWithUpdateSuccessful++
					}
				}
			},

			Entry("no scaling required as newMachineSet replicas match deployment replicas", &data{
				setup: setup{
					oldMachineSetReplicas:     2,
					newMachineSetReplicas:     3,
					nodesWithUpdateSuccessful: 0,
				},
				expect: expect{
					scaled: false,
				},
			}),
			Entry("scale down newMachineSet as it has more replicas than deployment", &data{
				setup: setup{
					oldMachineSetReplicas:     2,
					newMachineSetReplicas:     4,
					nodesWithUpdateSuccessful: 0,
				},
				expect: expect{
					scaled: true,
				},
			}),
			Entry("scale up newMachineSet by transferring machines from oldMachineSet", &data{
				setup: setup{
					oldMachineSetReplicas:     2,
					newMachineSetReplicas:     1,
					nodesWithUpdateSuccessful: 1,
				},
				expect: expect{
					scaled: true,
				},
			}),
			Entry("scale up newMachineSet by scaling up newMachineSet if there are zero machines in oldMachineSet", &data{
				setup: setup{
					oldMachineSetReplicas:     0,
					newMachineSetReplicas:     1,
					nodesWithUpdateSuccessful: 0,
				},
				expect: expect{
					scaled: true,
				},
			}),
		)
	})

	Describe("reconcileOldMachineSetsInPlace", func() {
		type setup struct {
			oldMachineSetReplicas           int32
			oldISAvailableMachines          int32
			oldISCandidateForUpdateMachines int
			oldISSelectedForUpdateMachines  int
			newMachineSetReplicas           int32
			newISAvailableMachines          int32
		}
		type expect struct {
			scaled bool
		}
		type data struct {
			setup  setup
			expect expect
		}

		machineSets := newMachineSets(
			2,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-machine-class",
					},
				},
			}, 1, 500, &machinev1.MachineSetStatus{AvailableReplicas: 1}, nil, nil, nil,
		)

		oldMachineSet := machineSets[0]
		newMachineSet := machineSets[1]

		deployment := &machinev1.MachineDeployment{
			Spec: machinev1.MachineDeploymentSpec{
				Replicas: int32(3),
				Strategy: machinev1.MachineDeploymentStrategy{
					Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxUnavailable: ptr.To(intstr.FromInt32(1)),
							MaxSurge:       ptr.To(intstr.FromInt32(0)),
						},
					},
				},
			},
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				oldMachineSet.Spec.Replicas = data.setup.oldMachineSetReplicas
				oldMachineSet.Status.AvailableReplicas = data.setup.oldISAvailableMachines
				newMachineSet.Spec.Replicas = data.setup.newMachineSetReplicas
				newMachineSet.Status.AvailableReplicas = data.setup.newISAvailableMachines

				controlMachineObjects := []runtime.Object{}
				controlMachineObjects = append(controlMachineObjects, oldMachineSet, newMachineSet)

				machines := []*machinev1.Machine{}
				machines = append(machines, newMachinesFromMachineSet(int(data.setup.oldMachineSetReplicas), oldMachineSet, &machinev1.MachineStatus{}, nil, nil)...)
				machines = append(machines, newMachinesFromMachineSet(int(data.setup.newMachineSetReplicas), newMachineSet, &machinev1.MachineStatus{}, nil, nil)...)
				for i, machine := range machines {
					machine.Labels[machinev1.NodeLabelKey] = fmt.Sprintf("node-%d", i)
				}

				for _, o := range machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				nodesCandidateForUpdate := 0
				nodesSelectedForUpdate := 0
				nodes := newNodes(len(machines), map[string]string{}, &corev1.NodeSpec{}, nil)
				for i := range machines {
					nodes[i].Labels = machines[i].Labels
					if nodesCandidateForUpdate < data.setup.oldISCandidateForUpdateMachines {
						nodes[i].Labels[machinev1.LabelKeyNodeCandidateForUpdate] = "true"
						if nodesSelectedForUpdate < data.setup.oldISSelectedForUpdateMachines {
							nodes[i].Labels[machinev1.LabelKeyNodeSelectedForUpdate] = "true"
							nodesSelectedForUpdate++
						}
						nodesCandidateForUpdate++
					}
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				count, err := controller.reconcileOldMachineSetsInPlace(context.TODO(), []*machinev1.MachineSet{oldMachineSet, newMachineSet}, []*machinev1.MachineSet{oldMachineSet}, newMachineSet, deployment)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(data.expect.scaled))
			},
			Entry("no machines selected for update because there is no machines with candidate for update label", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 0,
					oldISSelectedForUpdateMachines:  0,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          1,
				},
				expect: expect{
					scaled: false,
				},
			}),
			Entry("no machines selected for update because there is not enough available replicas because some of new machines are unavailable", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 1,
					oldISSelectedForUpdateMachines:  0,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          0,
				},
				expect: expect{
					scaled: false,
				},
			}),
			Entry("no machines selected for update because there is still old replicas undergoing update respecting min avaialble", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 1,
					oldISSelectedForUpdateMachines:  1,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          1,
				},
				expect: expect{
					scaled: false,
				},
			}),
			Entry("select one machine even though mutiple machines can be updated respecting min available", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 2,
					oldISSelectedForUpdateMachines:  0,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          1,
				},
				expect: expect{
					scaled: true,
				},
			}),
			Entry("scale down all old machine sets if new machine set already has replicas equal to deployment replicas", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 2,
					oldISSelectedForUpdateMachines:  0,
					newMachineSetReplicas:           3,
					newISAvailableMachines:          3,
				},
				expect: expect{
					scaled: true,
				},
			}),
			Entry("no scale down because there is no old machine set with replicas > 0", &data{
				setup: setup{
					oldMachineSetReplicas:           0,
					oldISAvailableMachines:          0,
					oldISCandidateForUpdateMachines: 0,
					oldISSelectedForUpdateMachines:  0,
					newMachineSetReplicas:           3,
					newISAvailableMachines:          3,
				},
				expect: expect{
					scaled: false,
				},
			}),
		)
	})

	Describe("selectNumOfMachineForUpdate", func() {
		type setup struct {
			oldMachineSetReplicas           int32
			oldISAvailableMachines          int32
			oldISCandidateForUpdateMachines int
			newMachineSetReplicas           int32
			newISAvailableMachines          int32
		}
		type expect struct {
			count int32
		}
		type data struct {
			setup  setup
			action int32
			expect expect
		}

		machineSets := newMachineSets(
			2,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-machine-class",
					},
				},
			}, 1, 500, &machinev1.MachineSetStatus{AvailableReplicas: 1}, nil, nil, nil,
		)

		oldMachineSet := machineSets[0]
		newMachineSet := machineSets[1]

		deployment := &machinev1.MachineDeployment{
			Spec: machinev1.MachineDeploymentSpec{
				Replicas: int32(3),
				Strategy: machinev1.MachineDeploymentStrategy{
					Type: machinev1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: machinev1.UpdateConfiguration{
							MaxUnavailable: ptr.To(intstr.FromInt32(1)),
							MaxSurge:       ptr.To(intstr.FromInt32(0)),
						},
					},
				},
			},
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				oldMachineSet.Spec.Replicas = data.setup.oldMachineSetReplicas
				oldMachineSet.Status.AvailableReplicas = data.setup.oldISAvailableMachines
				newMachineSet.Spec.Replicas = data.setup.newMachineSetReplicas
				newMachineSet.Status.AvailableReplicas = data.setup.newISAvailableMachines

				controlMachineObjects := []runtime.Object{}
				controlMachineObjects = append(controlMachineObjects, oldMachineSet, newMachineSet)

				machines := []*machinev1.Machine{}
				machines = append(machines, newMachinesFromMachineSet(int(data.setup.oldMachineSetReplicas), oldMachineSet, &machinev1.MachineStatus{}, nil, nil)...)
				for i := range data.setup.oldISCandidateForUpdateMachines {
					machines[i].Labels[machinev1.LabelKeyNodeCandidateForUpdate] = "true"
				}

				machines = append(machines, newMachinesFromMachineSet(int(data.setup.newMachineSetReplicas), newMachineSet, &machinev1.MachineStatus{}, nil, nil)...)
				for i, machine := range machines {
					machine.Labels[machinev1.NodeLabelKey] = fmt.Sprintf("node-%d", i)
				}

				for _, o := range machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				nodes := newNodes(len(machines), map[string]string{}, &corev1.NodeSpec{}, nil)
				for i := range machines {
					nodes[i].Labels = machines[i].Labels
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				count, err := controller.selectNumOfMachineForUpdate(context.TODO(), []*machinev1.MachineSet{oldMachineSet, newMachineSet}, []*machinev1.MachineSet{oldMachineSet}, newMachineSet, deployment, data.action)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(data.expect.count))
			},
			Entry("no machines selected for update because there is no machines with candidate for update label", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 0,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          1,
				},
				action: 0,
				expect: expect{
					count: 0,
				},
			}),
			Entry("no machines selected for update because there is not enough available replicas because some of new machines are unavailable", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 1,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          0,
				},
				action: 0,
				expect: expect{
					count: 0,
				},
			}),
			Entry("no machines selected for update because there is still old replicas undergoing update respecting min avaialble", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 1,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          1,
				},
				action: 1,
				expect: expect{
					count: 0,
				},
			}),
			Entry("select one machine even though multiple machines can be updated respecting min available", &data{
				setup: setup{
					oldMachineSetReplicas:           2,
					oldISAvailableMachines:          2,
					oldISCandidateForUpdateMachines: 2,
					newMachineSetReplicas:           1,
					newISAvailableMachines:          1,
				},
				action: 0,
				expect: expect{
					count: 1,
				},
			}),
		)
	})

	Describe("labelNodesBackingMachineSets", func() {
		type setup struct {
			nodes       []*corev1.Node
			machineSets []*machinev1.MachineSet
			machines    []*machinev1.Machine
		}
		type expect struct {
			machines []*machinev1.Machine
			nodes    []*corev1.Node
			err      bool
		}
		type data struct {
			setup  setup
			action []*machinev1.MachineSet
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			Namespace: testNamespace,
		}
		machineSets := newMachineSets(
			1,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
					Labels: map[string]string{
						"key": "value",
					},
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-machine-class",
					},
				},
			}, 3, 500, nil, nil, nil, nil,
		)

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				for _, o := range data.setup.machineSets {
					controlMachineObjects = append(controlMachineObjects, o)
				}
				for _, o := range data.setup.machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range data.setup.nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				err := controller.labelNodesBackingMachineSets(context.TODO(), data.action, "key", "value")
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				for _, expectedMachine := range data.expect.machines {
					actualMachine, err := controller.controlMachineClient.Machines(testNamespace).Get(context.TODO(), expectedMachine.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualMachine.Labels).Should(Equal(expectedMachine.Labels))
				}

				for _, expectedNode := range data.expect.nodes {
					actualNode, err := controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), expectedNode.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualNode.Labels).Should(Equal(expectedNode.Labels))
				}
			},

			Entry("labels on nodes backing machineSet", &data{
				setup: setup{
					machines: newMachinesFromMachineSet(1, machineSets[0], &machinev1.MachineStatus{}, nil, map[string]string{machinev1.NodeLabelKey: "node-0"}),
					nodes:    newNodes(1, nil, &corev1.NodeSpec{}, nil),
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "MachineClass",
								Name: "test-machine-class",
							},
						},
					}, 3, 500, nil, nil, nil, nil,
				),
				expect: expect{
					machines: newMachinesFromMachineSet(1, machineSets[0], &machinev1.MachineStatus{}, nil, map[string]string{machinev1.NodeLabelKey: "node-0", "key": "value"}),
					nodes:    newNodes(1, map[string]string{"key": "value"}, &corev1.NodeSpec{}, nil),
					err:      false,
				},
			}),
		)
	})

	Describe("getMachinesUndergoingUpdate", func() {
		type setup struct {
			machineSets []*machinev1.MachineSet
			machines    []*machinev1.Machine
		}
		type expect struct {
			count int32
			err   bool
		}
		type data struct {
			setup  setup
			action []*machinev1.MachineSet
			expect expect
		}

		objMeta := &metav1.ObjectMeta{
			Namespace: testNamespace,
		}

		machineSets := newMachineSets(
			1,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-machine-class",
					},
				},
			}, 3, 500, nil, nil, nil, nil,
		)

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				for _, o := range data.setup.machineSets {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				for i, machine := range data.setup.machines {
					if machine.Labels == nil {
						machine.Labels = map[string]string{}
					}
					machine.Labels[machinev1.NodeLabelKey] = fmt.Sprintf("node-%d", i)
				}

				for _, o := range data.setup.machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				nodes := newNodes(len(data.setup.machines), map[string]string{}, &corev1.NodeSpec{}, nil)
				for i := range data.setup.machines {
					nodes[i].Labels = data.setup.machines[i].Labels
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				count, err := controller.getMachinesUndergoingUpdate(data.action)
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				Expect(count).To(Equal(data.expect.count))
			},
			Entry("no machines undergoing update", &data{
				setup: setup{
					machineSets: machineSets,
					machines:    newMachinesFromMachineSet(1, machineSets[0], &machinev1.MachineStatus{}, nil, nil),
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "MachineClass",
								Name: "test-machine-class",
							},
						},
					}, 3, 500, nil, nil, nil, nil,
				),
				expect: expect{
					count: 0,
					err:   false,
				},
			}),
			Entry("one machine undergoing update", &data{
				setup: setup{
					machineSets: machineSets,
					machines:    newMachinesFromMachineSet(1, machineSets[0], &machinev1.MachineStatus{}, nil, map[string]string{machinev1.LabelKeyNodeSelectedForUpdate: "true"}),
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "MachineClass",
								Name: "test-machine-class",
							},
						},
					}, 3, 500, nil, nil, nil, nil,
				),
				expect: expect{
					count: 1,
					err:   false,
				},
			}),
			Entry("multiple machines undergoing update", &data{
				setup: setup{
					machineSets: machineSets,
					machines:    newMachinesFromMachineSet(2, machineSets[0], &machinev1.MachineStatus{}, nil, map[string]string{machinev1.LabelKeyNodeSelectedForUpdate: "true"}),
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "MachineClass",
								Name: "test-machine-class",
							},
						},
					}, 3, 500, nil, nil, nil, nil,
				),
				expect: expect{
					count: 2,
					err:   false,
				},
			}),
			Entry("only one machines is undergoing update", &data{
				setup: setup{
					machineSets: machineSets,
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.LabelKeyNodeSelectedForUpdate: "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-1",
								Namespace: testNamespace,
							},
						},
					},
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "MachineClass",
								Name: "test-machine-class",
							},
						},
					}, 3, 500, nil, nil, nil, nil,
				),
				expect: expect{
					count: 1,
					err:   false,
				},
			}),
			Entry("there are no machines", &data{
				setup: setup{
					machines:    []*machinev1.Machine{},
					machineSets: []*machinev1.MachineSet{},
				},
				action: []*machinev1.MachineSet{},
				expect: expect{
					count: 0,
					err:   false,
				},
			}),
		)
	})

	Describe("getMachinesForDrain", func() {
		type setup struct {
			machineSet *machinev1.MachineSet
			machines   []*machinev1.Machine
		}
		type expect struct {
			machines []*machinev1.Machine
			err      bool
		}
		type data struct {
			setup  setup
			action int32
			expect expect
		}

		machineSet := newMachineSets(
			1,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-machine-class",
					},
				},
			}, 3, 500, nil, nil, nil, nil,
		)[0]

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{data.setup.machineSet}
				for _, o := range data.setup.machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				nodes := newNodes(len(data.setup.machines), map[string]string{}, &corev1.NodeSpec{}, nil)
				for i := range data.setup.machines {
					nodes[i].Labels = data.setup.machines[i].Labels
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				machines, err := controller.getMachinesForDrain(data.setup.machineSet, data.action)
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				Expect(len(machines)).To(Equal(len(data.expect.machines)))
			},
			Entry("select machines for drain", &data{
				setup: setup{
					machineSet: machineSet,
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-1",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 1),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
									machinev1.LabelKeyNodeSelectedForUpdate:  "true",
								},
							},
						},
					},
				},
				action: 1,
				expect: expect{
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
					},
					err: false,
				},
			}),
			Entry("select all machines for drain", &data{
				setup: setup{
					machineSet: machineSet,
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-1",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 1),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
					},
				},
				action: 2,
				expect: expect{
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-1",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 1),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
					},
					err: false,
				},
			}),
			Entry("select only required count of machines even though more machines can be selected", &data{
				setup: setup{
					machineSet: machineSet,
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-1",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 1),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
					},
				},
				action: 1,
				expect: expect{
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
					},
					err: false,
				},
			}),
			Entry("select only machines which has label candidate for update", &data{
				setup: setup{
					machineSet: machineSet,
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-1",
								Namespace: testNamespace,
							},
						},
					},
				},
				action: 2,
				expect: expect{
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
								},
							},
						},
					},
					err: false,
				},
			}),
			Entry("no machines available for drain all are already selected", &data{
				setup: setup{
					machineSet: machineSet,
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 0),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
									machinev1.LabelKeyNodeSelectedForUpdate:  "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-1",
								Namespace: testNamespace,
								Labels: map[string]string{
									machinev1.NodeLabelKey:                   fmt.Sprintf("node-%d", 1),
									machinev1.LabelKeyNodeCandidateForUpdate: "true",
									machinev1.LabelKeyNodeSelectedForUpdate:  "true",
								},
							},
						},
					},
				},
				action: 1,
				expect: expect{
					machines: []*machinev1.Machine{},
					err:      false,
				},
			}),
			Entry("no machines available for drain", &data{
				setup: setup{
					machineSet: machineSet,
					machines: []*machinev1.Machine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-0",
								Namespace: testNamespace,
							},
						},
					},
				},
				action: 1,
				expect: expect{
					machines: []*machinev1.Machine{},
					err:      false,
				},
			}),
		)
	})
})
