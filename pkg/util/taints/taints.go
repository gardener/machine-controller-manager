/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/util/taints/taints.go

Modifications Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package taints implements utilites for working with taints
package taints

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// MODIFIED refer's to the object was modified
	MODIFIED = "modified"
	// TAINTED refer's if it was tainted
	TAINTED = "tainted"
	// UNTAINTED refer's if it was untainted
	UNTAINTED = "untainted"
)

// parseTaint parses a taint from a string. Taint must be off the format '<key>=<value>:<effect>'.
func parseTaint(st string) (v1.Taint, error) {
	var taint v1.Taint
	parts := strings.Split(st, "=")
	if len(parts) != 2 || len(parts[1]) == 0 || len(validation.IsQualifiedName(parts[0])) > 0 {
		return taint, fmt.Errorf("invalid taint spec: %v", st)
	}

	parts2 := strings.Split(parts[1], ":")

	effect := v1.TaintEffect(parts2[1])

	errs := validation.IsValidLabelValue(parts2[0])
	if len(parts2) != 2 || len(errs) != 0 {
		return taint, fmt.Errorf("invalid taint spec: %v, %s", st, strings.Join(errs, "; "))
	}

	if effect != v1.TaintEffectNoSchedule && effect != v1.TaintEffectPreferNoSchedule && effect != v1.TaintEffectNoExecute {
		return taint, fmt.Errorf("invalid taint spec: %v, unsupported taint effect", st)
	}

	taint.Key = parts[0]
	taint.Value = parts2[0]
	taint.Effect = effect

	return taint, nil
}

// NewVar wraps []api.Taint in a struct that implements flag.Value to allow taints to be
// bound to command line flags.
func NewVar(ptr *[]v1.Taint) Var {
	return Var{
		ptr: ptr,
	}
}

// Var are varibles used to tarck taints
type Var struct {
	ptr *[]v1.Taint
}

// Set sets the type for Var
func (t Var) Set(s string) error {
	sts := strings.Split(s, ",")
	var taints []v1.Taint
	for _, st := range sts {
		taint, err := parseTaint(st)
		if err != nil {
			return err
		}
		taints = append(taints, v1.Taint{Key: taint.Key, Value: taint.Value, Effect: v1.TaintEffect(taint.Effect)})
	}
	*t.ptr = taints
	return nil
}

// String gets the String for Var
func (t Var) String() string {
	if len(*t.ptr) == 0 {
		return "<nil>"
	}
	var taints []string
	for _, taint := range *t.ptr {
		taints = append(taints, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
	}
	return strings.Join(taints, ",")
}

// Type gets the type for Var
func (t Var) Type() string {
	return "[]v1.Taint"
}

// ParseTaints takes a spec which is an array and creates slices for new taints to be added, taints to be deleted.
func ParseTaints(spec []string) ([]v1.Taint, []v1.Taint, error) {
	var taints, taintsToRemove []v1.Taint
	uniqueTaints := map[v1.TaintEffect]sets.String{}

	for _, taintSpec := range spec {
		if strings.Index(taintSpec, "=") != -1 && strings.Index(taintSpec, ":") != -1 {
			newTaint, err := parseTaint(taintSpec)
			if err != nil {
				return nil, nil, err
			}
			// validate if taint is unique by <key, effect>
			if len(uniqueTaints[newTaint.Effect]) > 0 && uniqueTaints[newTaint.Effect].Has(newTaint.Key) {
				return nil, nil, fmt.Errorf("duplicated taints with the same key and effect: %v", newTaint)
			}
			// add taint to existingTaints for uniqueness check
			if len(uniqueTaints[newTaint.Effect]) == 0 {
				uniqueTaints[newTaint.Effect] = sets.String{}
			}
			uniqueTaints[newTaint.Effect].Insert(newTaint.Key)

			taints = append(taints, newTaint)
		} else if strings.HasSuffix(taintSpec, "-") {
			taintKey := taintSpec[:len(taintSpec)-1]
			var effect v1.TaintEffect
			if strings.Index(taintKey, ":") != -1 {
				parts := strings.Split(taintKey, ":")
				taintKey = parts[0]
				effect = v1.TaintEffect(parts[1])
			}
			taintsToRemove = append(taintsToRemove, v1.Taint{Key: taintKey, Effect: effect})
		} else {
			return nil, nil, fmt.Errorf("unknown taint spec: %v", taintSpec)
		}
	}
	return taints, taintsToRemove, nil
}

// ReorganizeTaints returns the updated set of taints, taking into account old taints that were not updated,
// old taints that were updated, old taints that were deleted, and new taints.
func ReorganizeTaints(node *v1.Node, overwrite bool, taintsToAdd []v1.Taint, taintsToRemove []v1.Taint) (string, []v1.Taint, error) {
	newTaints := append([]v1.Taint{}, taintsToAdd...)
	oldTaints := node.Spec.Taints
	// add taints that already existing but not updated to newTaints
	added := addTaints(oldTaints, &newTaints)
	allErrs, deleted := deleteTaints(taintsToRemove, &newTaints)
	if (added && deleted) || overwrite {
		return MODIFIED, newTaints, utilerrors.NewAggregate(allErrs)
	} else if added {
		return TAINTED, newTaints, utilerrors.NewAggregate(allErrs)
	}
	return UNTAINTED, newTaints, utilerrors.NewAggregate(allErrs)
}

// deleteTaints deletes the given taints from the node's taintlist.
func deleteTaints(taintsToRemove []v1.Taint, newTaints *[]v1.Taint) ([]error, bool) {
	allErrs := []error{}
	var removed bool
	for _, taintToRemove := range taintsToRemove {
		removed = false
		if len(taintToRemove.Effect) > 0 {
			*newTaints, removed = DeleteTaint(*newTaints, &taintToRemove)
		} else {
			*newTaints, removed = DeleteTaintsByKey(*newTaints, taintToRemove.Key)
		}
		if !removed {
			allErrs = append(allErrs, fmt.Errorf("taint %q not found", taintToRemove.ToString()))
		}
	}
	return allErrs, removed
}

// addTaints adds the newTaints list to existing ones and updates the newTaints List.
// TODO: This needs a rewrite to take only the new values instead of appended newTaints list to be consistent.
func addTaints(oldTaints []v1.Taint, newTaints *[]v1.Taint) bool {
	for _, oldTaint := range oldTaints {
		existsInNew := false
		for _, taint := range *newTaints {
			if taint.MatchTaint(&oldTaint) {
				existsInNew = true
				break
			}
		}
		if !existsInNew {
			*newTaints = append(*newTaints, oldTaint)
		}
	}
	return len(oldTaints) != len(*newTaints)
}

// CheckIfTaintsAlreadyExists checks if the node already has taints that we want to add and returns a string with taint keys.
func CheckIfTaintsAlreadyExists(oldTaints []v1.Taint, taints []v1.Taint) string {
	var existingTaintList = make([]string, 0)
	for _, taint := range taints {
		for _, oldTaint := range oldTaints {
			if taint.Key == oldTaint.Key && taint.Effect == oldTaint.Effect {
				existingTaintList = append(existingTaintList, taint.Key)
			}
		}
	}
	return strings.Join(existingTaintList, ",")
}

// DeleteTaintsByKey removes all the taints that have the same key to given taintKey
func DeleteTaintsByKey(taints []v1.Taint, taintKey string) ([]v1.Taint, bool) {
	newTaints := []v1.Taint{}
	deleted := false
	for i := range taints {
		if taintKey == taints[i].Key {
			deleted = true
			continue
		}
		newTaints = append(newTaints, taints[i])
	}
	return newTaints, deleted
}

// DeleteTaint removes all the the taints that have the same key and effect to given taintToDelete.
func DeleteTaint(taints []v1.Taint, taintToDelete *v1.Taint) ([]v1.Taint, bool) {
	newTaints := []v1.Taint{}
	deleted := false
	for i := range taints {
		if taintToDelete.MatchTaint(&taints[i]) {
			deleted = true
			continue
		}
		newTaints = append(newTaints, taints[i])
	}
	return newTaints, deleted
}

// RemoveTaint tries to remove a taint from annotations list. Returns a new copy of updated Node and true if something was updated
// false otherwise.
func RemoveTaint(node *v1.Node, taint *v1.Taint) (*v1.Node, bool, error) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints
	if len(nodeTaints) == 0 {
		return newNode, false, nil
	}

	if !TaintExists(nodeTaints, taint) {
		return newNode, false, nil
	}

	newTaints, _ := DeleteTaint(nodeTaints, taint)
	newNode.Spec.Taints = newTaints
	return newNode, true, nil
}

// AddOrUpdateTaint tries to add a taint to annotations list. Returns a new copy of updated Node and true if something was updated
// false otherwise.
func AddOrUpdateTaint(node *v1.Node, taint *v1.Taint) (*v1.Node, bool, error) {

	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints

	var newTaints []v1.Taint
	updated := false
	for i := range nodeTaints {
		if taint.MatchTaint(&nodeTaints[i]) {
			if equality.Semantic.DeepEqual(*taint, nodeTaints[i]) {
				return newNode, false, nil
			}
			newTaints = append(newTaints, *taint)
			updated = true
			continue
		}

		newTaints = append(newTaints, nodeTaints[i])
	}

	if !updated {
		newTaints = append(newTaints, *taint)
	}

	newNode.Spec.Taints = newTaints
	return newNode, true, nil
}

// TaintExists checks if the given taint exists in list of taints. Returns true if exists false otherwise.
func TaintExists(taints []v1.Taint, taintToFind *v1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

// TaintSetDiff set difference between the taints
func TaintSetDiff(t1, t2 []v1.Taint) (taintsToAdd []*v1.Taint, taintsToRemove []*v1.Taint) {
	for _, taint := range t1 {
		if !TaintExists(t2, &taint) {
			t := taint
			taintsToAdd = append(taintsToAdd, &t)
		}
	}

	for _, taint := range t2 {
		if !TaintExists(t1, &taint) {
			t := taint
			taintsToRemove = append(taintsToRemove, &t)
		}
	}

	return
}

// TaintSetFilter filter from the set of taints
func TaintSetFilter(taints []v1.Taint, fn func(*v1.Taint) bool) []v1.Taint {
	res := []v1.Taint{}

	for _, taint := range taints {
		if fn(&taint) {
			res = append(res, taint)
		}
	}

	return res
}
