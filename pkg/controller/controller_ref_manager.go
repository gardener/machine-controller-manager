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

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
	"fmt"
	"sync"

	"github.com/golang/glog"
	"github.com/gardener/node-controller-manager/pkg/apis/node/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"


	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type BaseControllerRefManager struct {
	Controller metav1.Object
	Selector   labels.Selector

	canAdoptErr  error
	canAdoptOnce sync.Once
	CanAdoptFunc func() error
}

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
			// If the Instance no longer exists, ignore the error.
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
		// If the Instance no longer exists, ignore the error.
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



type InstanceControllerRefManager struct {
	BaseControllerRefManager
	controllerKind      schema.GroupVersionKind
	instanceControl     InstanceControlInterface
}

// NewInstanceControllerRefManager returns a InstanceControllerRefManager that exposes
// methods to manage the controllerRef of Instances.
//
// The CanAdopt() function can be used to perform a potentially expensive check
// (such as a live GET from the API server) prior to the first adoption.
// It will only be called (at most once) if an adoption is actually attempted.
// If CanAdopt() returns a non-nil error, all adoptions will fail.
//
// NOTE: Once CanAdopt() is called, it will not be called again by the same
//       InstanceControllerRefManager instance. Create a new instance if it makes
//       sense to check CanAdopt() again (e.g. in a different sync pass).
func NewInstanceControllerRefManager(
	instanceControl InstanceControlInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func() error,
) *InstanceControllerRefManager {
	return &InstanceControllerRefManager{
		BaseControllerRefManager: BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind: controllerKind,
		instanceControl: instanceControl,
	}
}

// ClaimInstances tries to take ownership of a list of Instances.
//
// It will reconcile the following:
//   * Adopt orphans if the selector matches.
//   * Release owned objects if the selector no longer matches.
//
// Optional: If one or more filters are specified, a Instance will only be claimed if
// all filters return true.
//
// A non-nil error is returned if some form of reconciliation was attempted and
// failed. Usually, controllers should try again later in case reconciliation
// is still needed.
//
// If the error is nil, either the reconciliation succeeded, or no
// reconciliation was necessary. The list of Instances that you now own is returned.
func (m *InstanceControllerRefManager) ClaimInstances(instances []*v1alpha1.Instance, filters ...func(*v1alpha1.Instance) bool) ([]*v1alpha1.Instance, error) {
	var claimed []*v1alpha1.Instance
	var errlist []error

	match := func(obj metav1.Object) bool {
		instance := obj.(*v1alpha1.Instance)
		// Check selector first so filters only run on potentially matching Instances.
		if instance.Status.CurrentStatus.Phase == "" {
			glog.Info("Instance not yet ready for deletion")
			return false
		}
		if !m.Selector.Matches(labels.Set(instance.Labels)) {
			return false
		}
		for _, filter := range filters {
			if !filter(instance) {
				return false
			}
		}
		return true
	}
	
	adopt := func(obj metav1.Object) error {
		return m.AdoptInstance(obj.(*v1alpha1.Instance))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseInstance(obj.(*v1alpha1.Instance))
	}
	
	for _, instance := range instances {
		ok, err := m.ClaimObject(instance, match, adopt, release)

		//glog.Info(instance.Name, " OK:", ok, " ERR:", err)

		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, instance)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

// AdoptInstance sends a patch to take control of the Instance. It returns the error if
// the patching fails.
func (m *InstanceControllerRefManager) AdoptInstance(instance *v1alpha1.Instance) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt instance %v/%v (%v): %v", instance.Namespace, instance.Name, instance.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	addControllerPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"apiVersion":"node.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), instance.UID)
	err := m.instanceControl.PatchInstance(instance.Name, []byte(addControllerPatch))
	return err
}

// ReleaseInstance sends a patch to free the Instance from the control of the controller.
// It returns the error if the patching fails. 404 and 422 errors are ignored.
func (m *InstanceControllerRefManager) ReleaseInstance(instance *v1alpha1.Instance) error {
	glog.V(2).Infof("patching instance %s_%s to remove its controllerRef to %s/%s:%s",
		instance.Namespace, instance.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	deleteOwnerRefPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"$patch":"delete", "apiVersion":"node.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), instance.UID)

	err := m.instanceControl.PatchInstance(instance.Name, []byte(deleteOwnerRefPatch))
	if err != nil {
		if errors.IsNotFound(err) {
			// If the Instance no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the Instance
			// has no owner reference, 2. the uid of the Instance doesn't
			// match, which means the Instance is deleted and then recreated.
			// In both cases, the error can be ignored.

			// TODO: If the Instance has owner references, but none of them
			// has the owner.UID, server will silently ignore the patch.
			// Investigate why.
			return nil
		}
	}
	return err
}

// InstanceSetControllerRefManager is used to manage controllerRef of InstanceSets.
// Three methods are defined on this object 1: Classify 2: AdoptInstanceSet and
// 3: ReleaseInstanceSet which are used to classify the InstanceSets into appropriate
// categories and accordingly adopt or release them. See comments on these functions
// for more details.
type InstanceSetControllerRefManager struct {
	BaseControllerRefManager
	controllerKind schema.GroupVersionKind
	isControl      ISControlInterface
}

// NewInstanceSetControllerRefManager returns a InstanceSetControllerRefManager that exposes
// methods to manage the controllerRef of InstanceSets.
//
// The CanAdopt() function can be used to perform a potentially expensive check
// (such as a live GET from the API server) prior to the first adoption.
// It will only be called (at most once) if an adoption is actually attempted.
// If CanAdopt() returns a non-nil error, all adoptions will fail.
//
// NOTE: Once CanAdopt() is called, it will not be called again by the same
//       InstanceSetControllerRefManager instance. Create a new instance if it
//       makes sense to check CanAdopt() again (e.g. in a different sync pass).
func NewInstanceSetControllerRefManager(
	isControl ISControlInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func() error,
) *InstanceSetControllerRefManager {
	return &InstanceSetControllerRefManager{
		BaseControllerRefManager: BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind: controllerKind,
		isControl:      isControl,
	}
}

// ClaimInstanceSets tries to take ownership of a list of InstanceSets.
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
// reconciliation was necessary. The list of InstanceSets that you now own is
// returned.
func (m *InstanceSetControllerRefManager) ClaimInstanceSets(sets []*v1alpha1.InstanceSet) ([]*v1alpha1.InstanceSet, error) {
	var claimed []*v1alpha1.InstanceSet
	var errlist []error

	match := func(obj metav1.Object) bool {
		instanceSet := obj.(*v1alpha1.InstanceSet)
		//return m.Selector.Matches(labels.Set(instanceSet.GetLabels()))
		return m.Selector.Matches(labels.Set(instanceSet.Labels))
	}

	adopt := func(obj metav1.Object) error {
		return m.AdoptInstanceSet(obj.(*v1alpha1.InstanceSet))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseInstanceSet(obj.(*v1alpha1.InstanceSet))
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

// AdoptInstanceSet sends a patch to take control of the InstanceSet. It returns
// the error if the patching fails.
func (m *InstanceSetControllerRefManager) AdoptInstanceSet(is *v1alpha1.InstanceSet) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt InstanceSet %v (%v): %v", is.Name, is.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	addControllerPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"apiVersion":"node.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), is.UID)
	return m.isControl.PatchInstanceSet(is.Namespace, is.Name, []byte(addControllerPatch))
}

// ReleaseInstanceSet sends a patch to free the InstanceSet from the control of the InstanceDeployment controller.
// It returns the error if the patching fails. 404 and 422 errors are ignored.
func (m *InstanceSetControllerRefManager) ReleaseInstanceSet(instanceSet *v1alpha1.InstanceSet) error {
	glog.V(2).Infof("patching InstanceSet %s_%s to remove its controllerRef to %s:%s",
		instanceSet.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	//deleteOwnerRefPatch := fmt.Sprintf(`{"metadata":{"ownerReferences":[{"$patch":"delete","uid":"%s"}],"uid":"%s"}}`, m.Controller.GetUID(), instanceSet.UID)
	deleteOwnerRefPatch := fmt.Sprintf(
		`{"metadata":{"ownerReferences":[{"$patch":"delete", "apiVersion":"node.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"uid":"%s"}}`,
		m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), instanceSet.UID)	
	err := m.isControl.PatchInstanceSet(instanceSet.Namespace, instanceSet.Name, []byte(deleteOwnerRefPatch))
	if err != nil {
		if errors.IsNotFound(err) {
			// If the InstanceSet no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the InstanceSet
			// has no owner reference, 2. the uid of the InstanceSet doesn't
			// match, which means the InstanceSet is deleted and then recreated.
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
