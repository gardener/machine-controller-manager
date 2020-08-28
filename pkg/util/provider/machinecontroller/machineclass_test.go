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
	"fmt"
	"math"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	TestMachineName           = "test-machine"
	TestMachineSetName        = "test-machine-set"
	TestMachineDeploymentName = "test-machine-deployment"
	TestMachineClassName      = "test-mc"
	TestNamespace             = "test-ns"
)

var _ = Describe("machineclass", func() {
	Describe("#reconcileClusterMachineClass", func() {
		type setup struct {
			machineClasses      []*v1alpha1.MachineClass
			machines            []*v1alpha1.Machine
			fakeResourceActions *customfake.ResourceActions
		}
		type action struct {
			fakeDriver       *driver.FakeDriver
			machineClassName string
		}
		type expect struct {
			machineClass *v1alpha1.MachineClass
			err          error
		}
		type data struct {
			setup  setup
			action action
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				machineObjects := []runtime.Object{}
				for _, o := range data.setup.machineClasses {
					machineObjects = append(machineObjects, o)
				}
				for _, o := range data.setup.machines {
					machineObjects = append(machineObjects, o)
				}

				fakeDriver := driver.NewFakeDriver(
					data.action.fakeDriver,
				)

				controller, trackers := createController(stop, TestNamespace, machineObjects, nil, nil, fakeDriver)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				action := data.action
				machineClass, err := controller.controlMachineClient.MachineClasses(TestNamespace).Get(action.machineClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if data.setup.fakeResourceActions != nil {
					trackers.TargetCore.SetFakeResourceActions(data.setup.fakeResourceActions, math.MaxInt32)
				}

				err = controller.reconcileClusterMachineClass(machineClass)
				if data.expect.err == nil {
					Expect(err).To(Not(HaveOccurred()))
				} else {
					Expect(err).To(Equal(data.expect.err))
				}

				machineClass, err = controller.controlMachineClient.MachineClasses(TestNamespace).Get(action.machineClassName, metav1.GetOptions{})
				Expect(err).To(Not(HaveOccurred()))
				Expect(data.expect.machineClass).To(Equal(machineClass))

				// Expect(machine.Spec).To(Equal(data.expect.machine.Spec))
				// Expect(machine.Status.CurrentStatus.Phase).To(Equal(data.expect.machine.Status.CurrentStatus.Phase))
				// Expect(machine.Status.LastOperation.State).To(Equal(data.expect.machine.Status.LastOperation.State))
				// Expect(machine.Status.LastOperation.Type).To(Equal(data.expect.machine.Status.LastOperation.Type))
				// Expect(machine.Status.LastOperation.Description).To(Equal(data.expect.machine.Status.LastOperation.Description))
				// Expect(machine.Finalizers).To(Equal(data.expect.machine.Finalizers))

			},
			Entry(
				"Add finalizer to machine class",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.MachineClass{
							{
								TypeMeta: metav1.TypeMeta{},
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineClassName,
									Namespace: TestNamespace,
								},
								ProviderSpec: runtime.RawExtension{},
								SecretRef:    &v1.SecretReference{},
								Provider:     "",
							},
						},
						machines: []*v1alpha1.Machine{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.MachineSpec{
									Class: v1alpha1.ClassSpec{
										Name: TestMachineClassName,
										Kind: machineutils.MachineClassKind,
									},
									ProviderID:           "",
									NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
									MachineConfiguration: &v1alpha1.MachineConfiguration{},
								},
								Status: v1alpha1.MachineStatus{},
							},
						},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.MachineClass{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
								Finalizers: []string{MCMFinalizerName},
							},
							ProviderSpec: runtime.RawExtension{},
							SecretRef:    &v1.SecretReference{},
							Provider:     "",
						},
						err: nil,
					},
				},
			),
			Entry(
				"Finalizer exists, so do nothing",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.MachineClass{
							{
								TypeMeta: metav1.TypeMeta{},
								ObjectMeta: metav1.ObjectMeta{
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
									Finalizers: []string{MCMFinalizerName},
								},
								ProviderSpec: runtime.RawExtension{},
								SecretRef:    &v1.SecretReference{},
								Provider:     "",
							},
						},
						machines: []*v1alpha1.Machine{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.MachineSpec{
									Class: v1alpha1.ClassSpec{
										Name: TestMachineClassName,
										Kind: machineutils.MachineClassKind,
									},
									ProviderID:           "",
									NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
									MachineConfiguration: &v1alpha1.MachineConfiguration{},
								},
								Status: v1alpha1.MachineStatus{},
							},
						},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.MachineClass{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
								Finalizers: []string{MCMFinalizerName},
							},
							ProviderSpec: runtime.RawExtension{},
							SecretRef:    &v1.SecretReference{},
							Provider:     "",
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class deletion fails as machine objects are still referring the machine class hence retry",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.MachineClass{
							{
								TypeMeta: metav1.TypeMeta{},
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: &metav1.Time{
										Time: time.Time{},
									},
									Finalizers: []string{MCMFinalizerName},
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
								},
								ProviderSpec: runtime.RawExtension{},
								SecretRef:    &v1.SecretReference{},
								Provider:     "",
							},
						},
						machines: []*v1alpha1.Machine{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.MachineSpec{
									Class: v1alpha1.ClassSpec{
										Name: TestMachineClassName,
										Kind: machineutils.MachineClassKind,
									},
									ProviderID:           "",
									NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
									MachineConfiguration: &v1alpha1.MachineConfiguration{},
								},
								Status: v1alpha1.MachineStatus{},
							},
						},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.MachineClass{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
								Finalizers: []string{MCMFinalizerName},
							},
							ProviderSpec: runtime.RawExtension{},
							SecretRef:    &v1.SecretReference{},
							Provider:     "",
						},
						err: fmt.Errorf("Retry as machine objects are still referring the machineclass"),
					},
				},
			),
			Entry(
				"Class deletion succeeds with removal of finalizer",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.MachineClass{
							{
								TypeMeta: metav1.TypeMeta{},
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: &metav1.Time{
										Time: time.Time{},
									},
									Finalizers: []string{MCMFinalizerName},
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
								},
								ProviderSpec: runtime.RawExtension{},
								SecretRef:    &v1.SecretReference{},
								Provider:     "",
							},
						},
						machines: []*v1alpha1.Machine{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.MachineSpec{
									Class: v1alpha1.ClassSpec{
										Name: "DifferentClassName",
										Kind: machineutils.MachineClassKind,
									},
									ProviderID:           "",
									NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
									MachineConfiguration: &v1alpha1.MachineConfiguration{},
								},
								Status: v1alpha1.MachineStatus{},
							},
						},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.MachineClass{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Finalizers: []string{},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							ProviderSpec: runtime.RawExtension{},
							SecretRef:    &v1.SecretReference{},
							Provider:     "",
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class deletion succeeds as no finalizer exists",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.MachineClass{
							{
								TypeMeta: metav1.TypeMeta{},
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineClassName,
									Namespace: TestNamespace,
								},
								ProviderSpec: runtime.RawExtension{},
								SecretRef:    &v1.SecretReference{},
								Provider:     "",
							},
						},
						machines: []*v1alpha1.Machine{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.MachineSpec{
									Class: v1alpha1.ClassSpec{
										Name: "DifferentClassName",
										Kind: machineutils.MachineClassKind,
									},
									ProviderID:           "",
									NodeTemplateSpec:     v1alpha1.NodeTemplateSpec{},
									MachineConfiguration: &v1alpha1.MachineConfiguration{},
								},
								Status: v1alpha1.MachineStatus{},
							},
						},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.MachineClass{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								Name:      TestMachineClassName,
								Namespace: TestNamespace,
							},
							ProviderSpec: runtime.RawExtension{},
							SecretRef:    &v1.SecretReference{},
							Provider:     "",
						},
						err: nil,
					},
				},
			),
		)
	})

})
