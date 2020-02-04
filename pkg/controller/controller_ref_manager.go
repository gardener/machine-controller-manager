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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/controller_ref_manager.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"sync"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/klog"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// BaseControllerRefManager is the struct is used to identify the base controller of the object
type BaseControllerRefManager struct {
	Controller metav1.Object
	Selector   labels.Selector

	canAdoptErr  error
	canAdoptOnce sync.Once
	CanAdoptFunc func() error
}

// CanAdopt is used to identify if the object can be adopted by the controller
func (m *BaseControllerRefManager) CanAdopt() error {
	m.canAdoptOnce.Do(func() {
		if m.CanAdoptFunc != nil {
			m.canAdoptErr = m.CanAdoptFunc()
		}
	})
	return m.canAdoptErr
}

// ClaimObject tries to take ownership of an object for this controller.
//
// It will reconcile the following:
//   * Adopt orphans if the match function returns true.
//   * Release owned objects if the match function returns false.
//
// A non-nil error is returned if some form of reconciliation was attempted and
// failed. Usually, controllers should try again later in case reconciliation
// is still needed.
//
// If the error is nil, either the reconciliation succeeded, or no
// reconciliation was necessary. The returned boolean indicates whether you now
// own the object.
//
// No reconciliation will be attempted if the controller is being deleted.
func (m *BaseControllerRefManager) ClaimObject(obj metav1.Object, match func(metav1.Object) bool, adopt, release func(metav1.Object) error) (bool, error) {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef != nil {
		if controllerRef.UID != m.Controller.GetUID() {
			// Owned by someone else. Ignore.
			return false, nil
		}
		if match(obj) {
			// We already own it and the selector matches.
			// Return true (successfully claimed) before checking deletion timestamp.
			// We're still allowed to claim things we already own while being deleted
			// because doing so requires taking no actions.
			return true, nil
		}
		// Owned by us but selector doesn't match.
		// Try to release, unless we're being deleted.
		if m.Controller.GetDeletionTimestamp() != nil {
			return false, nil
		}
		if err := release(obj); err != nil {
			// If the Machine no longer exists, ignore the error.
			if errors.IsNotFound(err) {
				return false, nil
			}
			// Either someone else released it, or there was a transient error.
			// The controller should requeue and try again if it's still stale.
			return false, err
		}
		// Successfully released.
		return false, nil
	}

	// It's an orphan.
	if m.Controller.GetDeletionTimestamp() != nil || !match(obj) {
		// Ignore if we're being deleted or selector doesn't match.
		return false, nil
	}

	if obj.GetDeletionTimestamp() != nil {
		// Ignore if the object is being deleted
		return false, nil
	}

	// Selector matches. Try to adopt.
	if err := adopt(obj); err != nil {
		// If the Machine no longer exists, ignore the error.
		if errors.IsNotFound(err) {
			return false, nil
		}

		// Either someone else claimed it first, or there was a transient error.
		// The controller should requeue and try again if it's still orphaned.
		return false, err
	}
	// Successfully adopted.
	return true, nil
}

// MachineControllerRefManager is the struct used to manage the machines
type MachineControllerRefManager struct {
	BaseControllerRefManager
	controllerKind schema.GroupVersionKind
	machineControl MachineControlInterface
}

// NewMachineControllerRefManager returns a MachineControllerRefManager that exposes
// methods to manage the controllerRef of Machines.
//
// The CanAdopt() function can be used to perform a potentially expensive check
// (such as a live GET from the API server) prior to the first adoption.
// It will only be called (at most once) if an adoption is actually attempted.
// If CanAdopt() returns a non-nil error, all adoptions will fail.
//
// NOTE: Once CanAdopt() is called, it will not be called again by the same
//       MachineControllerRefManager machine. Create a new machine if it makes
//       sense to check CanAdopt() again (e.g. in a different sync pass).
func NewMachineControllerRefManager(
	machineControl MachineControlInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func() error,
) *MachineControllerRefManager {
	return &MachineControllerRefManager{
		BaseControllerRefManager: BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind: controllerKind,
		machineControl: machineControl,
	}
}

// ClaimMachines tries to take ownership of a list of Machines.
//
// It will reconcile the following:
//   * Adopt orphans if the selector matches.
//   * Release owned objects if the selector no longer matches.
//
// Optional: If one or more filters are specified, a Machine will only be claimed if
// all filters return true.
//
// A non-nil error is returned if some form of reconciliation was attempted and
// failed. Usually, controllers should try again later in case reconciliation
// is still needed.
//
// If the error is nil, either the reconciliation succeeded, or no
// reconciliation was necessary. The list of Machines that you now own is returned.
func (m *MachineControllerRefManager) ClaimMachines(machines []*v1alpha1.Machine, filters ...func(*v1alpha1.Machine) bool) ([]*v1alpha1.Machine, error) {
	var claimed []*v1alpha1.Machine
	var errlist []error

	match := func(obj metav1.Object) bool {
		machine := obj.(*v1alpha1.Machine)
		// Check selector first so filters only run on potentially matching Machines.
		if !m.Selector.Matches(labels.Set(machine.Labels)) {
			return false
		}
		for _, filter := range filters {
			if !filter(machine) {
				return false
			}
		}
		return true
	}

	adopt := func(obj metav1.Object) error {
		return m.AdoptMachine(obj.(*v1alpha1.Machine))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseMachine(obj.(*v1alpha1.Machine))
	}

	for _, machine := range machines {
		ok, err := m.ClaimObject(machine, match, adopt, release)

		//klog.Info(machine.Name, " OK:", ok, " ERR:", err)

		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, machine)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

// AdoptMachine sends a patch to take control of the Machine. It returns the error if
// the patching fails.
func (m *MachineControllerRefManager) AdoptMachine(machine *v1alpha1.Machine) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt machine %v/%v (%v): %v", machine.Namespace, machine.Name, machine.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	addControllerPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"apiVersion":"machine.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), machine.UID)
	err := m.machineControl.PatchMachine(machine.Namespace, machine.Name, []byte(addControllerPatch))
	return err
}

// ReleaseMachine sends a patch to free the Machine from the control of the controller.
// It returns the error if the patching fails. 404 and 422 errors are ignored.
func (m *MachineControllerRefManager) ReleaseMachine(machine *v1alpha1.Machine) error {
	klog.V(4).Infof("patching machine %s_%s to remove its controllerRef to %s/%s:%s",
		machine.Namespace, machine.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	deleteOwnerRefPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"$patch":"delete", "apiVersion":"machine.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), machine.UID)

	err := m.machineControl.PatchMachine(machine.Namespace, machine.Name, []byte(deleteOwnerRefPatch))
	if err != nil {
		if errors.IsNotFound(err) {
			// If the Machine no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the Machine
			// has no owner reference, 2. the uid of the Machine doesn't
			// match, which means the Machine is deleted and then recreated.
			// In both cases, the error can be ignored.

			// TODO: If the Machine has owner references, but none of them
			// has the owner.UID, server will silently ignore the patch.
			// Investigate why.
			return nil
		}
	}
	return err
}

// MachineSetControllerRefManager is used to manage controllerRef of MachineSets.
// Three methods are defined on this object 1: Classify 2: AdoptMachineSet and
// 3: ReleaseMachineSet which are used to classify the MachineSets into appropriate
// categories and accordingly adopt or release them. See comments on these functions
// for more details.
type MachineSetControllerRefManager struct {
	BaseControllerRefManager
	controllerKind    schema.GroupVersionKind
	machineSetControl MachineSetControlInterface
}

// NewMachineSetControllerRefManager returns a MachineSetControllerRefManager that exposes
// methods to manage the controllerRef of MachineSets.
//
// The CanAdopt() function can be used to perform a potentially expensive check
// (such as a live GET from the API server) prior to the first adoption.
// It will only be called (at most once) if an adoption is actually attempted.
// If CanAdopt() returns a non-nil error, all adoptions will fail.
//
// NOTE: Once CanAdopt() is called, it will not be called again by the same
//       MachineSetControllerRefManager machine. Create a new machine if it
//       makes sense to check CanAdopt() again (e.g. in a different sync pass).
func NewMachineSetControllerRefManager(
	machineSetControl MachineSetControlInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func() error,
) *MachineSetControllerRefManager {
	return &MachineSetControllerRefManager{
		BaseControllerRefManager: BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind:    controllerKind,
		machineSetControl: machineSetControl,
	}
}

// ClaimMachineSets tries to take ownership of a list of MachineSets.
//
// It will reconcile the following:
//   * Adopt orphans if the selector matches.
//   * Release owned objects if the selector no longer matches.
//
// A non-nil error is returned if some form of reconciliation was attempted and
// failed. Usually, controllers should try again later in case reconciliation
// is still needed.
//
// If the error is nil, either the reconciliation succeeded, or no
// reconciliation was necessary. The list of MachineSets that you now own is
// returned.
func (m *MachineSetControllerRefManager) ClaimMachineSets(sets []*v1alpha1.MachineSet) ([]*v1alpha1.MachineSet, error) {
	var claimed []*v1alpha1.MachineSet
	var errlist []error

	match := func(obj metav1.Object) bool {
		machineSet := obj.(*v1alpha1.MachineSet)
		//return m.Selector.Matches(labels.Set(machineSet.GetLabels()))
		return m.Selector.Matches(labels.Set(machineSet.Labels))
	}

	adopt := func(obj metav1.Object) error {
		return m.AdoptMachineSet(obj.(*v1alpha1.MachineSet))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseMachineSet(obj.(*v1alpha1.MachineSet))
	}

	for _, is := range sets {
		ok, err := m.ClaimObject(is, match, adopt, release)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, is)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

// AdoptMachineSet sends a patch to take control of the MachineSet. It returns
// the error if the patching fails.
func (m *MachineSetControllerRefManager) AdoptMachineSet(is *v1alpha1.MachineSet) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt MachineSet %v (%v): %v", is.Name, is.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	addControllerPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"apiVersion":"machine.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), is.UID)
	return m.machineSetControl.PatchMachineSet(is.Namespace, is.Name, []byte(addControllerPatch))
}

// ReleaseMachineSet sends a patch to free the MachineSet from the control of the MachineDeployment controller.
// It returns the error if the patching fails. 404 and 422 errors are ignored.
func (m *MachineSetControllerRefManager) ReleaseMachineSet(machineSet *v1alpha1.MachineSet) error {
	klog.V(4).Infof("patching MachineSet %s_%s to remove its controllerRef to %s:%s",
		machineSet.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	//deleteOwnerRefPatch := fmt.Sprintf(`{"metadata":{"ownerReferences":[{"$patch":"delete","uid":"%s"}],"uid":"%s"}}`, m.Controller.GetUID(), machineSet.UID)
	deleteOwnerRefPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"$patch":"delete", "apiVersion":"machine.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), machineSet.UID)
	err := m.machineSetControl.PatchMachineSet(machineSet.Namespace, machineSet.Name, []byte(deleteOwnerRefPatch))
	if err != nil {
		if errors.IsNotFound(err) {
			// If the MachineSet no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the MachineSet
			// has no owner reference, 2. the uid of the MachineSet doesn't
			// match, which means the MachineSet is deleted and then recreated.
			// In both cases, the error can be ignored.
			return nil
		}
	}
	return err
}

// RecheckDeletionTimestamp returns a CanAdopt() function to recheck deletion.
//
// The CanAdopt() function calls getObject() to fetch the latest value,
// and denies adoption attempts if that object has a non-nil DeletionTimestamp.
func RecheckDeletionTimestamp(getObject func() (metav1.Object, error)) func() error {
	return func() error {
		obj, err := getObject()
		if err != nil {
			return fmt.Errorf("can't recheck DeletionTimestamp: %v", err)
		}
		if obj.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", obj.GetNamespace(), obj.GetName(), obj.GetDeletionTimestamp())
		}
		return nil
	}
}
