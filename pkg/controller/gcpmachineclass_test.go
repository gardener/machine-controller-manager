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
	TestMachineName      = "test-machine"
	TestMachineClassName = "test-mc"
	TestNamespace        = "test-ns"
	TestSecret           = "test-secret"
)

var _ = Describe("machineclass", func() {
	Describe("#reconcileClusterGCPMachineClass", func() {
		type setup struct {
			machineClasses      []*v1alpha1.GCPMachineClass
			machines            []*v1alpha1.Machine
			fakeResourceActions *customfake.ResourceActions
		}
		type action struct {
			fakeDriver       *driver.FakeDriver
			machineClassName string
		}
		type expect struct {
			machineClass *v1alpha1.GCPMachineClass
			err          error
			deleted      bool
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

				controller, trackers := createController(stop, TestNamespace, machineObjects, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				action := data.action
				machineClass, err := controller.controlMachineClient.GCPMachineClasses(TestNamespace).Get(action.machineClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if data.setup.fakeResourceActions != nil {
					trackers.TargetCore.SetFakeResourceActions(data.setup.fakeResourceActions, math.MaxInt32)
				}

				err = controller.reconcileClusterGCPMachineClass(machineClass)
				if data.expect.err == nil {
					Expect(err).To(Not(HaveOccurred()))
				} else {
					Expect(err).To(Equal(data.expect.err))
				}

				machineClass, err = controller.controlMachineClient.GCPMachineClasses(TestNamespace).Get(action.machineClassName, metav1.GetOptions{})
				if data.expect.deleted {
					Expect(err).To((HaveOccurred()))
				} else {
					Expect(err).To(Not(HaveOccurred()))
					Expect(data.expect.machineClass).To(Equal(machineClass))
				}
			},
			Entry(
				"Add finalizer to a machine class",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.GCPMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineClassName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.GCPMachineClassSpec{
									CanIpForward:       false,
									DeletionProtection: false,
									Description:        new(string),
									Disks: []*v1alpha1.GCPDisk{
										{
											AutoDelete: new(bool),
											Boot:       true,
											SizeGb:     50,
											Type:       "pd-ssd",
											Image:      "test-image",
										},
									},
									Labels:      map[string]string{},
									MachineType: "test-type",
									Metadata:    []*v1alpha1.GCPMetadata{},
									NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
										{
											DisableExternalIP: false,
											Network:           "test-network",
											Subnetwork:        "test-subnetwork",
										},
									},
									Scheduling: v1alpha1.GCPScheduling{
										AutomaticRestart:  false,
										OnHostMaintenance: "MIGRATE",
										Preemptible:       false,
									},
									SecretRef: &v1.SecretReference{
										Name:      TestSecret,
										Namespace: TestNamespace,
									},
									ServiceAccounts: []v1alpha1.GCPServiceAccount{
										{
											Email:  "test@test.com",
											Scopes: []string{"test-scope"},
										},
									},
									Tags: []string{
										"kubernetes-io-cluster-test",
										"kubernetes-io-role-node",
									},
									Region: "test-region",
									Zone:   "test-zone",
								},
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
										Kind: GCPMachineClassKind,
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
						machineClass: &v1alpha1.GCPMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec: v1alpha1.GCPMachineClassSpec{
								CanIpForward:       false,
								DeletionProtection: false,
								Description:        new(string),
								Disks: []*v1alpha1.GCPDisk{
									{
										AutoDelete: new(bool),
										Boot:       true,
										SizeGb:     50,
										Type:       "pd-ssd",
										Image:      "test-image",
									},
								},
								Labels:      map[string]string{},
								MachineType: "test-type",
								Metadata:    []*v1alpha1.GCPMetadata{},
								NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
									{
										DisableExternalIP: false,
										Network:           "test-network",
										Subnetwork:        "test-subnetwork",
									},
								},
								Scheduling: v1alpha1.GCPScheduling{
									AutomaticRestart:  false,
									OnHostMaintenance: "MIGRATE",
									Preemptible:       false,
								},
								SecretRef: &v1.SecretReference{
									Name:      TestSecret,
									Namespace: TestNamespace,
								},
								ServiceAccounts: []v1alpha1.GCPServiceAccount{
									{
										Email:  "test@test.com",
										Scopes: []string{"test-scope"},
									},
								},
								Tags: []string{
									"kubernetes-io-cluster-test",
									"kubernetes-io-role-node",
								},
								Region: "test-region",
								Zone:   "test-zone",
							},
						},
						err: nil,
					},
				},
			),
			Entry(
				"Finalizer exists so do nothing",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.GCPMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									Finalizers: []string{DeleteFinalizerName},
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.GCPMachineClassSpec{
									CanIpForward:       false,
									DeletionProtection: false,
									Description:        new(string),
									Disks: []*v1alpha1.GCPDisk{
										{
											AutoDelete: new(bool),
											Boot:       true,
											SizeGb:     50,
											Type:       "pd-ssd",
											Image:      "test-image",
										},
									},
									Labels:      map[string]string{},
									MachineType: "test-type",
									Metadata:    []*v1alpha1.GCPMetadata{},
									NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
										{
											DisableExternalIP: false,
											Network:           "test-network",
											Subnetwork:        "test-subnetwork",
										},
									},
									Scheduling: v1alpha1.GCPScheduling{
										AutomaticRestart:  false,
										OnHostMaintenance: "MIGRATE",
										Preemptible:       false,
									},
									SecretRef: &v1.SecretReference{
										Name:      TestSecret,
										Namespace: TestNamespace,
									},
									ServiceAccounts: []v1alpha1.GCPServiceAccount{
										{
											Email:  "test@test.com",
											Scopes: []string{"test-scope"},
										},
									},
									Tags: []string{
										"kubernetes-io-cluster-test",
										"kubernetes-io-role-node",
									},
									Region: "test-region",
									Zone:   "test-zone",
								},
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
										Kind: GCPMachineClassKind,
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
						machineClass: &v1alpha1.GCPMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec: v1alpha1.GCPMachineClassSpec{
								CanIpForward:       false,
								DeletionProtection: false,
								Description:        new(string),
								Disks: []*v1alpha1.GCPDisk{
									{
										AutoDelete: new(bool),
										Boot:       true,
										SizeGb:     50,
										Type:       "pd-ssd",
										Image:      "test-image",
									},
								},
								Labels:      map[string]string{},
								MachineType: "test-type",
								Metadata:    []*v1alpha1.GCPMetadata{},
								NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
									{
										DisableExternalIP: false,
										Network:           "test-network",
										Subnetwork:        "test-subnetwork",
									},
								},
								Scheduling: v1alpha1.GCPScheduling{
									AutomaticRestart:  false,
									OnHostMaintenance: "MIGRATE",
									Preemptible:       false,
								},
								SecretRef: &v1.SecretReference{
									Name:      TestSecret,
									Namespace: TestNamespace,
								},
								ServiceAccounts: []v1alpha1.GCPServiceAccount{
									{
										Email:  "test@test.com",
										Scopes: []string{"test-scope"},
									},
								},
								Tags: []string{
									"kubernetes-io-cluster-test",
									"kubernetes-io-role-node",
								},
								Region: "test-region",
								Zone:   "test-zone",
							},
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class deletion failure due to machines referring to it",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.GCPMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: &metav1.Time{
										Time: time.Time{},
									},
									Finalizers: []string{DeleteFinalizerName},
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.GCPMachineClassSpec{
									CanIpForward:       false,
									DeletionProtection: false,
									Description:        new(string),
									Disks: []*v1alpha1.GCPDisk{
										{
											AutoDelete: new(bool),
											Boot:       true,
											SizeGb:     50,
											Type:       "pd-ssd",
											Image:      "test-image",
										},
									},
									Labels:      map[string]string{},
									MachineType: "test-type",
									Metadata:    []*v1alpha1.GCPMetadata{},
									NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
										{
											DisableExternalIP: false,
											Network:           "test-network",
											Subnetwork:        "test-subnetwork",
										},
									},
									Scheduling: v1alpha1.GCPScheduling{
										AutomaticRestart:  false,
										OnHostMaintenance: "MIGRATE",
										Preemptible:       false,
									},
									SecretRef: &v1.SecretReference{
										Name:      TestSecret,
										Namespace: TestNamespace,
									},
									ServiceAccounts: []v1alpha1.GCPServiceAccount{
										{
											Email:  "test@test.com",
											Scopes: []string{"test-scope"},
										},
									},
									Tags: []string{
										"kubernetes-io-cluster-test",
										"kubernetes-io-role-node",
									},
									Region: "test-region",
									Zone:   "test-zone",
								},
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
										Kind: GCPMachineClassKind,
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
						machineClass: &v1alpha1.GCPMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec: v1alpha1.GCPMachineClassSpec{
								CanIpForward:       false,
								DeletionProtection: false,
								Description:        new(string),
								Disks: []*v1alpha1.GCPDisk{
									{
										AutoDelete: new(bool),
										Boot:       true,
										SizeGb:     50,
										Type:       "pd-ssd",
										Image:      "test-image",
									},
								},
								Labels:      map[string]string{},
								MachineType: "test-type",
								Metadata:    []*v1alpha1.GCPMetadata{},
								NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
									{
										DisableExternalIP: false,
										Network:           "test-network",
										Subnetwork:        "test-subnetwork",
									},
								},
								Scheduling: v1alpha1.GCPScheduling{
									AutomaticRestart:  false,
									OnHostMaintenance: "MIGRATE",
									Preemptible:       false,
								},
								SecretRef: &v1.SecretReference{
									Name:      TestSecret,
									Namespace: TestNamespace,
								},
								ServiceAccounts: []v1alpha1.GCPServiceAccount{
									{
										Email:  "test@test.com",
										Scopes: []string{"test-scope"},
									},
								},
								Tags: []string{
									"kubernetes-io-cluster-test",
									"kubernetes-io-role-node",
								},
								Region: "test-region",
								Zone:   "test-zone",
							},
						},
						err: fmt.Errorf("Retry as machine objects are still referring the machineclass"),
					},
				},
			),
			Entry(
				"Class deletion succeeds with removal of finalizer",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.GCPMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: &metav1.Time{
										Time: time.Time{},
									},
									Finalizers: []string{DeleteFinalizerName},
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.GCPMachineClassSpec{
									CanIpForward:       false,
									DeletionProtection: false,
									Description:        new(string),
									Disks: []*v1alpha1.GCPDisk{
										{
											AutoDelete: new(bool),
											Boot:       true,
											SizeGb:     50,
											Type:       "pd-ssd",
											Image:      "test-image",
										},
									},
									Labels:      map[string]string{},
									MachineType: "test-type",
									Metadata:    []*v1alpha1.GCPMetadata{},
									NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
										{
											DisableExternalIP: false,
											Network:           "test-network",
											Subnetwork:        "test-subnetwork",
										},
									},
									Scheduling: v1alpha1.GCPScheduling{
										AutomaticRestart:  false,
										OnHostMaintenance: "MIGRATE",
										Preemptible:       false,
									},
									SecretRef: &v1.SecretReference{
										Name:      TestSecret,
										Namespace: TestNamespace,
									},
									ServiceAccounts: []v1alpha1.GCPServiceAccount{
										{
											Email:  "test@test.com",
											Scopes: []string{"test-scope"},
										},
									},
									Tags: []string{
										"kubernetes-io-cluster-test",
										"kubernetes-io-role-node",
									},
									Region: "test-region",
									Zone:   "test-zone",
								},
							},
						},
						machines:            []*v1alpha1.Machine{},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.GCPMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Finalizers: []string{},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec: v1alpha1.GCPMachineClassSpec{
								CanIpForward:       false,
								DeletionProtection: false,
								Description:        new(string),
								Disks: []*v1alpha1.GCPDisk{
									{
										AutoDelete: new(bool),
										Boot:       true,
										SizeGb:     50,
										Type:       "pd-ssd",
										Image:      "test-image",
									},
								},
								Labels:      map[string]string{},
								MachineType: "test-type",
								Metadata:    []*v1alpha1.GCPMetadata{},
								NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
									{
										DisableExternalIP: false,
										Network:           "test-network",
										Subnetwork:        "test-subnetwork",
									},
								},
								Scheduling: v1alpha1.GCPScheduling{
									AutomaticRestart:  false,
									OnHostMaintenance: "MIGRATE",
									Preemptible:       false,
								},
								SecretRef: &v1.SecretReference{
									Name:      TestSecret,
									Namespace: TestNamespace,
								},
								ServiceAccounts: []v1alpha1.GCPServiceAccount{
									{
										Email:  "test@test.com",
										Scopes: []string{"test-scope"},
									},
								},
								Tags: []string{
									"kubernetes-io-cluster-test",
									"kubernetes-io-role-node",
								},
								Region: "test-region",
								Zone:   "test-zone",
							},
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class deletion succeeds as no finalizer exists",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.GCPMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: &metav1.Time{
										Time: time.Time{},
									},
									Name:      TestMachineClassName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.GCPMachineClassSpec{
									CanIpForward:       false,
									DeletionProtection: false,
									Description:        new(string),
									Disks: []*v1alpha1.GCPDisk{
										{
											AutoDelete: new(bool),
											Boot:       true,
											SizeGb:     50,
											Type:       "pd-ssd",
											Image:      "test-image",
										},
									},
									Labels:      map[string]string{},
									MachineType: "test-type",
									Metadata:    []*v1alpha1.GCPMetadata{},
									NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
										{
											DisableExternalIP: false,
											Network:           "test-network",
											Subnetwork:        "test-subnetwork",
										},
									},
									Scheduling: v1alpha1.GCPScheduling{
										AutomaticRestart:  false,
										OnHostMaintenance: "MIGRATE",
										Preemptible:       false,
									},
									SecretRef: &v1.SecretReference{
										Name:      TestSecret,
										Namespace: TestNamespace,
									},
									ServiceAccounts: []v1alpha1.GCPServiceAccount{
										{
											Email:  "test@test.com",
											Scopes: []string{"test-scope"},
										},
									},
									Tags: []string{
										"kubernetes-io-cluster-test",
										"kubernetes-io-role-node",
									},
									Region: "test-region",
									Zone:   "test-zone",
								},
							},
						},
						machines:            []*v1alpha1.Machine{},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.GCPMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Name:      TestMachineClassName,
								Namespace: TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec: v1alpha1.GCPMachineClassSpec{
								CanIpForward:       false,
								DeletionProtection: false,
								Description:        new(string),
								Disks: []*v1alpha1.GCPDisk{
									{
										AutoDelete: new(bool),
										Boot:       true,
										SizeGb:     50,
										Type:       "pd-ssd",
										Image:      "test-image",
									},
								},
								Labels:      map[string]string{},
								MachineType: "test-type",
								Metadata:    []*v1alpha1.GCPMetadata{},
								NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
									{
										DisableExternalIP: false,
										Network:           "test-network",
										Subnetwork:        "test-subnetwork",
									},
								},
								Scheduling: v1alpha1.GCPScheduling{
									AutomaticRestart:  false,
									OnHostMaintenance: "MIGRATE",
									Preemptible:       false,
								},
								SecretRef: &v1.SecretReference{
									Name:      TestSecret,
									Namespace: TestNamespace,
								},
								ServiceAccounts: []v1alpha1.GCPServiceAccount{
									{
										Email:  "test@test.com",
										Scopes: []string{"test-scope"},
									},
								},
								Tags: []string{
									"kubernetes-io-cluster-test",
									"kubernetes-io-role-node",
								},
								Region: "test-region",
								Zone:   "test-zone",
							},
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class migration has completed, set deletion timestamp",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.GCPMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									Annotations: map[string]string{
										machineutils.MigratedMachineClass: "present",
									},
									Finalizers: []string{DeleteFinalizerName},
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec: v1alpha1.GCPMachineClassSpec{
									CanIpForward:       false,
									DeletionProtection: false,
									Description:        new(string),
									Disks: []*v1alpha1.GCPDisk{
										{
											AutoDelete: new(bool),
											Boot:       true,
											SizeGb:     50,
											Type:       "pd-ssd",
											Image:      "test-image",
										},
									},
									Labels:      map[string]string{},
									MachineType: "test-type",
									Metadata:    []*v1alpha1.GCPMetadata{},
									NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
										{
											DisableExternalIP: false,
											Network:           "test-network",
											Subnetwork:        "test-subnetwork",
										},
									},
									Scheduling: v1alpha1.GCPScheduling{
										AutomaticRestart:  false,
										OnHostMaintenance: "MIGRATE",
										Preemptible:       false,
									},
									SecretRef: &v1.SecretReference{
										Name:      TestSecret,
										Namespace: TestNamespace,
									},
									ServiceAccounts: []v1alpha1.GCPServiceAccount{
										{
											Email:  "test@test.com",
											Scopes: []string{"test-scope"},
										},
									},
									Tags: []string{
										"kubernetes-io-cluster-test",
										"kubernetes-io-role-node",
									},
									Region: "test-region",
									Zone:   "test-zone",
								},
							},
						},
						machines:            []*v1alpha1.Machine{},
						fakeResourceActions: &customfake.ResourceActions{},
					},
					action: action{
						fakeDriver:       &driver.FakeDriver{},
						machineClassName: TestMachineClassName,
					},
					expect: expect{
						machineClass: &v1alpha1.GCPMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec: v1alpha1.GCPMachineClassSpec{
								CanIpForward:       false,
								DeletionProtection: false,
								Description:        new(string),
								Disks: []*v1alpha1.GCPDisk{
									{
										AutoDelete: new(bool),
										Boot:       true,
										SizeGb:     50,
										Type:       "pd-ssd",
										Image:      "test-image",
									},
								},
								Labels:      map[string]string{},
								MachineType: "test-type",
								Metadata:    []*v1alpha1.GCPMetadata{},
								NetworkInterfaces: []*v1alpha1.GCPNetworkInterface{
									{
										DisableExternalIP: false,
										Network:           "test-network",
										Subnetwork:        "test-subnetwork",
									},
								},
								Scheduling: v1alpha1.GCPScheduling{
									AutomaticRestart:  false,
									OnHostMaintenance: "MIGRATE",
									Preemptible:       false,
								},
								SecretRef: &v1.SecretReference{
									Name:      TestSecret,
									Namespace: TestNamespace,
								},
								ServiceAccounts: []v1alpha1.GCPServiceAccount{
									{
										Email:  "test@test.com",
										Scopes: []string{"test-scope"},
									},
								},
								Tags: []string{
									"kubernetes-io-cluster-test",
									"kubernetes-io-role-node",
								},
								Region: "test-region",
								Zone:   "test-zone",
							},
						},
						err:     fmt.Errorf("Retry deletion as deletion timestamp is now set"),
						deleted: true,
					},
				},
			),
		)
	})
})
