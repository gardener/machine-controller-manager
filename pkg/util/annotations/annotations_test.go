/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

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
package annotations

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

})
