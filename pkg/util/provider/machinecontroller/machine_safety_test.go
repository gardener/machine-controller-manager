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

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("#machine_safety", func() {

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

var _ = Describe("machineCrashloopBackoff", func() {
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

			testMachineClass := &machinev1.MachineClass{
				ObjectMeta: *newObjectMeta(objMeta, 0),
				SecretRef:  testSecretReference,
			}

			controlCoreObjects := []runtime.Object{}
			controlCoreObjects = append(controlCoreObjects, testSecret)

			// Create test machine object in CrashloopBackoff state
			testMachineObject1 := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testmachine_1",
					Namespace: testNamespace,
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
						Phase: v1alpha1.MachineCrashLoopBackOff,
					},
				},
			}

			// Create another test machine object in Running state
			testMachineObject2 := &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testmachine_2",
					Namespace: testNamespace,
				},
				Status: machinev1.MachineStatus{
					CurrentStatus: machinev1.CurrentStatus{
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
			_, _ = c.checkMachineClass(testMachineClass)

			// after this, the testmachine in crashloopbackoff phase
			// should remain and the other one should
			// be deleted because it is an orphan VM
			listMachinesResponse, _ := fd.ListMachines(context.Background(), listMachinesRequest)

			Expect(listMachinesResponse.MachineList["testmachine-ip1"]).To(Equal("testmachine_1"))
			Expect(listMachinesResponse.MachineList["testmachine-ip2"]).To(Equal(""))
		})
	})
})
