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
	"sync"
	"time"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	fakeuntyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/fake"
	faketyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const namespace = "test"

var _ = Describe("machine", func() {
	DescribeTable("##freezeMachineSetsAndDeployments",
		func(initialMachines, initialMachineSets map[string]string, initialMachineDeployments []string, freezeMachineSets, freezeMachineDeployments []string) {
			createController := func(objects ...runtime.Object) *controller {
				fakeUntypedClient := fakeuntyped.NewSimpleClientset(objects...)
				fakeTypedClient := &faketyped.FakeMachineV1alpha1{
					&fakeUntypedClient.Fake,
				}

				return &controller{
					controlMachineClient: fakeTypedClient,
				}
			}

			const freezeReason = OverShootingReplicaCount
			const freezeMessage = OverShootingReplicaCount

			objects := newInitialObjects(initialMachines, initialMachineSets, initialMachineDeployments)
			c := createController(objects...)

			for _, msName := range freezeMachineSets {
				machineSets, err := c.controlMachineClient.MachineSets(namespace).List(metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(machineSets).To(Not(BeNil()))
				for _, ms := range machineSets.Items {
					if ms.Name != msName {
						continue
					}

					c.freezeMachineSetsAndDeployments(&ms, freezeReason, freezeMessage)
				}
			}
		},
		Entry("no objects", nil, nil, nil, nil, nil),
		Entry("one machineset", nil, map[string]string{"machineset-1": ""}, nil, []string{"machineset-1"}, nil),
	)

	DescribeTable("##checkAndFreezeORUnfreezeMachineSets",
		func(initialMachines, initialMachineSets map[string]string, initialMachineDeployments []string) {
			createController := func(stop <-chan struct{}, objects ...runtime.Object) *controller {
				fakeUntypedClient := fakeuntyped.NewSimpleClientset(objects...)
				fakeTypedClient := &faketyped.FakeMachineV1alpha1{
					&fakeUntypedClient.Fake,
				}
				controlMachineInformerFactory := machineinformers.NewSharedInformerFactory(fakeUntypedClient, 100*time.Millisecond)
				defer controlMachineInformerFactory.Start(stop)

				machineSharedInformers := controlMachineInformerFactory.Machine().V1alpha1()
				machines := machineSharedInformers.Machines()
				machineSets := machineSharedInformers.MachineSets()
				machineDeployments := machineSharedInformers.MachineDeployments()

				return &controller{
					controlMachineClient:    fakeTypedClient,
					machineLister:           machines.Lister(),
					machineSetLister:        machineSets.Lister(),
					machineDeploymentLister: machineDeployments.Lister(),
					machineSynced:           machines.Informer().HasSynced,
					machineSetSynced:        machineSets.Informer().HasSynced,
					machineDeploymentSynced: machineDeployments.Informer().HasSynced,
				}
			}

			stop := make(chan struct{})
			defer close(stop)

			objects := newInitialObjects(initialMachines, initialMachineSets, initialMachineDeployments)
			c := createController(stop, objects...)

			Expect(cache.WaitForCacheSync(stop, c.machineSynced, c.machineSetSynced, c.machineDeploymentSynced)).To(BeTrue())

			machineSets, err := c.machineSetLister.List(labels.Everything())
			Expect(err).To(BeNil())
			Expect(len(machineSets)).To(Equal(len(initialMachineSets)))

			wg := sync.WaitGroup{}
			wg.Add(1)
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			c.checkAndFreezeORUnfreezeMachineSets(&wg)

			select {
			case <-done:
			// All done!
			case <-time.After(100 * time.Millisecond):
				//Timeout!
			}

			Expect(done).To(BeClosed())
		},
		Entry("no objects", nil, nil, nil),
		Entry("one machineset", nil, map[string]string{"machineset-1": ""}, nil),
	)
})

func newMachineSet(name, namespace, owner string, labels map[string]string) *machinev1.MachineSet {
	ms := &machinev1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: machinev1.MachineSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}

	if owner == "" {
		return ms
	}

	ms.OwnerReferences = []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: controllerKindMachineSet.GroupVersion().String(),
			Kind:       controllerKindMachineSet.Kind,
			Name:       owner,
		},
	}

	return ms
}

func newInitialObjects(initialMachines, initialMachineSets map[string]string, initialMachineDeployments []string) []runtime.Object {
	objects := []runtime.Object{}
	for msName, owner := range initialMachineSets {
		objects = append(objects, newMachineSet(msName, namespace, owner, map[string]string{
			"machineset": owner,
		}))
	}
	return objects
}
