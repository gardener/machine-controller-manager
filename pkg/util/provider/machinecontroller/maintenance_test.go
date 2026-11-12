// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func TestMaintenanceIsActiveOnNode(t *testing.T) {
	maintenance := v1.NodeCondition{Type: machinev1alpha1.NodeMaintenanceInProgress, Status: v1.ConditionTrue}
	ready := v1.NodeCondition{Type: v1.NodeReady, Status: v1.ConditionTrue}
	notReady := v1.NodeCondition{Type: v1.NodeReady, Status: v1.ConditionFalse}

	cases := []struct {
		name       string
		conditions []v1.NodeCondition
		nilNode    bool
		expected   bool
	}{
		{name: "maintenance on a Ready node", conditions: []v1.NodeCondition{maintenance, ready}, expected: true},
		{name: "maintenance on a NotReady node", conditions: []v1.NodeCondition{maintenance, notReady}, expected: true},
		{name: "maintenance without a Ready condition", conditions: []v1.NodeCondition{maintenance}, expected: true},
		{name: "maintenance ended", conditions: []v1.NodeCondition{{Type: machinev1alpha1.NodeMaintenanceInProgress, Status: v1.ConditionFalse}, notReady}, expected: false},
		{name: "maintenance unknown", conditions: []v1.NodeCondition{{Type: machinev1alpha1.NodeMaintenanceInProgress, Status: v1.ConditionUnknown}}, expected: false},
		{name: "no maintenance condition", conditions: []v1.NodeCondition{notReady}, expected: false},
		{name: "no conditions", conditions: nil, expected: false},
		{name: "no node", nilNode: true, expected: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var node *v1.Node
			if !tc.nilNode {
				node = &v1.Node{Status: v1.NodeStatus{Conditions: tc.conditions}}
			}
			if got := maintenanceIsActiveOnNode(node); got != tc.expected {
				t.Fatalf("maintenanceIsActiveOnNode() = %v, want %v", got, tc.expected)
			}
		})
	}
}
