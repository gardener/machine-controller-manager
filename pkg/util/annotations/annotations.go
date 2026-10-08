// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package annotations implements utilites for working with annotatoins
package annotations

import (
	"slices"
	"strconv"
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

// GetMachineEffectiveCreationTimeout gets the value of the annotation [v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout]
// as a [metav1.Duration] if present.
func GetMachineEffectiveCreationTimeout(object runtime.Object) (creationTimeout metav1.Duration, err error) {
	durationStr, ok, err := getAnnotationValue(object, v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout)
	if err != nil || !ok {
		return
	}
	creationTimeout.Duration, err = time.ParseDuration(durationStr)
	return
}

// GetMachineEffectiveCreationTimeoutLastAppliedAt gets the value of the annotation [v1alpha1.AnnotationKeyMachineEffectiveCreationTimeoutLastAppliedAt]
// as a [metav1.Time] if present using the layout [time.RFC3339].
func GetMachineEffectiveCreationTimeoutLastAppliedAt(object runtime.Object) (appliedAt metav1.Time, err error) {
	lastAppliedAtStr, ok, err := getAnnotationValue(object, v1alpha1.AnnotationKeyMachineEffectiveCreationTimeoutLastAppliedAt)
	if err != nil || !ok {
		return
	}
	return parseLastAppliedAt(lastAppliedAtStr)
}

// GetMachineReplaceCycleCount gets the value of the annotation [v1alpha1.AnnotationKeyMachineReplaceCycleCount]
// as an int32 if present.
func GetMachineReplaceCycleCount(object runtime.Object) (cycleCount int32, err error) {
	cycleCountStr, ok, err := getAnnotationValue(object, v1alpha1.AnnotationKeyMachineReplaceCycleCount)
	if err != nil || !ok {
		return
	}
	val, err := strconv.ParseInt(cycleCountStr, 10, 32)
	if err != nil {
		return
	}
	cycleCount = int32(val)
	return
}

// GetMachineReplaceCycleCountLastAppliedAt gets the value of the annotation [v1alpha1.AnnotationKeyMachineReplaceCycleCountLastAppliedAt]
// as a [metav1.Time] if present using the layout [time.RFC3339]. Returns nil if no annotation value is present.
func GetMachineReplaceCycleCountLastAppliedAt(object runtime.Object) (appliedAt metav1.Time, err error) {
	replacementLastAppliedAtStr, ok, err := getAnnotationValue(object, v1alpha1.AnnotationKeyMachineReplaceCycleCountLastAppliedAt)
	if err != nil || !ok {
		return
	}
	return parseLastAppliedAt(replacementLastAppliedAtStr)
}

func parseLastAppliedAt(strVal string) (val metav1.Time, err error) {
	v, err := time.Parse(time.RFC3339, strVal)
	if err != nil {
		return
	}
	val = metav1.NewTime(v)
	return
}

func getAnnotationValue(object runtime.Object, annKey string) (annValue string, ok bool, err error) {
	metaObject, err := meta.Accessor(object)
	if err != nil {
		return
	}
	annValue, ok = metaObject.GetAnnotations()[annKey]
	return
}
