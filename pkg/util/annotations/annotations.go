// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package annotations implements utilites for working with annotatoins
package annotations

import (
	"slices"
	"strings"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// GetMachineNamesWithDeletionTimesTriggeredForDeletion returns the set of machine names with their marked for deletion times contained within the machineutils.TriggerDeletionByMCM annotation on the given MachineDeployment
func GetMachineNamesWithDeletionTimesTriggeredForDeletion(mcd *v1alpha1.MachineDeployment) []string {
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

// GetEffectiveMachineCreationTimeout gets the value of the annotation [v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout]
// as a [metav1.Duration] if present.
func GetEffectiveMachineCreationTimeout(object runtime.Object) (*metav1.Duration, error) {
	metaObject, err := meta.Accessor(object)
	if err != nil {
		return nil, err
	}
	effectiveMachineCreationTimeoutStr, ok := metaObject.GetAnnotations()[v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout]
	if !ok {
		return nil, nil
	}
	effectiveMachineCreationTimeout, err := time.ParseDuration(effectiveMachineCreationTimeoutStr)
	if err != nil {
		return nil, err
	}
	return &metav1.Duration{Duration: effectiveMachineCreationTimeout}, nil
}

// IsInstanceDeletionSuspended returns a deterministic human-readable message for all
// instance-deletion suspension annotations on the machine and whether any are set.
func IsInstanceDeletionSuspended(machine *v1alpha1.Machine) (string, bool) {
	suspensions := getInstanceDeletionSuspensions(machine)
	if len(suspensions) == 0 {
		return "", false
	}

	details := make([]string, 0, len(suspensions))
	for _, suspension := range suspensions {
		detail := suspension.owner
		if suspension.purpose != "" {
			detail += " for " + suspension.purpose
		}
		details = append(details, detail)
	}

	return "Instance Deletion suspended by " + strings.Join(details, ", ") + ".", true
}

type instanceDeletionSuspension struct {
	purpose string
	owner   string
}

func getInstanceDeletionSuspensions(machine *v1alpha1.Machine) []instanceDeletionSuspension {
	if machine == nil {
		return nil
	}

	var suspensions []instanceDeletionSuspension
	if owner, exists := machine.Annotations[v1alpha1.AnnotationSuspendInstanceDeletionPrefix]; exists {
		// Support the base annotation as a boolean-style hook.
		suspensions = append(suspensions, instanceDeletionSuspension{owner: owner})
	}

	prefix := v1alpha1.AnnotationSuspendInstanceDeletionPrefix + "/"
	for annotation, owner := range machine.Annotations {
		if !strings.HasPrefix(annotation, prefix) {
			continue
		}
		purpose := strings.TrimPrefix(annotation, prefix)
		suspensions = append(suspensions, instanceDeletionSuspension{purpose: purpose, owner: owner})
	}

	// Sort annotations to keep the condition message deterministic; annotation map iteration order is not stable.
	slices.SortFunc(suspensions, func(a, b instanceDeletionSuspension) int {
		if comparison := strings.Compare(a.purpose, b.purpose); comparison != 0 {
			return comparison
		}
		return strings.Compare(a.owner, b.owner)
	})
	return suspensions
}
