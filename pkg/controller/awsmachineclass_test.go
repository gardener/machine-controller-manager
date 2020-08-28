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

var (
	awsMachineClassSpec = v1alpha1.AWSMachineClassSpec{
		AMI:    "test-ami",
		Region: "test-region",
		BlockDevices: []v1alpha1.AWSBlockDeviceMappingSpec{
			{
				DeviceName: "test-device",
				Ebs: v1alpha1.AWSEbsBlockDeviceSpec{
					DeleteOnTermination: new(bool),
					Encrypted:           false,
					Iops:                100,
					KmsKeyID:            new(string),
					SnapshotID:          new(string),
					VolumeSize:          50,
					VolumeType:          "gp2",
				},
				NoDevice:    "test-no",
				VirtualName: "test-vname",
			},
		},
		EbsOptimized: false,
		IAM: v1alpha1.AWSIAMProfileSpec{
			ARN:  "test-arn",
			Name: "test-iam",
		},
		MachineType: "m4.xlarge",
		KeyName:     "test-key",
		Monitoring:  false,
		NetworkInterfaces: []v1alpha1.AWSNetworkInterfaceSpec{
			{
				AssociatePublicIPAddress: new(bool),
				DeleteOnTermination:      new(bool),
				Description:              new(string),
				SecurityGroupIDs:         []string{"sg-0324245"},
				SubnetID:                 "test-subnet",
			},
		},
		Tags: map[string]string{
			"kubernetes.io/cluster/name": "1",
			"kubernetes.io/role/node":    "1",
		},
		SpotPrice: new(string),
		SecretRef: &v1.SecretReference{
			Name:      "test-secret",
			Namespace: TestNamespace,
		},
	}
)

var _ = Describe("machineclass", func() {
	Describe("#reconcileClusterAWSMachineClass", func() {
		type setup struct {
			machineClasses      []*v1alpha1.AWSMachineClass
			machines            []*v1alpha1.Machine
			fakeResourceActions *customfake.ResourceActions
		}
		type action struct {
			fakeDriver       *driver.FakeDriver
			machineClassName string
		}
		type expect struct {
			machineClass *v1alpha1.AWSMachineClass
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
				machineClass, err := controller.controlMachineClient.AWSMachineClasses(TestNamespace).Get(action.machineClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if data.setup.fakeResourceActions != nil {
					trackers.TargetCore.SetFakeResourceActions(data.setup.fakeResourceActions, math.MaxInt32)
				}

				err = controller.reconcileClusterAWSMachineClass(machineClass)
				if data.expect.err == nil {
					Expect(err).To(Not(HaveOccurred()))
				} else {
					Expect(err).To(Equal(data.expect.err))
				}

				machineClass, err = controller.controlMachineClient.AWSMachineClasses(TestNamespace).Get(action.machineClassName, metav1.GetOptions{})
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
						machineClasses: []*v1alpha1.AWSMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      TestMachineClassName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec:     awsMachineClassSpec,
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
										Kind: AWSMachineClassKind,
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
						machineClass: &v1alpha1.AWSMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     awsMachineClassSpec,
						},
						err: nil,
					},
				},
			),
			Entry(
				"Finalizer exists so do nothing",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.AWSMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									Finalizers: []string{DeleteFinalizerName},
									Name:       TestMachineClassName,
									Namespace:  TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec:     awsMachineClassSpec,
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
										Kind: AWSMachineClassKind,
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
						machineClass: &v1alpha1.AWSMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     awsMachineClassSpec,
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class deletion failure due to machines referring to it",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.AWSMachineClass{
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
								Spec:     awsMachineClassSpec,
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
										Kind: AWSMachineClassKind,
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
						machineClass: &v1alpha1.AWSMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     awsMachineClassSpec,
						},
						err: fmt.Errorf("Retry as machine objects are still referring the machineclass"),
					},
				},
			),
			Entry(
				"Class deletion succeeds with removal of finalizer",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.AWSMachineClass{
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
								Spec:     awsMachineClassSpec,
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
						machineClass: &v1alpha1.AWSMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Finalizers: []string{},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     awsMachineClassSpec,
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class deletion succeeds as no finalizer exists",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.AWSMachineClass{
							{
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: &metav1.Time{
										Time: time.Time{},
									},
									Name:      TestMachineClassName,
									Namespace: TestNamespace,
								},
								TypeMeta: metav1.TypeMeta{},
								Spec:     awsMachineClassSpec,
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
						machineClass: &v1alpha1.AWSMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Name:      TestMachineClassName,
								Namespace: TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     awsMachineClassSpec,
						},
						err: nil,
					},
				},
			),
			Entry(
				"Class migration has completed, set deletion timestamp",
				&data{
					setup: setup{
						machineClasses: []*v1alpha1.AWSMachineClass{
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
								Spec:     awsMachineClassSpec,
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
						machineClass: &v1alpha1.AWSMachineClass{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: &metav1.Time{
									Time: time.Time{},
								},
								Finalizers: []string{DeleteFinalizerName},
								Name:       TestMachineClassName,
								Namespace:  TestNamespace,
							},
							TypeMeta: metav1.TypeMeta{},
							Spec:     awsMachineClassSpec,
						},
						err:     fmt.Errorf("Retry deletion as deletion timestamp is now set"),
						deleted: true,
					},
				},
			),
		)
	})
})
