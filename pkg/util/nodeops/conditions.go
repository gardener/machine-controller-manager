// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package nodeops is used to provide the node functionalities
package nodeops

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	clientset "k8s.io/client-go/kubernetes"
	clientretry "k8s.io/client-go/util/retry"
)

// CloneAndAddCondition adds condition to the conditions slice if it has changed.
func CloneAndAddCondition(conditions []v1.NodeCondition, condition v1.NodeCondition) ([]v1.NodeCondition, bool) {
	if condition.Type == "" || condition.Status == "" {
		return conditions, false
	}
	// Clone
	var newConditions []v1.NodeCondition
	updated := true

	for _, existingCondition := range conditions {
		if existingCondition.Type != condition.Type { // filter out the condition that is being updated
			newConditions = append(newConditions, existingCondition)
		} else { // condition with this type already exists
			updated = existingCondition.Status != condition.Status || existingCondition.Reason != condition.Reason || existingCondition.Message != condition.Message
			if existingCondition.Status == condition.Status && existingCondition.Reason == condition.Reason {
				// condition status and reason are  the same, keep existing transition time
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
		}
	}

	newConditions = append(newConditions, condition)
	return newConditions, updated
}

// AddOrUpdateCondition adds a condition to the condition list. Returns a new copy of updated Node
func AddOrUpdateCondition(node *v1.Node, condition v1.NodeCondition) *v1.Node {
	newNode := node.DeepCopy()
	nodeConditions := newNode.Status.Conditions
	newNode.Status.Conditions, _ = CloneAndAddCondition(nodeConditions, condition)
	return newNode
}

// GetCondition returns a condition matching the type from the node's status
func GetCondition(node *v1.Node, conditionType v1.NodeConditionType) *v1.NodeCondition {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType {
			return &cond
		}
	}
	return nil
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

// SetNodeCondition returns an array of node conditions after replacing an existing entry of the
// same type, or appending it if absent.
func SetNodeCondition(conditions []v1.NodeCondition, condition v1.NodeCondition) []v1.NodeCondition {
	for i := range conditions {
		if conditions[i].Type == condition.Type {
			conditions[i] = condition
			return conditions
		}
	}
	return append(conditions, condition)
}

// PatchNodeConditions adds or updates the given conditions on the node's status, matched by type.
// Conditions present on the node whose type is not in conditionsToChange are left unchanged. Each
// condition is written exactly as given, including its timestamps. No API server call is made when
// the conditions already match. The patch does not conflict with concurrent writes to other conditions.
func PatchNodeConditions(ctx context.Context, c clientset.Interface, node *v1.Node, conditionsToChange []v1.NodeCondition) (*v1.Node, error) {
	original := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: node.Name},
		Status:     v1.NodeStatus{Conditions: node.Status.Conditions},
	}
	base, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old node %q: %w", node.Name, err)
	}

	modified := original.DeepCopy()
	for _, updatedCondition := range conditionsToChange {
		modified.Status.Conditions = SetNodeCondition(modified.Status.Conditions, updatedCondition)
	}
	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new node %q: %w", node.Name, err)
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(base, modifiedJSON, v1.Node{})
	if err != nil {
		return nil, fmt.Errorf("failed to create status patch for node %q: %w", node.Name, err)
	}
	if string(patch) == "{}" {
		return node, nil
	}
	return c.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "status")
}

// RemoveNodeConditions deletes the conditions of the given types from the node's status. Types not
// present on the node are ignored, and no API server call is made when none of them are present.
// Conditions of other types are left unchanged, and it does not conflict with concurrent writes to them.
func RemoveNodeConditions(ctx context.Context, c clientset.Interface, node *v1.Node, conditionTypesToRemove []v1.NodeConditionType) (*v1.Node, error) {
	var conditionPatches []map[string]any
	for _, conditionType := range conditionTypesToRemove {
		if GetCondition(node, conditionType) == nil {
			continue
		}
		conditionPatches = append(conditionPatches, map[string]any{
			"type":   conditionType,
			"$patch": "delete",
		})
	}
	if len(conditionPatches) == 0 {
		return node, nil
	}

	patch, err := json.Marshal(map[string]any{
		"status": map[string]any{
			"conditions": conditionPatches,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create condition removal patch for node %q: %w", node.Name, err)
	}
	return c.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "status")
}
