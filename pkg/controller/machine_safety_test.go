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
			c := createController(stop, namespace, objects, nil)
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

	DescribeTable("##checkAndFreezeORUnfreezeMachineSets",
		func(machineSet *v1alpha1.MachineSet) {
			stop := make(chan struct{})
			defer close(stop)

			objects := []runtime.Object{}
			if machineSet != nil {
				objects = append(objects, machineSet)
			}
			c := createController(stop, namespace, objects, nil)
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
