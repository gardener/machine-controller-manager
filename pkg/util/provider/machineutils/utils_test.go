// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machineutils

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

var _ = Describe("machineutils", func() {
	Describe("#GetPreserveStateInfo", func() {
		type setup struct {
			machineAnnotations map[string]string
			preserveExpiryTime *metav1.Time
			node               *corev1.Node
		}
		type expect struct {
			machineAnnotated      bool
			machineValue          string
			nodeAnnotated         bool
			nodeValue             string
			lastAppliedNodeValue  string
			preserveExpiryTimeSet bool
		}
		type testCase struct {
			setup  setup
			expect expect
		}

		DescribeTable("GetPreserveStateInfo scenarios",
			func(tc testCase) {
				machine := &v1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "m1",
						Annotations: tc.setup.machineAnnotations,
					},
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							PreserveExpiryTime: tc.setup.preserveExpiryTime,
						},
					},
				}

				info := GetPreserveStateInfo(tc.setup.node, machine)
				Expect(info.MachineAnnotated).To(Equal(tc.expect.machineAnnotated))
				Expect(info.MachineValue).To(Equal(tc.expect.machineValue))
				Expect(info.NodeAnnotated).To(Equal(tc.expect.nodeAnnotated))
				Expect(info.NodeValue).To(Equal(tc.expect.nodeValue))
				Expect(info.LastAppliedNodeValue).To(Equal(tc.expect.lastAppliedNodeValue))
				Expect(info.PreserveExpiryTimeSet).To(Equal(tc.expect.preserveExpiryTimeSet))
			},
			Entry("machine has no annotations and node is nil", testCase{
				setup:  setup{},
				expect: expect{},
			}),
			Entry("machine has preserve annotation and node is not annotated", testCase{
				setup: setup{
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
					node: &corev1.Node{},
				},
				expect: expect{
					machineAnnotated: true,
					machineValue:     PreserveMachineAnnotationValueNow,
				},
			}),
			Entry("machine has last-applied node preserve value annotation but no preserve annotations on node or machine", testCase{
				setup: setup{
					machineAnnotations: map[string]string{
						LastAppliedNodePreserveValueAnnotationKey: PreserveMachineAnnotationValueNow,
					},
					node: &corev1.Node{},
				},
				expect: expect{
					lastAppliedNodeValue: PreserveMachineAnnotationValueNow,
				},
			}),
			Entry("machine has a non-nil preserveExpiryTime", testCase{
				setup: setup{
					preserveExpiryTime: &metav1.Time{Time: metav1.Now().Add(1 * time.Hour)},
					node:               &corev1.Node{},
				},
				expect: expect{
					preserveExpiryTimeSet: true,
				},
			}),
			Entry("node has preserve annotation", testCase{
				setup: setup{
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								PreserveMachineAnnotationKey: PreserveMachineAnnotationValueWhenFailed,
							},
						},
					},
				},
				expect: expect{
					nodeAnnotated: true,
					nodeValue:     PreserveMachineAnnotationValueWhenFailed,
				},
			}),
			Entry("nil node results in no node annotation state", testCase{
				setup: setup{
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey: PreserveMachineAnnotationValueNow,
					},
					node: nil,
				},
				expect: expect{
					machineAnnotated: true,
					machineValue:     PreserveMachineAnnotationValueNow,
				},
			}),
			Entry("both machine and node have preserve annotations", testCase{
				setup: setup{
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              PreserveMachineAnnotationValueNow,
						LastAppliedNodePreserveValueAnnotationKey: PreserveMachineAnnotationValueWhenFailed,
					},
					node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								PreserveMachineAnnotationKey: PreserveMachineAnnotationValueWhenFailed,
							},
						},
					},
				},
				expect: expect{
					machineAnnotated:     true,
					machineValue:         PreserveMachineAnnotationValueNow,
					lastAppliedNodeValue: PreserveMachineAnnotationValueWhenFailed,
					nodeAnnotated:        true,
					nodeValue:            PreserveMachineAnnotationValueWhenFailed,
				},
			}),
		)
	})

	Describe("#GetEffectivePreservationAnnotations", func() {
		type setup struct {
			nodeAnnotationValue string
			machineAnnotations  map[string]string
		}
		type expect struct {
			effectivePreserveValue string
			machineAnnotations     map[string]string
		}

		type testCase struct {
			setup  setup
			expect expect
		}

		DescribeTable("GetEffectivePreservationAnnotations scenarios",
			func(tc testCase) {
				info := &PreserveStateInfo{
					NodeValue:            tc.setup.nodeAnnotationValue,
					MachineValue:         tc.setup.machineAnnotations[PreserveMachineAnnotationKey],
					LastAppliedNodeValue: tc.setup.machineAnnotations[LastAppliedNodePreserveValueAnnotationKey],
				}
				preserveValue := GetEffectivePreservationAnnotations(info, true)
				Expect(preserveValue).To(Equal(tc.expect.effectivePreserveValue))
			},
			Entry("when node is not annotated and laNodeAnnotationValue is empty, should return machine's annotation value and empty string", testCase{
				setup: setup{
					nodeAnnotationValue: "",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "A",
						LastAppliedNodePreserveValueAnnotationKey: "",
					},
				},
				expect: expect{
					effectivePreserveValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "A",
						LastAppliedNodePreserveValueAnnotationKey: "",
					},
				},
			}),
			Entry("when neither node nor machine is not annotated and laNodeAnnotationValue is empty, should return two empty strings", testCase{
				setup: setup{
					nodeAnnotationValue: "",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "",
					},
				},
				expect: expect{
					effectivePreserveValue: "",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "",
					},
				},
			}),
			Entry("when neither node nor machine is annotated and laNodeAnnotationValue is \"A\", should return two empty strings", testCase{
				setup: setup{
					nodeAnnotationValue: "",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "A",
					},
				},
				expect: expect{
					effectivePreserveValue: "",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "",
					},
				},
			}),
			Entry("when node is annotated, laNodeAnnotationValue is empty, and machine is not annotated, should return node's annotation value as effective value and last applied value", testCase{
				setup: setup{
					nodeAnnotationValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "",
					},
				},
				expect: expect{
					effectivePreserveValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "A",
					},
				},
			}),
			Entry("when node is annotated, laNodeAnnotationValue is empty, and machine is annotated differently, should return node's annotation value as effective value and last applied value", testCase{
				setup: setup{
					nodeAnnotationValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "B",
						LastAppliedNodePreserveValueAnnotationKey: "",
					},
				},
				expect: expect{
					effectivePreserveValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "A",
					},
				},
			}),
			Entry("when node, machine annotation values and laNodeAnnotationValue are the same, should return node's annotation value as effective value and last applied value", testCase{
				setup: setup{
					nodeAnnotationValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "A",
						LastAppliedNodePreserveValueAnnotationKey: "A",
					},
				},
				expect: expect{
					effectivePreserveValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "A",
					},
				},
			}),
			Entry("when node, machine annotation values are the same and laNodeAnnotationValue differs, should return node's annotation value as effective value and last applied value", testCase{
				setup: setup{
					nodeAnnotationValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "A",
						LastAppliedNodePreserveValueAnnotationKey: "B",
					},
				},
				expect: expect{
					effectivePreserveValue: "A",
					machineAnnotations: map[string]string{
						PreserveMachineAnnotationKey:              "",
						LastAppliedNodePreserveValueAnnotationKey: "A",
					},
				},
			}),
		)
	})

	Describe("#IsPreservationRequested", func() {
		DescribeTable("IsPreservationRequested scenarios",
			func(value string, expected bool) {
				Expect(IsPreservationRequested(value)).To(Equal(expected))
			},
			Entry("preserve=now is a positive preserve value", PreserveMachineAnnotationValueNow, true),
			Entry("preserve=when-failed is a positive preserve value", PreserveMachineAnnotationValueWhenFailed, true),
			Entry("preserve=auto-preserved is a positive preserve value", PreserveMachineAnnotationValueAutoPreserved, true),
			Entry("preserve=false is not a positive preserve value", PreserveMachineAnnotationValueFalse, false),
			Entry("empty value is not a positive preserve value", "", false),
			Entry("unrecognized value is not a positive preserve value", "some-invalid-value", false),
		)
	})

	Describe("#HasMachinePreservationExpired", func() {
		DescribeTable("HasMachinePreservationExpired scenarios",
			func(preserveExpiryTime *metav1.Time, expected bool) {
				machine := &v1alpha1.Machine{
					Status: v1alpha1.MachineStatus{
						CurrentStatus: v1alpha1.CurrentStatus{
							PreserveExpiryTime: preserveExpiryTime,
						},
					},
				}
				Expect(HasMachinePreservationExpired(machine)).To(Equal(expected))
			},
			Entry("nil preserveExpiryTime has not expired", nil, false),
			Entry("preserveExpiryTime in the past has expired", &metav1.Time{Time: metav1.Now().Add(-1 * time.Hour)}, true),
			Entry("preserveExpiryTime in the future has not expired", &metav1.Time{Time: metav1.Now().Add(1 * time.Hour)}, false),
		)
	})
})
