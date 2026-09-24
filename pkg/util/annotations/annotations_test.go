// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"fmt"
	"strings"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	terminationHookOwner          = "my-controller"
	anotherTerminationHookOwner   = "another-controller"
	terminationHookPurpose        = "my-purpose"
	anotherTerminationHookPurpose = "another-purpose"
)

var _ = Describe("annotations", func() {

	Describe("#AddOrUpdateAnnotation", func() {
		type setup struct {
			existingAnnotations map[string]string
		}
		type expect struct {
			nodeAnnotations map[string]string
			updated         bool
			err             bool
		}
		type action struct {
			toBeAppliedAnnotations map[string]string
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

				nodeObject := corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-0",
					},
				}
				nodeObject.Annotations = data.setup.existingAnnotations

				newNode, updated, err := AddOrUpdateAnnotation(
					&nodeObject,
					data.action.toBeAppliedAnnotations,
				)
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				Expect(newNode.Annotations).Should(Equal(data.expect.nodeAnnotations))
				Expect(updated).Should(Equal(data.expect.updated))
			},
			Entry("Add the given annotation", &data{
				setup: setup{
					existingAnnotations: map[string]string{
						"anno1": "anno1",
					},
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno2": "anno2",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "anno2",
					},
					updated: true,
					err:     false,
				},
			}),
			Entry("Update the given annotation", &data{
				setup: setup{
					existingAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "annoDummy",
					},
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno2": "anno2",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "anno2",
					},
					updated: true,
					err:     false,
				},
			}),
			Entry("Add annotations when there are none in node", &data{
				setup: setup{
					existingAnnotations: map[string]string{},
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno2": "anno2",
						"anno1": "anno1",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "anno2",
					},
					updated: true,
					err:     false,
				},
			}),
		)
	})
	Describe("#RemoveAnnotation", func() {
		type setup struct {
			existingAnnotations map[string]string
		}
		type expect struct {
			nodeAnnotations map[string]string
			updated         bool
			err             bool
		}
		type action struct {
			toBeAppliedAnnotations map[string]string
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

				nodeObject := corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-0",
					},
				}
				nodeObject.Annotations = data.setup.existingAnnotations

				newNode, updated, err := RemoveAnnotation(
					&nodeObject,
					data.action.toBeAppliedAnnotations,
				)
				if !data.expect.err {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(HaveOccurred())
				}

				Expect(newNode.Annotations).Should(Equal(data.expect.nodeAnnotations))
				Expect(updated).Should(Equal(data.expect.updated))
			},
			Entry("Remove the given annotation when it already exists", &data{
				setup: setup{
					existingAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "anno2",
					},
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno2": "anno2",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
					},
					updated: true,
					err:     false,
				},
			}),
			Entry("Remove the given annotation when it exists but modified value", &data{
				setup: setup{
					existingAnnotations: map[string]string{
						"anno1": "anno1",
						"anno2": "annoDummy",
					},
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno2": "anno2",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{
						"anno1": "anno1",
					},
					updated: true,
					err:     false,
				},
			}),
			Entry("When the annotation doesnt exist in the node", &data{
				setup: setup{
					existingAnnotations: map[string]string{},
				},
				action: action{
					toBeAppliedAnnotations: map[string]string{
						"anno2": "anno2",
					},
				},
				expect: expect{
					nodeAnnotations: map[string]string{},
					updated:         false,
					err:             false,
				},
			}),
		)
	})

	Describe("#GetEffectiveMachineCreationTimeout", func() {
		type setup struct {
			object any
		}
		type expect struct {
			timeout *metav1.Duration
			err     bool
		}
		type data struct {
			setup  setup
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				timeout, err := GetEffectiveMachineCreationTimeout(data.setup.object.(runtime.Object))

				if data.expect.err {
					Expect(err).To(HaveOccurred())
					return
				}

				Expect(err).NotTo(HaveOccurred())
				Expect(timeout).To(Equal(data.expect.timeout))
			},

			Entry("annotation is absent", &data{
				setup: setup{
					object: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "node-0",
							Annotations: map[string]string{},
						},
					},
				},
				expect: expect{
					timeout: nil,
					err:     false,
				},
			}),

			Entry("annotation contains a valid duration", &data{
				setup: setup{
					object: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Annotations: map[string]string{
								v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout: "15m30s",
							},
						},
					},
				},
				expect: expect{
					timeout: &metav1.Duration{
						Duration: 15*time.Minute + 30*time.Second,
					},
					err: false,
				},
			}),

			Entry("annotation contains an invalid duration", &data{
				setup: setup{
					object: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-0",
							Annotations: map[string]string{
								v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout: "definitely-not-a-duration",
							},
						},
					},
				},
				expect: expect{
					timeout: nil,
					err:     true,
				},
			}),

			Entry("object does not implement metav1.Object", &data{
				setup: setup{
					object: &corev1.NodeList{},
				},
				expect: expect{
					timeout: nil,
					err:     true,
				},
			}),
		)
	})

	Describe("#IsInstanceDeletionSuspended", func() {
		It("returns a deterministic message and true for all suspension annotations", func() {
			machine := &v1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha1.AnnotationKeySuspendInstanceDeletionPrefix + "/" + terminationHookPurpose:        terminationHookOwner,
					v1alpha1.AnnotationKeySuspendInstanceDeletionPrefix + "/" + anotherTerminationHookPurpose: anotherTerminationHookOwner,
				},
			}}

			message, suspended := IsInstanceDeletionSuspended(machine)
			Expect(suspended).To(BeTrue())
			Expect(message).To(Equal(fmt.Sprintf("Instance Deletion suspended by %s for %s, %s for %s.", anotherTerminationHookOwner, anotherTerminationHookPurpose, terminationHookOwner, terminationHookPurpose)))
		})

		It("uses unknown owner when a suspension annotation has no owner", func() {
			machine := &v1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1alpha1.AnnotationKeySuspendInstanceDeletionPrefix + "/" + anotherTerminationHookPurpose: "",
			}}}

			message, suspended := IsInstanceDeletionSuspended(machine)
			Expect(suspended).To(BeTrue())
			Expect(message).To(Equal(fmt.Sprintf("Instance Deletion suspended by unknown owner for %s.", anotherTerminationHookPurpose)))
		})

		It("limits oversized messages and reports omitted owners", func() {
			annotations := make(map[string]string)
			for i := range 100 {
				annotations[fmt.Sprintf("%s/purpose-%03d", v1alpha1.AnnotationKeySuspendInstanceDeletionPrefix, i)] = terminationHookOwner
			}
			machine := &v1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}

			message, suspended := IsInstanceDeletionSuspended(machine)
			Expect(suspended).To(BeTrue())
			Expect(message).To(HavePrefix(fmt.Sprintf("Instance Deletion suspended by %s for purpose-000", terminationHookOwner)))
			Expect(message).To(ContainSubstring("other owner(s)."))
			Expect(message).NotTo(ContainSubstring("purpose-099"))
		})

		It("reports the owner count when the first detail exceeds the message limit", func() {
			owner := strings.Repeat(terminationHookOwner, 100)
			machine := &v1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1alpha1.AnnotationKeySuspendInstanceDeletionPrefix + "/first":  owner,
				v1alpha1.AnnotationKeySuspendInstanceDeletionPrefix + "/second": anotherTerminationHookOwner,
			}}}

			message, suspended := IsInstanceDeletionSuspended(machine)
			Expect(suspended).To(BeTrue())
			Expect(message).To(Equal("Instance Deletion suspended by 2 owner(s)."))
		})

		It("returns a message and true for the standalone suspension annotation", func() {
			machine := &v1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha1.AnnotationKeySuspendInstanceDeletionPrefix: terminationHookOwner,
				},
			}}

			message, suspended := IsInstanceDeletionSuspended(machine)
			Expect(suspended).To(BeTrue())
			Expect(message).To(Equal(fmt.Sprintf("Instance Deletion suspended by %s.", terminationHookOwner)))
		})

		It("returns an empty message and false when no suspension annotation exists", func() {
			machine := &v1alpha1.Machine{}
			message, suspended := IsInstanceDeletionSuspended(machine)
			Expect(message).To(BeEmpty())
			Expect(suspended).To(BeFalse())
		})
	})

})
