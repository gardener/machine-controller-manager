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
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

var _ = Describe("#machine_safety", func() {

	DescribeTable("##freezeMachineSetsAndDeployments",
		func(machineSet *v1alpha1.MachineSet) {
			stop := make(chan struct{})
			defer close(stop)

			const freezeReason = OverShootingReplicaCount
			const freezeMessage = OverShootingReplicaCount

			controlMachineObjects := []runtime.Object{}
			if machineSet != nil {
				controlMachineObjects = append(controlMachineObjects, machineSet)
			}
			c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, nil)
			defer trackers.Stop()

			machineSets, err := c.controlMachineClient.MachineSets(machineSet.Namespace).List(metav1.ListOptions{})
			Expect(err).To(BeNil())
			Expect(machineSets).To(Not(BeNil()))
			for _, ms := range machineSets.Items {
				if ms.Name != machineSet.Name {
					continue
				}

				c.freezeMachineSetAndDeployment(&ms, freezeReason, freezeMessage)
			}
		},
		Entry("one machineset", newMachineSet(&v1alpha1.MachineTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: testNamespace,
			},
		}, 1, 10, nil, nil, nil, nil)),
	)

	DescribeTable("##unfreezeMachineSetsAndDeployments",
		func(machineSetExists, machineSetIsFrozen, parentExists, parentIsFrozen bool) {
			testMachineSet := newMachineSet(&v1alpha1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine",
					Namespace: testNamespace,
					Labels: map[string]string{
						"name": "testMachineDeployment",
					},
				},
			}, 1, 10, nil, nil, nil, map[string]string{})
			if machineSetIsFrozen {
				testMachineSet.Labels["freeze"] = "True"
				msStatus := testMachineSet.Status
				mscond := NewMachineSetCondition(v1alpha1.MachineSetFrozen, v1alpha1.ConditionTrue, "testing", "freezing the machineset")
				SetCondition(&msStatus, mscond)
			}

			testMachineDeployment := &v1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testMachineDeployment",
					Namespace: testNamespace,
					Labels:    map[string]string{},
				},
				Spec: v1alpha1.MachineDeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": "testMachineDeployment",
						},
					},
				},
			}
			if parentIsFrozen {
				testMachineDeployment.Labels["freeze"] = "True"
				mdStatus := testMachineDeployment.Status
				mdCond := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentFrozen, v1alpha1.ConditionTrue, "testing", "freezing the machinedeployment")
				SetMachineDeploymentCondition(&mdStatus, *mdCond)
			}

			stop := make(chan struct{})
			defer close(stop)

			controlMachineObjects := []runtime.Object{}
			if machineSetExists {
				controlMachineObjects = append(controlMachineObjects, testMachineSet)
			}
			if parentExists {
				controlMachineObjects = append(controlMachineObjects, testMachineDeployment)
			}
			c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, nil)
			defer trackers.Stop()

			Expect(cache.WaitForCacheSync(stop, c.machineSetSynced, c.machineDeploymentSynced)).To(BeTrue())

			c.unfreezeMachineSetAndDeployment(testMachineSet)
			machineSet, err := c.controlMachineClient.MachineSets(testMachineSet.Namespace).Get(testMachineSet.Name, metav1.GetOptions{})
			if machineSetExists {
				Expect(machineSet.Labels["freeze"]).Should((BeEmpty()))
				Expect(GetCondition(&machineSet.Status, v1alpha1.MachineSetFrozen)).Should(BeNil())
				machineDeployment, err := c.controlMachineClient.MachineDeployments(testMachineDeployment.Namespace).Get(testMachineDeployment.Name, metav1.GetOptions{})
				if parentExists {
					//Expect(machineDeployment.Labels["freeze"]).Should((BeEmpty()))
					Expect(GetMachineDeploymentCondition(machineDeployment.Status, v1alpha1.MachineDeploymentFrozen)).Should(BeNil())
				} else {
					Expect(err).ShouldNot(BeNil())
				}
			} else {
				Expect(err).ShouldNot(BeNil())
			}
		},

		//Entry("Testdata format::::::", machineSetExists, machineSetFrozen, parentExists, parentFrozen)
		Entry("existing, frozen machineset and machinedeployment", true, true, true, true),
		Entry("non-existing but frozen machineset and existing, frozen machinedeployment", false, true, true, true),
		Entry("existing, frozen machineset but non-existing, frozen machinedeployment", true, true, false, true),
	)

	type setup struct {
		machineReplicas              int
		machineSetAnnotations        map[string]string
		machineSetLabels             map[string]string
		machineSetConditions         []v1alpha1.MachineSetCondition
		machineDeploymentAnnotations map[string]string
		machineDeploymentLabels      map[string]string
		machineDeploymentConditions  []v1alpha1.MachineDeploymentCondition
	}
	type action struct {
	}
	type expect struct {
		machineSetAnnotations        map[string]string
		machineSetLabels             map[string]string
		machineSetConditions         []v1alpha1.MachineSetCondition
		machineDeploymentAnnotations map[string]string
		machineDeploymentLabels      map[string]string
		machineDeploymentConditions  []v1alpha1.MachineDeploymentCondition
	}
	type data struct {
		setup  setup
		action action
		expect expect
	}

	DescribeTable("##reconcileClusterMachineSafetyOvershooting",
		func(data *data) {
			stop := make(chan struct{})
			defer close(stop)

			machineDeployment := newMachineDeployment(
				&v1alpha1.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine",
						Namespace: testNamespace,
						Labels: map[string]string{
							"machine": "test",
						},
					},
				},
				1,
				10,
				&v1alpha1.MachineDeploymentStatus{
					Conditions: data.setup.machineDeploymentConditions,
				},
				nil,
				data.setup.machineDeploymentAnnotations,
				data.setup.machineDeploymentLabels,
			)
			machineSet := newMachineSetFromMachineDeployment(
				machineDeployment,
				1,
				&v1alpha1.MachineSetStatus{
					Conditions: data.setup.machineSetConditions,
				},
				data.setup.machineSetAnnotations,
				data.setup.machineSetLabels,
			)
			machines := newMachinesFromMachineSet(
				data.setup.machineReplicas,
				machineSet,
				&v1alpha1.MachineStatus{},
				nil,
				nil,
			)

			controlMachineObjects := []runtime.Object{}
			controlMachineObjects = append(controlMachineObjects, machineDeployment)
			controlMachineObjects = append(controlMachineObjects, machineSet)
			for _, o := range machines {
				controlMachineObjects = append(controlMachineObjects, o)
			}

			c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, nil)
			defer trackers.Stop()
			waitForCacheSync(stop, c)

			machineSets, err := c.machineSetLister.List(labels.Everything())
			Expect(err).To(BeNil())
			Expect(len(machineSets)).To(Equal(1))

			machines, err = c.machineLister.List(labels.Everything())
			Expect(err).To(BeNil())
			Expect(len(machines)).To(Equal(data.setup.machineReplicas))

			c.reconcileClusterMachineSafetyOvershooting("")
			waitForCacheSync(stop, c)

			ms, err := c.controlMachineClient.MachineSets(testNamespace).List(metav1.ListOptions{})
			Expect(err).To(BeNil())
			Expect(ms.Items[0].Labels).To(Equal(data.expect.machineSetLabels))
			if len(data.expect.machineSetConditions) >= 1 {
				Expect(ms.Items[0].Status.Conditions[0].Type).To(Equal(data.expect.machineSetConditions[0].Type))
				Expect(ms.Items[0].Status.Conditions[0].Status).To(Equal(data.expect.machineSetConditions[0].Status))
				Expect(ms.Items[0].Status.Conditions[0].Message).To(Equal(data.expect.machineSetConditions[0].Message))
			} else {
				Expect(len(ms.Items[0].Status.Conditions)).To(Equal(0))
			}
			Expect(len(ms.Items[0].Annotations)).To(Equal(len(data.expect.machineSetAnnotations)))

			machineDeploy, err := c.controlMachineClient.MachineDeployments(testNamespace).List(metav1.ListOptions{})
			Expect(err).To(BeNil())
			Expect(machineDeploy.Items[0].Labels).To(Equal(data.expect.machineDeploymentLabels))
			Expect(len(machineDeploy.Items[0].Status.Conditions)).To(Equal(len(data.expect.machineDeploymentConditions)))
			if len(data.expect.machineDeploymentConditions) >= 1 {
				Expect(machineDeploy.Items[0].Status.Conditions[0].Type).To(Equal(data.expect.machineDeploymentConditions[0].Type))
			} else {
				Expect(len(machineDeploy.Items[0].Status.Conditions)).To(Equal(0))
			}
			Expect(len(machineDeploy.Items[0].Annotations)).To(Equal(len(data.expect.machineDeploymentAnnotations)))
		},

		Entry("Freeze a machineSet and its machineDeployment", &data{
			setup: setup{
				machineReplicas: 4,
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"freeze":  "True",
					"machine": "test",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
		}),
		Entry("Unfreeze a machineSet and its machineDeployment", &data{
			setup: setup{
				machineReplicas: 3,
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentLabels: map[string]string{},
			},
		}),
		Entry("Continue to be Frozen for machineSet and its machineDeployment", &data{
			setup: setup{
				machineReplicas: 4,
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"freeze":  "True",
					"machine": "test",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
		}),
		Entry("Unfreezing machineset with freeze conditions and none on machinedeployment", &data{
			setup: setup{
				machineReplicas: 3,
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentLabels: map[string]string{},
			},
		}),
		Entry("Unfreezing machineset with freeze label and none on machinedeployment", &data{
			setup: setup{
				machineReplicas: 3,
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentLabels: map[string]string{},
			},
		}),
		Entry("Unfreezing machineset with freeze label and freeze label on machinedeployment", &data{
			setup: setup{
				machineReplicas: 3,
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentLabels: map[string]string{},
			},
		}),
		Entry("Unfreezing freeze conditions on machineset and machinedeployment", &data{
			setup: setup{
				machineReplicas: 3,
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
			},
			expect: expect{
				machineSetAnnotations: map[string]string{},
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentAnnotations: map[string]string{},
				machineDeploymentLabels:      map[string]string{},
			},
		}),
		Entry("Unfreeze machineSet manually and let machineDeployment continue to be frozen", &data{
			setup: setup{
				machineReplicas: 4,
				machineSetAnnotations: map[string]string{
					UnfreezeAnnotation: "True",
				},
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetAnnotations: map[string]string{},
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
		}),
		Entry("Unfreeze machineDeployment manually and also triggers machineSet to be unfrozen", &data{
			setup: setup{
				machineReplicas: 4,
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentAnnotations: map[string]string{
					UnfreezeAnnotation: "True",
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{

				machineSetLabels: map[string]string{
					"machine": "test",
				},

				machineDeploymentAnnotations: map[string]string{},
				machineDeploymentLabels:      map[string]string{},
			},
		}),
		Entry("Unfreeze both machineSet & machineDeployment manually", &data{
			setup: setup{
				machineReplicas: 4,
				machineSetAnnotations: map[string]string{
					UnfreezeAnnotation: "True",
				},
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentAnnotations: map[string]string{
					UnfreezeAnnotation: "True",
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetAnnotations: map[string]string{},
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentAnnotations: map[string]string{},
				machineDeploymentLabels:      map[string]string{},
			},
		}),
		Entry("Unfreeze a machineDeployment with freeze condition and no machinesets frozen", &data{
			setup: setup{
				machineReplicas: 1,
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentLabels: map[string]string{},
			},
		}),
		Entry("Unfreeze a machineDeployment with freeze label and no machinesets frozen", &data{
			setup: setup{
				machineReplicas: 1,
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
				},
				machineDeploymentLabels: map[string]string{},
			},
		}),
		Entry("Freeze a machineDeployment with freeze condition and machinesets frozen", &data{
			setup: setup{
				machineReplicas: 4,
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
					"freeze":  "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
		}),
		Entry("Freeze a machineDeployment that is not frozen and backing machineset frozen", &data{
			setup: setup{
				machineReplicas: 4,
				machineSetLabels: map[string]string{
					"freeze": "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
			expect: expect{
				machineSetLabels: map[string]string{
					"machine": "test",
					"freeze":  "True",
				},
				machineSetConditions: []v1alpha1.MachineSetCondition{
					{
						Type:    v1alpha1.MachineSetFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
				machineDeploymentLabels: map[string]string{
					"freeze": "True",
				},
				machineDeploymentConditions: []v1alpha1.MachineDeploymentCondition{
					{
						Type:    v1alpha1.MachineDeploymentFrozen,
						Status:  v1alpha1.ConditionTrue,
						Message: "The number of machines backing MachineSet: machineset-0 is 4 >= 4 which is the Max-ScaleUp-Limit",
					},
				},
			},
		}),
	)

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

			testMachine := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testmachine1",
					Namespace: testNamespace,
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: v1alpha1.MachineUnknown,
					},
				},
			}
			controlMachineObjects := []runtime.Object{}
			controlMachineObjects = append(controlMachineObjects, testMachine)

			c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, nil)
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
