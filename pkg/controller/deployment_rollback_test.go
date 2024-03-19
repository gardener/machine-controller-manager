// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("deployment_rollback", func() {

	Describe("#removeTaintNodesBackingMachineSet", func() {
		type setup struct {
			nodes       []*corev1.Node
			machineSets []*machinev1.MachineSet
			machines    []*machinev1.Machine
		}
		type expect struct {
			nodes       []*corev1.Node
			machineSets []*machinev1.MachineSet
			err         bool
		}
		type data struct {
			setup  setup
			action *machinev1.MachineSet
			expect expect
		}
		objMeta := &metav1.ObjectMeta{
			Namespace: testNamespace,
		}
		machineSets := newMachineSets(
			1,
			&machinev1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "machineset-0",
					Labels: map[string]string{
						"key": "value",
					},
				},
				Spec: machinev1.MachineSpec{
					Class: machinev1.ClassSpec{
						Kind: "OpenStackMachineClass",
						Name: "test-machine-class",
					},
				},
			},
			3,
			500,
			nil,
			nil,
			map[string]string{
				PreferNoScheduleKey: "True",
			},
			nil,
		)

		DescribeTable("##table",
			func(data *data) {
				stop := make(chan struct{})
				defer close(stop)

				controlMachineObjects := []runtime.Object{}
				for _, o := range data.setup.machineSets {
					controlMachineObjects = append(controlMachineObjects, o)
				}
				for _, o := range data.setup.machines {
					controlMachineObjects = append(controlMachineObjects, o)
				}

				targetCoreObjects := []runtime.Object{}
				for _, o := range data.setup.nodes {
					targetCoreObjects = append(targetCoreObjects, o)
				}

				controller, trackers := createController(stop, testNamespace, controlMachineObjects, nil, targetCoreObjects)
				defer trackers.Stop()
				waitForCacheSync(stop, controller)

				err := controller.removeTaintNodesBackingMachineSet(
					context.TODO(),
					data.action,
					&v1.Taint{
						Key:    PreferNoScheduleKey,
						Value:  "True",
						Effect: "PreferNoSchedule",
					})

				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				for _, expectedMachineSet := range data.expect.machineSets {
					actualMachineSet, err := controller.controlMachineClient.MachineSets(testNamespace).Get(context.TODO(), expectedMachineSet.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualMachineSet.Annotations).Should(Equal(expectedMachineSet.Annotations))
				}

				for _, expectedNode := range data.expect.nodes {
					actualNode, err := controller.targetCoreClient.CoreV1().Nodes().Get(context.TODO(), expectedNode.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualNode.Spec.Taints).Should(ConsistOf(expectedNode.Spec.Taints))
				}

			},
			Entry("removed taints on nodes backing machineSet", &data{
				setup: setup{
					machineSets: machineSets,
					machines: newMachinesFromMachineSet(
						1,
						machineSets[0],
						&machinev1.MachineStatus{},
						nil,
						map[string]string{machinev1.NodeLabelKey: "node-0"},
					),
					nodes: newNodes(
						1,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{
								{
									Key:    PreferNoScheduleKey,
									Value:  "True",
									Effect: "PreferNoSchedule",
								},
							},
						},
						nil,
					),
				},
				action: newMachineSets(
					1,
					&machinev1.MachineTemplateSpec{
						ObjectMeta: *newObjectMeta(objMeta, 0),
						Spec: machinev1.MachineSpec{
							Class: machinev1.ClassSpec{
								Kind: "OpenStackMachineClass",
								Name: "test-machine-class",
							},
						},
					},
					3,
					500,
					nil,
					nil,
					map[string]string{
						PreferNoScheduleKey: "True",
					},
					nil,
				)[0],
				expect: expect{
					machineSets: newMachineSets(
						1,
						&machinev1.MachineTemplateSpec{
							ObjectMeta: *newObjectMeta(objMeta, 0),
							Spec: machinev1.MachineSpec{
								Class: machinev1.ClassSpec{
									Kind: "OpenStackMachineClass",
									Name: "test-machine-class",
								},
							},
						},
						3,
						500,
						nil,
						nil,
						map[string]string{},
						nil,
					),
					nodes: newNodes(
						1,
						&corev1.NodeSpec{
							Taints: []corev1.Taint{},
						},
						nil,
					),
					err: false,
				},
			}),
		)
	})
})
