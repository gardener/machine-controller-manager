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

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	fakemachineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

var _ = Describe("machine", func() {
	var (
		fakeMachineClient *fakemachineapi.FakeMachineV1alpha1
		c                 *controller
	)

	Describe("##updateMachineStatus", func() {
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
					Namespace: "test",
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
})
