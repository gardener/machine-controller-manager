package controller

import (
	"context"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("machine_safety_util", func() {
	Describe("#updateNodeWithAnnotations", func() {

		type setup struct{}
		type action struct {
			node        *corev1.Node
			annotations map[string]string
		}
		type expect struct {
			node *corev1.Node
			err  error
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

				c, trackers := createController(stop, testNamespace, nil, nil, nil, nil)
				defer trackers.Stop()
				waitForCacheSync(stop, c)

				testNode := data.action.node
				expectedNode := data.expect.node

				err := c.updateNodeWithAnnotation(context.TODO(), testNode, data.action.annotations)

				waitForCacheSync(stop, c)

				if data.expect.err == nil {
					Expect(err).To(BeNil())
				} else {
					Expect(err).To(Equal(data.expect.err))
				}
				if expectedNode != nil {
					Expect(testNode.Annotations).Should(Equal(expectedNode.Annotations))
				}
			},

			Entry("when annotations are already present", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1": "value1",
							},
						},
					},
					annotations: map[string]string{
						machineutils.NotManagedByMCM: "1",
					},
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								"anno1":                      "value1",
								machineutils.NotManagedByMCM: "1",
							},
						},
					},
					err: apierrors.NewNotFound(corev1.Resource("nodes"), "test-node-0"),
				},
			}),
			Entry("when no annotations are present already", &data{
				setup: setup{},
				action: action{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
						},
					},
					annotations: map[string]string{
						machineutils.NotManagedByMCM: "1",
					},
				},
				expect: expect{
					node: &corev1.Node{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Node",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-0",
							Annotations: map[string]string{
								machineutils.NotManagedByMCM: "1",
							},
						},
					},
					err: apierrors.NewNotFound(corev1.Resource("nodes"), "test-node-0"),
				},
			}),
		)

	})

})
