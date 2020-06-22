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
	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	machines = []*v1alpha1.Machine{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestMachineName,
				Namespace: TestNamespace,
			},
			TypeMeta: metav1.TypeMeta{},
			Spec: v1alpha1.MachineSpec{
				Class: v1alpha1.ClassSpec{
					Kind: GCPMachineClassKind,
					Name: TestMachineClassName,
				},
				ProviderID:           "",
				NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
				MachineConfiguration: &v1alpha1.MachineConfiguration{},
			},
			Status: v1alpha1.MachineStatus{},
		},
	}
	machineSets = []*v1alpha1.MachineSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestMachineSetName,
				Namespace: TestNamespace,
			},
			TypeMeta: metav1.TypeMeta{},
			Spec: v1alpha1.MachineSetSpec{
				Replicas: 0,
				Selector: &metav1.LabelSelector{},
				Template: v1alpha1.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.MachineSpec{
						Class: v1alpha1.ClassSpec{
							Kind: GCPMachineClassKind,
							Name: TestMachineClassName,
						},
						ProviderID:           "",
						NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
						MachineConfiguration: &v1alpha1.MachineConfiguration{},
					},
				},
				MinReadySeconds: 0,
			},
			Status: v1alpha1.MachineSetStatus{},
		},
	}
	machineDeployments = []*v1alpha1.MachineDeployment{
		{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestMachineDeploymentName,
				Namespace: TestNamespace,
			},
			Spec: v1alpha1.MachineDeploymentSpec{
				Replicas: 0,
				Selector: &metav1.LabelSelector{},
				Template: v1alpha1.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1alpha1.MachineSpec{
						Class: v1alpha1.ClassSpec{
							Kind: GCPMachineClassKind,
							Name: TestMachineClassName,
						},
						ProviderID:           "",
						NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
						MachineConfiguration: &v1alpha1.MachineConfiguration{},
					},
				},
				Strategy:                v1alpha1.MachineDeploymentStrategy{},
				MinReadySeconds:         0,
				RevisionHistoryLimit:    new(int32),
				Paused:                  false,
				RollbackTo:              &v1alpha1.RollbackConfig{},
				ProgressDeadlineSeconds: new(int32),
			},
			Status: v1alpha1.MachineDeploymentStatus{},
		},
	}
)

var _ = Describe("machine", func() {

	Describe("#TryMachineClassMigration", func() {
		type setup struct {
			gcpMachineClass     []*v1alpha1.GCPMachineClass
			fakeResourceActions *customfake.ResourceActions
			machineClass        []*v1alpha1.MachineClass
		}
		type action struct {
			classSpec  *v1alpha1.ClassSpec
			fakeDriver *driver.FakeDriver
		}
		type expect struct {
			err             error
			gcpMachineClass *v1alpha1.GCPMachineClass
			machineClass    *v1alpha1.MachineClass

			retry machineutils.Retry
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}
		DescribeTable("##gcp",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				machineObjects := []runtime.Object{}

				for _, o := range data.setup.gcpMachineClass {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range data.setup.machineClass {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range machines {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range machineSets {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range machineDeployments {
					machineObjects = append(machineObjects, o)
				}

				fakedriver := driver.NewFakeDriver(data.action.fakeDriver)

				controller, trackers := createController(stop, TestNamespace, machineObjects, nil, nil, fakedriver)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				action := data.action
				_, err := controller.controlMachineClient.GCPMachineClasses(TestNamespace).Get(action.classSpec.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				_, _, retry, err := controller.TryMachineClassMigration(data.action.classSpec)
				Expect(retry).To(Equal(data.expect.retry))
				waitForCacheSync(stop, controller)

				if data.expect.err != nil || err != nil {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(data.expect.err))
				} else {
					Expect(err).ToNot(HaveOccurred())

					actualGCPMachineClass, err := controller.controlMachineClient.GCPMachineClasses(TestNamespace).Get(action.classSpec.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(data.expect.gcpMachineClass).To(Equal(actualGCPMachineClass))

					actualMachineClass, err := controller.controlMachineClient.MachineClasses(TestNamespace).Get(action.classSpec.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(data.expect.machineClass).To(Equal(actualMachineClass))

					actualMachine, err := controller.controlMachineClient.Machines(TestNamespace).Get(TestMachineName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualMachine.Spec.Class.Kind).To(Equal(machineutils.MachineClassKind))

					actualMachineSet, err := controller.controlMachineClient.MachineSets(TestNamespace).Get(TestMachineSetName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualMachineSet.Spec.Template.Spec.Class.Kind).To(Equal(machineutils.MachineClassKind))

					actualMachineDeployment, err := controller.controlMachineClient.MachineDeployments(TestNamespace).Get(TestMachineDeploymentName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualMachineDeployment.Spec.Template.Spec.Class.Kind).To(Equal(machineutils.MachineClassKind))
				}
			},

			Entry("MachineClass migration successful for GCP machine class by creating new machine class", &data{
				setup: setup{
					gcpMachineClass: []*v1alpha1.GCPMachineClass{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      TestMachineClassName,
								Namespace: TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     v1alpha1.GCPMachineClassSpec{},
						},
					},
				},
				action: action{
					classSpec: &v1alpha1.ClassSpec{
						Kind: GCPMachineClassKind,
						Name: TestMachineClassName,
					},
					fakeDriver: &driver.FakeDriver{
						VMExists:   false,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					machineClass: &v1alpha1.MachineClass{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      TestMachineClassName,
							Namespace: TestNamespace,
						},
						ProviderSpec: runtime.RawExtension{},
						SecretRef:    nil,
						Provider:     "FakeProvider",
					},
					gcpMachineClass: &v1alpha1.GCPMachineClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      TestMachineClassName,
							Namespace: TestNamespace,
							Annotations: map[string]string{
								machineutils.MigratedMachineClass: "machine-controller-GCPMachineClass",
							},
						},
						TypeMeta: metav1.TypeMeta{},
						Spec:     v1alpha1.GCPMachineClassSpec{},
					},
					err:   nil,
					retry: true,
				},
			}),
			Entry("MachineClass migration successful for GCP machine class by updating existing machine class", &data{
				setup: setup{
					gcpMachineClass: []*v1alpha1.GCPMachineClass{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      TestMachineClassName,
								Namespace: TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     v1alpha1.GCPMachineClassSpec{},
						},
					},
					machineClass: []*v1alpha1.MachineClass{
						{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								Name:      TestMachineClassName,
								Namespace: TestNamespace,
							},
							ProviderSpec: runtime.RawExtension{},
							SecretRef:    nil,
							Provider:     "ERROR",
						},
					},
				},
				action: action{
					classSpec: &v1alpha1.ClassSpec{
						Kind: GCPMachineClassKind,
						Name: TestMachineClassName,
					},
					fakeDriver: &driver.FakeDriver{
						VMExists:   false,
						ProviderID: "fakeID-0",
						NodeName:   "fakeNode-0",
						Err:        nil,
					},
				},
				expect: expect{
					machineClass: &v1alpha1.MachineClass{
						TypeMeta: metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{
							Name:      TestMachineClassName,
							Namespace: TestNamespace,
						},
						ProviderSpec: runtime.RawExtension{},
						SecretRef:    nil,
						Provider:     "FakeProvider",
					},
					gcpMachineClass: &v1alpha1.GCPMachineClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      TestMachineClassName,
							Namespace: TestNamespace,
							Annotations: map[string]string{
								machineutils.MigratedMachineClass: "machine-controller-GCPMachineClass",
							},
						},
						TypeMeta: metav1.TypeMeta{},
						Spec:     v1alpha1.GCPMachineClassSpec{},
					},
					err:   nil,
					retry: true,
				},
			}),
		)
	})
})
