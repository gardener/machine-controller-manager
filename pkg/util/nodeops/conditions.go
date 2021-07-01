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

// Package nodeops is used to provide the node functionalities
package nodeops

import (
	"context"
	"fmt"

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

// getNodeCondition returns a condition matching the type from the node's status
func getNodeCondition(node *v1.Node, conditionType v1.NodeConditionType) *v1.NodeCondition {
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
	return getNodeCondition(node, conditionType), nil
}

// AddOrUpdateConditionsOnNode adds a condition to the node's status
func AddOrUpdateConditionsOnNode(ctx context.Context, c clientset.Interface, nodeName string, condition v1.NodeCondition) error {
	firstTry := true
	return clientretry.RetryOnConflict(Backoff, func() error {
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
		return UpdateNodeConditions(ctx, c, nodeName, oldNode, newNode)
	})
}

// UpdateNodeConditions is for updating the node conditions from oldNode to the newNode
// using the nodes Update() method
func UpdateNodeConditions(ctx context.Context, c clientset.Interface, nodeName string, oldNode *v1.Node, newNode *v1.Node) error {
	newNodeClone := oldNode.DeepCopy()
	newNodeClone.Status.Conditions = newNode.Status.Conditions

	_, err := c.CoreV1().Nodes().UpdateStatus(ctx, newNodeClone, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create update conditions for node %q: %v", nodeName, err)
	}

	return nil
}
