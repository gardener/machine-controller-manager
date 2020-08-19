package conditions

import (
	v1 "k8s.io/api/core/v1"
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

// GetNodeCondition returns a condition matching the type from the node's status
func GetNodeCondition(node *v1.Node, conditionType v1.NodeConditionType) *v1.NodeCondition {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType {
			return &cond
		}
	}
	return nil
}
