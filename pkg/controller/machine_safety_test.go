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
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const namespace = "test"

var _ = Describe("machine", func() {
	DescribeTable("##freezeMachineSetsAndDeployments",
		func(machineSet *v1alpha1.MachineSet) {
			stop := make(chan struct{})
			defer close(stop)

			const freezeReason = OverShootingReplicaCount
			const freezeMessage = OverShootingReplicaCount

			objects := []runtime.Object{}
			if machineSet != nil {
				objects = append(objects, machineSet)
			}
			c, w := createController(stop, namespace, objects, nil)
			defer w.Stop()

			machineSets, err := c.controlMachineClient.MachineSets(machineSet.Namespace).List(metav1.ListOptions{})
			Expect(err).To(BeNil())
			Expect(machineSets).To(Not(BeNil()))
			for _, ms := range machineSets.Items {
				if ms.Name != machineSet.Name {
					continue
				}

				c.freezeMachineSetsAndDeployments(&ms, freezeReason, freezeMessage)
			}
		},
		Entry("one machineset", newMachineSet(&v1alpha1.MachineTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: "test",
			},
		}, 1, 10, nil, nil)),
	)

	DescribeTable("##unfreezeMachineSetsAndDeployments",
		func(machineSetExists, machineSetIsFrozen, parentExists, parentIsFrozen bool) {
			testMachineSet := newMachineSet(&v1alpha1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testMachineSet",
					Namespace: machinenamespace,
					Labels: map[string]string{
						"name": "testMachineDeployment",
					},
				},
			}, 1, 10, nil, nil)
			// testMachineSet := &v1alpha1.MachineSet{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      "testMachineSet",
			// 		Namespace: machinenamespace,
			// 		Labels: map[string]string{
			// 			"name": "testMachineDeployment",
			// 		},
			// 	},
			// 	Spec: v1alpha1.MachineSetSpec{
			// 		Selector: &metav1.LabelSelector{
			// 			MatchLabels: map[string]string{
			// 				"name": "testMachineDeployment",
			// 			},
			// 		},
			// 	},
			// }
			if machineSetIsFrozen {
				testMachineSet.Labels["freeze"] = "True"
				msStatus := testMachineSet.Status
				mscond := NewMachineSetCondition(v1alpha1.MachineSetFrozen, v1alpha1.ConditionTrue, "testing", "freezing the machineset")
				SetCondition(&msStatus, mscond)
			}

			testMachineDeployment := &v1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testMachineDeployment",
					Namespace: machinenamespace,
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

			objects := []runtime.Object{}
			if machineSetExists {
				objects = append(objects, testMachineSet)
			}
			if parentExists {
				objects = append(objects, testMachineDeployment)
			}
			c, w := createController(stop, namespace, objects, nil)
			defer w.Stop()

			Expect(cache.WaitForCacheSync(stop, c.machineSetSynced, c.machineDeploymentSynced)).To(BeTrue())

			c.unfreezeMachineSetsAndDeployments(testMachineSet)
			machineSet, err := c.controlMachineClient.MachineSets(testMachineSet.Namespace).Get(testMachineSet.Name, metav1.GetOptions{})
			if machineSetExists {
				Expect(machineSet.Labels["freeze"]).Should((BeEmpty()))
				Expect(GetCondition(&machineSet.Status, v1alpha1.MachineSetFrozen)).Should(BeNil())
				machineDeployment, err := c.controlMachineClient.MachineDeployments(testMachineDeployment.Namespace).Get(testMachineDeployment.Name, metav1.GetOptions{})
				if parentExists {
					Expect(machineDeployment.Labels["freeze"]).Should((BeEmpty()))
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

	DescribeTable("##checkAndFreezeORUnfreezeMachineSets",
		func(machineSet *v1alpha1.MachineSet) {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			if machineSet != nil {
				objects = append(objects, machineSet)
			}
			c, w := createController(stop, namespace, objects, nil)
			defer w.Stop()

			waitForCacheSync(stop, c)
			machineSets, err := c.machineSetLister.List(labels.Everything())
			Expect(err).To(BeNil())
			Expect(len(machineSets)).To(Equal(len(objects)))

			c.checkAndFreezeORUnfreezeMachineSets()
		},
		Entry("no objects", nil),
		Entry("one machineset", newMachineSet(&v1alpha1.MachineTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: "test",
			},
		}, 1, 10, nil, nil)),
	)
})
