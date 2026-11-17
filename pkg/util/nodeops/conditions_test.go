// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeops

import (
	"context"

	machinev1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var _ = Describe("#PatchNodeConditions", func() {
	type testCase struct {
		existingConditions []corev1.NodeCondition
		updatedConditions  []corev1.NodeCondition
		expectPatch        bool
		expectConditions   map[corev1.NodeConditionType]corev1.ConditionStatus
	}
	DescribeTable("patches node status conditions",
		func(tc testCase) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status:     corev1.NodeStatus{Conditions: tc.existingConditions},
			}
			c := k8sfake.NewSimpleClientset(node)

			patched := false
			c.Fake.PrependReactor("patch", "nodes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
				patched = true
				return false, nil, nil
			})

			result, err := PatchNodeConditions(context.TODO(), c, node, tc.updatedConditions)
			Expect(err).To(BeNil())
			Expect(patched).To(Equal(tc.expectPatch))
			if !tc.expectPatch {
				Expect(result).To(Equal(node))
			}

			updatedNode, getErr := c.CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
			Expect(getErr).To(BeNil())
			for condType, condStatus := range tc.expectConditions {
				cond := GetCondition(updatedNode, condType)
				Expect(cond).NotTo(BeNil())
				Expect(cond.Status).To(Equal(condStatus))
			}
		},
		Entry("adds a new condition when absent", testCase{
			existingConditions: []corev1.NodeCondition{},
			updatedConditions:  []corev1.NodeCondition{{Type: machinev1.NodeDrained, Status: corev1.ConditionTrue, Reason: machinev1.DrainCompleted, LastHeartbeatTime: metav1.Now(), LastTransitionTime: metav1.Now()}},
			expectPatch:        true,
			expectConditions:   map[corev1.NodeConditionType]corev1.ConditionStatus{machinev1.NodeDrained: corev1.ConditionTrue},
		}),
		Entry("updates an existing condition and leaves others untouched", testCase{
			existingConditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady"},
				{Type: machinev1.NodeDrained, Status: corev1.ConditionFalse, Reason: machinev1.DrainStarted},
			},
			updatedConditions: []corev1.NodeCondition{{Type: machinev1.NodeDrained, Status: corev1.ConditionTrue, Reason: machinev1.DrainCompleted, LastHeartbeatTime: metav1.Now(), LastTransitionTime: metav1.Now()}},
			expectPatch:       true,
			expectConditions:  map[corev1.NodeConditionType]corev1.ConditionStatus{machinev1.NodeDrained: corev1.ConditionTrue, corev1.NodeReady: corev1.ConditionTrue},
		}),
		Entry("makes no API call when the condition already matches", testCase{
			existingConditions: []corev1.NodeCondition{{Type: machinev1.NodeDrained, Status: corev1.ConditionFalse, Reason: machinev1.DrainStarted, Message: "Node drain is in progress."}},
			updatedConditions:  []corev1.NodeCondition{{Type: machinev1.NodeDrained, Status: corev1.ConditionFalse, Reason: machinev1.DrainStarted, Message: "Node drain is in progress."}},
			expectPatch:        false,
			expectConditions:   map[corev1.NodeConditionType]corev1.ConditionStatus{machinev1.NodeDrained: corev1.ConditionFalse},
		}),
	)
})

var _ = Describe("#RemoveNodeConditions", func() {
	type testCase struct {
		existingConditions []corev1.NodeCondition
		typesToRemove      []corev1.NodeConditionType
		expectPatch        bool
		expectRemoved      []corev1.NodeConditionType
		expectPresent      []corev1.NodeConditionType
	}
	DescribeTable("removes node status conditions by type",
		func(tc testCase) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status:     corev1.NodeStatus{Conditions: tc.existingConditions},
			}
			c := k8sfake.NewSimpleClientset(node)

			patched := false
			c.Fake.PrependReactor("patch", "nodes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
				patched = true
				return false, nil, nil
			})

			result, err := RemoveNodeConditions(context.TODO(), c, node, tc.typesToRemove)
			Expect(err).To(BeNil())
			Expect(patched).To(Equal(tc.expectPatch))
			if !tc.expectPatch {
				Expect(result).To(Equal(node))
			}

			updatedNode, getErr := c.CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
			Expect(getErr).To(BeNil())
			for _, condType := range tc.expectRemoved {
				Expect(GetCondition(updatedNode, condType)).To(BeNil())
			}
			for _, condType := range tc.expectPresent {
				Expect(GetCondition(updatedNode, condType)).NotTo(BeNil())
			}
		},
		Entry("removes only the listed types and leaves others untouched", testCase{
			existingConditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady"},
				{Type: machinev1.NodeDrained, Status: corev1.ConditionTrue, Reason: machinev1.DrainCompleted},
				{Type: machinev1.NodePreserved, Status: corev1.ConditionTrue, Reason: machinev1.PreservedNowByUser},
			},
			typesToRemove: []corev1.NodeConditionType{machinev1.NodeDrained, machinev1.NodePreserved},
			expectPatch:   true,
			expectRemoved: []corev1.NodeConditionType{machinev1.NodeDrained, machinev1.NodePreserved},
			expectPresent: []corev1.NodeConditionType{corev1.NodeReady},
		}),
		Entry("makes no API call when none of the listed types are present", testCase{
			existingConditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady"}},
			typesToRemove:      []corev1.NodeConditionType{machinev1.NodeDrained, machinev1.NodePreserved},
			expectPatch:        false,
			expectPresent:      []corev1.NodeConditionType{corev1.NodeReady},
		}),
	)
})

var _ = Describe("#SetNodeCondition", func() {
	type testCase struct {
		conditions   []corev1.NodeCondition
		condition    corev1.NodeCondition
		expectLen    int
		expectStatus corev1.ConditionStatus
	}
	DescribeTable("sets a condition in a slice by type",
		func(tc testCase) {
			result := SetNodeCondition(tc.conditions, tc.condition)
			Expect(result).To(HaveLen(tc.expectLen))
			cond := GetCondition(&corev1.Node{Status: corev1.NodeStatus{Conditions: result}}, tc.condition.Type)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(tc.expectStatus))
		},
		Entry("appends when the type is absent", testCase{
			conditions:   []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			condition:    corev1.NodeCondition{Type: machinev1.NodeDrained, Status: corev1.ConditionFalse},
			expectLen:    2,
			expectStatus: corev1.ConditionFalse,
		}),
		Entry("replaces when the type is present", testCase{
			conditions:   []corev1.NodeCondition{{Type: machinev1.NodeDrained, Status: corev1.ConditionFalse, Reason: machinev1.DrainStarted}},
			condition:    corev1.NodeCondition{Type: machinev1.NodeDrained, Status: corev1.ConditionTrue, Reason: machinev1.DrainCompleted},
			expectLen:    1,
			expectStatus: corev1.ConditionTrue,
		}),
	)
})
