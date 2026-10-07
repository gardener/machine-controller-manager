// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package nodeops is used to provide the node functionalities
package nodeops

import (
	"context"
	"fmt"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	clientretry "k8s.io/client-go/util/retry"
)

// CloneAndAddCondition adds condition to the conditions slice if
func CloneAndAddCondition(conditions []v1.NodeCondition, condition v1.NodeCondition) []v1.NodeCondition {
	if condition.Type == "" || condition.Status == "" {
		return conditions
	}
	// Clone
	var newConditions []v1.NodeCondition

	for _, existingCondition := range conditions {
		if existingCondition.Type != condition.Type { // filter out the condition that is being updated
			newConditions = append(newConditions, existingCondition)
		} else { // condition with this type already exists
			if existingCondition.Status == condition.Status && existingCondition.Reason == condition.Reason {
				// condition status and reason are  the same, keep existing transition time
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
		}
	}

	newConditions = append(newConditions, condition)
	return newConditions
}

// AddOrUpdateCondition adds a condition to the condition list. Returns a new copy of updated Node
func AddOrUpdateCondition(node *v1.Node, condition v1.NodeCondition) *v1.Node {
	newNode := node.DeepCopy()
	nodeConditions := newNode.Status.Conditions
	newNode.Status.Conditions = CloneAndAddCondition(nodeConditions, condition)
	return newNode
}

// GetCondition returns a condition matching the type from the node's status
func GetCondition(node *v1.Node, conditionType v1.NodeConditionType) *v1.NodeCondition {
	return FilterNodeConditionOfType(node.Status.Conditions, conditionType)
}

// GetNodeCondition get the nodes condition matching the specified type
func GetNodeCondition(ctx context.Context, c clientset.Interface, nodeName string, conditionType v1.NodeConditionType) (*v1.NodeCondition, error) {
	node, err := c.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return GetCondition(node, conditionType), nil
}

// AddOrUpdateConditionsOnNode adds a condition to the node's status
func AddOrUpdateConditionsOnNode(ctx context.Context, c clientset.Interface, nodeName string, condition v1.NodeCondition) (*v1.Node, error) {
	firstTry := true
	var updatedNode *v1.Node
	err := clientretry.RetryOnConflict(Backoff, func() error {
		var err error
		var oldNode *v1.Node
		// First we try getting node from the API server cache, as it's cheaper. If it fails
		// we get it from etcd to be sure to have fresh data.
		if firstTry {
			oldNode, err = c.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{ResourceVersion: "0"})
			firstTry = false
		} else {
			oldNode, err = c.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		}

		if err != nil {
			return err
		}

		var newNode *v1.Node
		oldNodeCopy := oldNode
		newNode = AddOrUpdateCondition(oldNodeCopy, condition)
		updatedNode, err = UpdateNodeConditions(ctx, c, nodeName, oldNode, newNode)
		return err
	})
	return updatedNode, err
}

// UpdateNodeConditions is for updating the node conditions from oldNode to the newNode
// using the node's UpdateStatus() method
func UpdateNodeConditions(ctx context.Context, c clientset.Interface, nodeName string, oldNode *v1.Node, newNode *v1.Node) (*v1.Node, error) {
	newNodeClone := oldNode.DeepCopy()
	newNodeClone.Status.Conditions = newNode.Status.Conditions
	updatedNode, err := c.CoreV1().Nodes().UpdateStatus(ctx, newNodeClone, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create/update conditions on node %q: %v", nodeName, err)
	}
	return updatedNode, nil
}

// FilterNodeConditionOfType filters conditions and returns the NodeCondition belonging to the given conditionType or nil if not present.
func FilterNodeConditionOfType(conditions []v1.NodeCondition, conditionType v1.NodeConditionType) *v1.NodeCondition {
	for _, c := range conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// IsPureMachineCondition returns true for condition types that are set by MCM and machine specific and not sourced from the node.
func IsPureMachineCondition(condType v1.NodeConditionType) bool {
	return condType == v1alpha1.ConditionMachineJoined
}

// NodeConditionsHaveChanged compares two node status.conditions to see if any of the statuses have changed.
// Ignores pure machine conditions (see [IsPureMachineCondition]).
func NodeConditionsHaveChanged(oldConditions []v1.NodeCondition, newConditions []v1.NodeCondition) ([]v1.NodeCondition, []v1.NodeCondition, bool) {
	var (
		oldConditionsByType      = make(map[v1.NodeConditionType]v1.NodeCondition, len(oldConditions))
		newConditionsByType      = make(map[v1.NodeConditionType]v1.NodeCondition, len(newConditions))
		addedOrUpdatedConditions = make([]v1.NodeCondition, 0, len(newConditions))
		removedConditions        = make([]v1.NodeCondition, 0, len(oldConditions))
	)

	for _, c := range oldConditions {
		if IsPureMachineCondition(c.Type) {
			continue
		}
		oldConditionsByType[c.Type] = c
	}
	for _, c := range newConditions {
		if IsPureMachineCondition(c.Type) {
			continue
		}
		newConditionsByType[c.Type] = c
	}

	// checking for any added/updated new condition
	for _, c := range newConditions {
		oldC, exists := oldConditionsByType[c.Type]
		if !exists || (oldC.Status != c.Status) || (c.Type == v1alpha1.NodeInPlaceUpdate && oldC.Reason != c.Reason) {
			addedOrUpdatedConditions = append(addedOrUpdatedConditions, c)
		}
	}

	// checking for any deleted condition
	for _, c := range oldConditions {
		if _, exists := newConditionsByType[c.Type]; !exists {
			removedConditions = append(removedConditions, c)
		}
	}

	return addedOrUpdatedConditions, removedConditions, len(addedOrUpdatedConditions) != 0 || len(removedConditions) != 0
}

// ReplacePreservingPureMachineConditions replaces existing with newConditions, re-inserting any
// pure machine conditions (see [IsPureMachineCondition]) from existing so they are not lost when newConditions
// originates from an external source (e.g. a node) that is unaware of MCM-internal condition types.
func ReplacePreservingPureMachineConditions(existing, newConditions []v1.NodeCondition) []v1.NodeCondition {
	result := newConditions
	for _, c := range existing {
		if IsPureMachineCondition(c.Type) {
			result = CloneAndAddCondition(result, c)
		}
	}
	return result
}
