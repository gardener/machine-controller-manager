// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package annotations implements utilites for working with annotatoins
package annotations

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	v1 "k8s.io/api/core/v1"
	"slices"
	"strings"
)

// AddOrUpdateAnnotation tries to add an annotation. Returns a new copy of updated Node and true if something was updated
// false otherwise.
func AddOrUpdateAnnotation(node *v1.Node, annotations map[string]string) (*v1.Node, bool, error) {

	newNode := node.DeepCopy()
	nodeAnnotations := newNode.Annotations
	updated := false

	if nodeAnnotations == nil {
		nodeAnnotations = make(map[string]string)
	}

	for annotationKey, annotationValue := range annotations {
		if nodeAnnotationValue, exists := nodeAnnotations[annotationKey]; exists {
			if nodeAnnotationValue == annotationValue {
				// Annotation is already available on the node.
				continue
			}
		}
		// If the given annotation doesnt exist in the nodeAnnotation, we anyways update.
		nodeAnnotations[annotationKey] = annotationValue
		updated = true
	}

	newNode.Annotations = nodeAnnotations

	return newNode, updated, nil
}

// RemoveAnnotation tries to remove an annotation from annotations list. Returns a new copy of updated Node and true if something was updated
// false otherwise.
func RemoveAnnotation(node *v1.Node, annotations map[string]string) (*v1.Node, bool, error) {
	newNode := node.DeepCopy()
	nodeAnnotations := newNode.Annotations
	deleted := false

	// Short circuit if annotation doesnt exist for limiting API calls.
	if node == nil || node.Annotations == nil || annotations == nil {
		return newNode, deleted, nil
	}

	newAnnotations, deleted := DeleteAnnotation(nodeAnnotations, annotations)
	newNode.Annotations = newAnnotations
	return newNode, deleted, nil
}

// DeleteAnnotation removes the annotation with annotationKey.
func DeleteAnnotation(nodeAnnotations map[string]string, annotations map[string]string) (map[string]string, bool) {
	newAnnotations := make(map[string]string)
	deleted := false
	for key, value := range nodeAnnotations {
		if _, exists := annotations[key]; exists {
			deleted = true
			continue
		}
		newAnnotations[key] = value
	}
	return newAnnotations, deleted
}

// GetMachineNamesTriggeredForDeletion returns the set of machine names contained within the machineutils.TriggerDeletionByMCM annotation on the given MachineDeployment
func GetMachineNamesTriggeredForDeletion(mcd *v1alpha1.MachineDeployment) []string {
	if mcd.Annotations == nil || mcd.Annotations[machineutils.TriggerDeletionByMCM] == "" {
		return nil
	}
	return strings.Split(mcd.Annotations[machineutils.TriggerDeletionByMCM], ",")
}

// CreateMachinesTriggeredForDeletionAnnotValue constructs the annotation value for machineutils.TriggerDeletionByMCM from the given machine names.
func CreateMachinesTriggeredForDeletionAnnotValue(machineNames []string) string {
	slices.Sort(machineNames)
	return strings.Join(machineNames, ",")
}
