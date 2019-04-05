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

var (
	fiveSecondsBeforeNow = time.Now().Add(-time.Duration(5 * time.Second))
	fiveMinutesBeforeNow = time.Now().Add(-time.Duration(5 * time.Minute))
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

				c.freezeMachineSetsAndDeployments(&ms, freezeReason, freezeMessage)
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

			c.unfreezeMachineSetsAndDeployments(testMachineSet)
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

	DescribeTable("##checkAndFreezeORUnfreezeMachineSets",
		func(machineSet *v1alpha1.MachineSet) {
			stop := make(chan struct{})
			defer close(stop)

			controlMachineObjects := []runtime.Object{}
			if machineSet != nil {
				controlMachineObjects = append(controlMachineObjects, machineSet)
			}
			c, trackers := createController(stop, testNamespace, controlMachineObjects, nil, nil)
			defer trackers.Stop()

			waitForCacheSync(stop, c)
			machineSets, err := c.machineSetLister.List(labels.Everything())
			Expect(err).To(BeNil())
			Expect(len(machineSets)).To(Equal(len(controlMachineObjects)))

			c.checkAndFreezeORUnfreezeMachineSets()
		},
		Entry("no objects", nil),
		Entry("one machineset", newMachineSet(&v1alpha1.MachineTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: testNamespace,
			},
		}, 1, 10, nil, nil, nil)),
	)

	DescribeTable("##reconcileClusterMachineSafetyAPIServer",
		func(
			controlAPIServerIsUp bool,
			targetAPIServerIsUp bool,
			apiServerInactiveStartTime time.Time,
			preMachineControllerIsFrozen bool,
			postMachineControllerFrozen bool,
		) {
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
			true, true, time.Time{}, false, false),
		Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: false",
			true, true, time.Time{}, true, false),
		Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
			true, true, fiveSecondsBeforeNow, false, false),
		Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: false",
			true, true, fiveSecondsBeforeNow, true, false),
		Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: false",
			true, true, fiveMinutesBeforeNow, false, false),
		Entry("Control APIServer: Reachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: false",
			true, true, fiveMinutesBeforeNow, true, false),

		// Target APIServer is not reachable
		Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: false = Post-Frozen: false",
			true, false, time.Time{}, false, false),
		Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: true",
			true, false, time.Time{}, true, true),
		Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
			true, false, fiveSecondsBeforeNow, false, false),
		Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: true",
			true, false, fiveSecondsBeforeNow, true, true),
		Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: true",
			true, false, fiveMinutesBeforeNow, false, true),
		Entry("Control APIServer: Reachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: true",
			true, false, fiveMinutesBeforeNow, true, true),

		// Control APIServer is not reachable
		Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Inactive, Pre-Frozen: false = Post-Frozen: false",
			false, true, time.Time{}, false, false),
		Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: true",
			false, true, time.Time{}, true, true),
		Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
			false, true, fiveSecondsBeforeNow, false, false),
		Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: true",
			false, true, fiveSecondsBeforeNow, true, true),
		Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: true",
			false, true, fiveMinutesBeforeNow, false, true),
		Entry("Control APIServer: UnReachable, Target APIServer: Reachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: true",
			false, true, fiveMinutesBeforeNow, true, true),

		// Both APIServers are not reachable
		Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: false = Post-Frozen: false",
			false, false, time.Time{}, false, false),
		Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Inactive, Pre-Frozen: true = Post-Frozen: true",
			false, false, time.Time{}, true, true),
		Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: false = Post-Frozen: false",
			false, false, fiveSecondsBeforeNow, false, false),
		Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Started, Pre-Frozen: true = Post-Frozen: true",
			false, false, fiveSecondsBeforeNow, true, true),
		Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: false = Post-Frozen: true",
			false, false, fiveMinutesBeforeNow, false, true),
		Entry("Control APIServer: UnReachable, Target APIServer: UnReachable, Inactive Timer: Elapsed, Pre-Frozen: true = Post-Frozen: true",
			false, false, fiveMinutesBeforeNow, true, true),
	)
})
