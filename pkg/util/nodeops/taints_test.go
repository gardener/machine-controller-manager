// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeops

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var _ = Describe("#PatchNodeTaints", func() {
	type testCase struct {
		resourceVersion string
		existingTaints  []corev1.Taint
		newTaints       []corev1.Taint
		expectPatch     bool
		expectErr       func(error) bool
		expectKeys      []string
	}
	DescribeTable("patches node spec taints",
		func(tc testCase) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1", ResourceVersion: tc.resourceVersion},
				Spec:       corev1.NodeSpec{Taints: tc.existingTaints},
			}
			c := k8sfake.NewSimpleClientset(node)

			patched := false
			c.Fake.PrependReactor("patch", "nodes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
				patched = true
				if tc.expectErr != nil {
					return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "nodes"}, node.Name, nil)
				}
				return false, nil, nil
			})

			newNode := node.DeepCopy()
			newNode.Spec.Taints = tc.newTaints
			result, err := PatchNodeTaints(context.Background(), c, node, newNode)

			if tc.expectErr != nil {
				Expect(err).NotTo(BeNil())
				Expect(tc.expectErr(err)).To(BeTrue())
				return
			}

			Expect(err).To(BeNil())
			Expect(patched).To(Equal(tc.expectPatch))
			if !tc.expectPatch {
				Expect(result).To(Equal(node))
			}

			updatedNode, getErr := c.CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
			Expect(getErr).To(BeNil())
			keys := map[string]bool{}
			for _, taint := range updatedNode.Spec.Taints {
				keys[taint.Key] = true
			}
			for _, key := range tc.expectKeys {
				Expect(keys[key]).To(BeTrue(), "expected taint key %q to be present", key)
			}
		},
		Entry("applies a new taint and preserves existing taints", testCase{
			resourceVersion: "1",
			existingTaints:  []corev1.Taint{{Key: "existing", Effect: corev1.TaintEffectNoSchedule}},
			newTaints: []corev1.Taint{
				{Key: "existing", Effect: corev1.TaintEffectNoSchedule},
				{Key: "node.machine.sapcloud.io/preserved", Effect: corev1.TaintEffectNoSchedule},
			},
			expectPatch: true,
			expectKeys:  []string{"existing", "node.machine.sapcloud.io/preserved"},
		}),
		Entry("makes no API call when taints and resourceVersion are both identical", testCase{
			existingTaints: []corev1.Taint{{Key: "node.machine.sapcloud.io/preserved", Effect: corev1.TaintEffectNoSchedule}},
			newTaints:      []corev1.Taint{{Key: "node.machine.sapcloud.io/preserved", Effect: corev1.TaintEffectNoSchedule}},
			expectPatch:    false,
			expectKeys:     []string{"node.machine.sapcloud.io/preserved"},
		}),
		Entry("removes a taint when absent from newNode", testCase{
			resourceVersion: "1",
			existingTaints: []corev1.Taint{
				{Key: "existing", Effect: corev1.TaintEffectNoSchedule},
				{Key: "node.machine.sapcloud.io/preserved", Effect: corev1.TaintEffectNoSchedule},
			},
			newTaints:   []corev1.Taint{{Key: "existing", Effect: corev1.TaintEffectNoSchedule}},
			expectPatch: true,
			expectKeys:  []string{"existing"},
		}),
		Entry("returns conflict error when API server returns 409", testCase{
			resourceVersion: "1",
			existingTaints:  []corev1.Taint{{Key: "existing", Effect: corev1.TaintEffectNoSchedule}},
			newTaints: []corev1.Taint{
				{Key: "existing", Effect: corev1.TaintEffectNoSchedule},
				{Key: "node.machine.sapcloud.io/preserved", Effect: corev1.TaintEffectNoSchedule},
			},
			expectPatch: true,
			expectErr:   apierrors.IsConflict,
		}),
	)
})
