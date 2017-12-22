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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/util/deployment_util.go

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	intstrutil "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/integer"
	labelsutil "k8s.io/kubernetes/pkg/util/labels"
	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"
	v1alpha1client "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/clientset/typed/node/v1alpha1"

	v1alpha1listers "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/listers/node/v1alpha1"
)

// DeploymentListerExpansion allows custom methods to be added to
// DeploymentLister.
type InstanceDeploymentListerExpansion interface {
	GetInstanceDeploymentsForInstanceSet(is *v1alpha1.InstanceSet) ([]*v1alpha1.InstanceDeployment, error)
}

// DeploymentNamespaceListerExpansion allows custom methods to be added to
// DeploymentNamespaceLister.
type InstanceDeploymentNamespaceListerExpansion interface{}

// GetDeploymentsForInstanceSet returns a list of Deployments that potentially
// match a InstanceSet. Only the one specified in the InstanceSet's ControllerRef
// will actually manage it.
// Returns an error only if no matching Deployments are found.
func (c *controller) GetInstanceDeploymentsForInstanceSet(is *v1alpha1.InstanceSet) ([]*v1alpha1.InstanceDeployment, error) {
	if len(is.Labels) == 0 {
		return nil, fmt.Errorf("no deployments found for InstanceSet %v because it has no labels", is.Name)
	}

	// TODO: MODIFY THIS METHOD so that it checks for the instanceTemplateSpecHash label
	dList, err := c.instanceDeploymentLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var deployments []*v1alpha1.InstanceDeployment
	for _, d := range dList {
		selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %v", err)
		}
		// If a deployment with a nil or empty selector creeps in, it should match nothing, not everything.
		if selector.Empty() || !selector.Matches(labels.Set(is.Labels)) {
			continue
		}
		deployments = append(deployments, d)
	}

	if len(deployments) == 0 {
		return nil, fmt.Errorf("could not find deployments set for InstanceSet %s with labels: %v", is.Name, is.Labels)
	}

	return deployments, nil
}

const (
	// RevisionAnnotation is the revision annotation of a deployment's replica sets which records its rollout sequence
	RevisionAnnotation = "deployment.kubernetes.io/revision"
	// RevisionHistoryAnnotation maintains the history of all old revisions that a replica set has served for a deployment.
	RevisionHistoryAnnotation = "deployment.kubernetes.io/revision-history"
	// DesiredReplicasAnnotation is the desired replicas for a deployment recorded as an annotation
	// in its replica sets. Helps in separating scaling events from the rollout process and for
	// determining if the new replica set for a deployment is really saturated.
	DesiredReplicasAnnotation = "deployment.kubernetes.io/desired-replicas"
	// MaxReplicasAnnotation is the maximum replicas a deployment can have at a given point, which
	// is deployment.spec.replicas + maxSurge. Used by the underlying replica sets to estimate their
	// proportions in case the deployment has surge replicas.
	MaxReplicasAnnotation = "deployment.kubernetes.io/max-replicas"

	// RollbackRevisionNotFound is not found rollback event reason
	RollbackRevisionNotFound = "DeploymentRollbackRevisionNotFound"
	// RollbackTemplateUnchanged is the template unchanged rollback event reason
	RollbackTemplateUnchanged = "DeploymentRollbackTemplateUnchanged"
	// RollbackDone is the done rollback event reason
	RollbackDone = "DeploymentRollback"
	// Reasons for deployment conditions
	//
	// Progressing:
	//
	// InstanceSetUpdatedReason is added in a deployment when one of its replica sets is updated as part
	// of the rollout process.
	InstanceSetUpdatedReason = "InstanceSetUpdated"
	// FailedRSCreateReason is added in a deployment when it cannot create a new replica set.
	FailedISCreateReason = "InstanceSetCreateError"
	// NewInstanceSetReason is added in a deployment when it creates a new replica set.
	NewInstanceSetReason = "NewInstanceSetCreated"
	// FoundNewRSReason is added in a deployment when it adopts an existing replica set.
	FoundNewISReason = "FoundNewInstanceSet"
	// NewRSAvailableReason is added in a deployment when its newest replica set is made available
	// ie. the number of new instances that have passed readiness checks and run for at least minReadySeconds
	// is at least the minimum available instances that need to run for the deployment.
	NewISAvailableReason = "NewInstanceSetAvailable"
	// TimedOutReason is added in a deployment when its newest replica set fails to show any progress
	// within the given deadline (progressDeadlineSeconds).
	TimedOutReason = "ProgressDeadlineExceeded"
	// PausedDeployReason is added in a deployment when it is paused. Lack of progress shouldn't be
	// estimated once a deployment is paused.
	PausedInstanceDeployReason = "DeploymentPaused"
	// ResumedDeployReason is added in a deployment when it is resumed. Useful for not failing accidentally
	// deployments that paused amidst a rollout and are bounded by a deadline.
	ResumedInstanceDeployReason = "DeploymentResumed"
	//
	// Available:
	//
	// MinimumReplicasAvailable is added in a deployment when it has its minimum replicas required available.
	MinimumReplicasAvailable = "MinimumReplicasAvailable"
	// MinimumReplicasUnavailable is added in a deployment when it doesn't have the minimum required replicas
	// available.
	MinimumReplicasUnavailable = "MinimumReplicasUnavailable"
)

// NewInstanceDeploymentCondition creates a new deployment condition.
func NewInstanceDeploymentCondition(condType v1alpha1.InstanceDeploymentConditionType, status v1alpha1.ConditionStatus, reason, message string) *v1alpha1.InstanceDeploymentCondition {
	return &v1alpha1.InstanceDeploymentCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetInstanceDeploymentCondition returns the condition with the provided type.
func GetInstanceDeploymentCondition(status v1alpha1.InstanceDeploymentStatus, condType v1alpha1.InstanceDeploymentConditionType) *v1alpha1.InstanceDeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// TODO: remove the duplicate
// GetInstanceDeploymentConditionInternal returns the condition with the provided type.
// Avoiding Internal versions, use standard versions only.
func GetInstanceDeploymentConditionInternal(status v1alpha1.InstanceDeploymentStatus, condType v1alpha1.InstanceDeploymentConditionType) *v1alpha1.InstanceDeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetInstanceDeploymentCondition updates the deployment to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetInstanceDeploymentCondition(status *v1alpha1.InstanceDeploymentStatus, condition v1alpha1.InstanceDeploymentCondition) {
	currentCond := GetInstanceDeploymentCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutDeploymentCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveInstanceDeploymentCondition removes the deployment condition with the provided type.
func RemoveInstanceDeploymentCondition(status *v1alpha1.InstanceDeploymentStatus, condType v1alpha1.InstanceDeploymentConditionType) {
	status.Conditions = filterOutDeploymentCondition(status.Conditions, condType)
}

// filterOutDeploymentCondition returns a new slice of deployment conditions without conditions with the provided type.
func filterOutDeploymentCondition(conditions []v1alpha1.InstanceDeploymentCondition, condType v1alpha1.InstanceDeploymentConditionType) []v1alpha1.InstanceDeploymentCondition {
	var newConditions []v1alpha1.InstanceDeploymentCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// InstanceSetToInstanceDeploymentCondition converts a replica set condition into a deployment condition.
// Useful for promoting replica set failure conditions into deployments.
func InstanceSetToInstanceDeploymentCondition(cond v1alpha1.InstanceSetCondition) v1alpha1.InstanceDeploymentCondition {
	return v1alpha1.InstanceDeploymentCondition{
		Type:               v1alpha1.InstanceDeploymentConditionType(cond.Type),
		Status:             cond.Status,
		LastTransitionTime: cond.LastTransitionTime,
		LastUpdateTime:     cond.LastTransitionTime,
		Reason:             cond.Reason,
		Message:            cond.Message,
	}
}

// SetDeploymentRevision updates the revision for a deployment.
func SetInstanceDeploymentRevision(deployment *v1alpha1.InstanceDeployment, revision string) bool {
	updated := false

	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}
	if deployment.Annotations[RevisionAnnotation] != revision {
		deployment.Annotations[RevisionAnnotation] = revision
		updated = true
	}

	return updated
}

// MaxRevision finds the highest revision in the instance sets
func MaxRevision(allISs []*v1alpha1.InstanceSet) int64 {
	max := int64(0)
	for _, is := range allISs {
		if v, err := Revision(is); err != nil {
			// Skip the replica sets when it failed to parse their revision information
			glog.V(4).Infof("Error: %v. Couldn't parse revision for instance set %#v, instance deployment controller will skip it when reconciling revisions.", err, is)
		} else if v > max {
			max = v
		}
	}
	return max
}

// LastRevision finds the second max revision number in all replica sets (the last revision)
func LastRevision(allISs []*v1alpha1.InstanceSet) int64 {
	max, secMax := int64(0), int64(0)
	for _, is := range allISs {
		if v, err := Revision(is); err != nil {
			// Skip the replica sets when it failed to parse their revision information
			glog.V(4).Infof("Error: %v. Couldn't parse revision for instance set %#v, instance deployment controller will skip it when reconciling revisions.", err, is)
		} else if v >= max {
			secMax = max
			max = v
		} else if v > secMax {
			secMax = v
		}
	}
	return secMax
}

// Revision returns the revision number of the input object.
func Revision(obj runtime.Object) (int64, error) {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return 0, err
	}
	v, ok := acc.GetAnnotations()[RevisionAnnotation]
	if !ok {
		return 0, nil
	}
	return strconv.ParseInt(v, 10, 64)
}

// SetNewInstanceSetAnnotations sets new replica set's annotations appropriately by updating its revision and
// copying required deployment annotations to it; it returns true if replica set's annotation is changed.
func SetNewInstanceSetAnnotations(deployment *v1alpha1.InstanceDeployment, newIS *v1alpha1.InstanceSet, newRevision string, exists bool) bool {
	// First, copy deployment's annotations (except for apply and revision annotations)
	annotationChanged := copyInstanceDeploymentAnnotationsToInstanceSet(deployment, newIS)
	// Then, update replica set's revision annotation
	if newIS.Annotations == nil {
		newIS.Annotations = make(map[string]string)
	}
	oldRevision, ok := newIS.Annotations[RevisionAnnotation]
	// The newRS's revision should be the greatest among all RSes. Usually, its revision number is newRevision (the max revision number
	// of all old RSes + 1). However, it's possible that some of the old RSes are deleted after the newRS revision being updated, and
	// newRevision becomes smaller than newRS's revision. We should only update newRS revision when it's smaller than newRevision.

	oldRevisionInt, err := strconv.ParseInt(oldRevision, 10, 64)
	if err != nil {
		if oldRevision != "" {
			glog.Warningf("Updating instance set revision OldRevision not int %s", err)
			return false
		}
		//If the RS annotation is empty then initialise it to 0
		oldRevisionInt = 0
	}
	newRevisionInt, err := strconv.ParseInt(newRevision, 10, 64)
	if err != nil {
		glog.Warningf("Updating instance set revision NewRevision not int %s", err)
		return false
	}
	if oldRevisionInt < newRevisionInt {
		newIS.Annotations[RevisionAnnotation] = newRevision
		annotationChanged = true
		glog.V(4).Infof("Updating instance set %q revision to %s", newIS.Name, newRevision)
	}
	// If a revision annotation already existed and this replica set was updated with a new revision
	// then that means we are rolling back to this replica set. We need to preserve the old revisions
	// for historical information.
	if ok && annotationChanged {
		revisionHistoryAnnotation := newIS.Annotations[RevisionHistoryAnnotation]
		oldRevisions := strings.Split(revisionHistoryAnnotation, ",")
		if len(oldRevisions[0]) == 0 {
			newIS.Annotations[RevisionHistoryAnnotation] = oldRevision
		} else {
			oldRevisions = append(oldRevisions, oldRevision)
			newIS.Annotations[RevisionHistoryAnnotation] = strings.Join(oldRevisions, ",")
		}
	}
	// If the new replica set is about to be created, we need to add replica annotations to it.
	if !exists && SetReplicasAnnotations(newIS, (deployment.Spec.Replicas), (deployment.Spec.Replicas)+MaxSurge(*deployment)) {
		annotationChanged = true
	}
	return annotationChanged
}

var annotationsToSkip = map[string]bool{
	v1.LastAppliedConfigAnnotation: true,
	RevisionAnnotation:             true,
	RevisionHistoryAnnotation:      true,
	DesiredReplicasAnnotation:      true,
	MaxReplicasAnnotation:          true,
}

// skipCopyAnnotation returns true if we should skip copying the annotation with the given annotation key
// TODO: How to decide which annotations should / should not be copied?
//       See https://github.com/kubernetes/kubernetes/pull/20035#issuecomment-179558615
func skipCopyAnnotation(key string) bool {
	return annotationsToSkip[key]
}

// copyDeploymentAnnotationsToInstanceSet copies deployment's annotations to replica set's annotations,
// and returns true if replica set's annotation is changed.
// Note that apply and revision annotations are not copied.
func copyInstanceDeploymentAnnotationsToInstanceSet(deployment *v1alpha1.InstanceDeployment, is *v1alpha1.InstanceSet) bool {
	isAnnotationsChanged := false
	if is.Annotations == nil {
		is.Annotations = make(map[string]string)
	}
	for k, v := range deployment.Annotations {
		// newRS revision is updated automatically in getNewInstanceSet, and the deployment's revision number is then updated
		// by copying its newRS revision number. We should not copy deployment's revision to its newRS, since the update of
		// deployment revision number may fail (revision becomes stale) and the revision number in newRS is more reliable.
		if skipCopyAnnotation(k) || is.Annotations[k] == v {
			continue
		}
		is.Annotations[k] = v
		isAnnotationsChanged = true
	}
	return isAnnotationsChanged
}

// SetDeploymentAnnotationsTo sets deployment's annotations as given RS's annotations.
// This action should be done if and only if the deployment is rolling back to this rs.
// Note that apply and revision annotations are not changed.
func SetInstanceDeploymentAnnotationsTo(deployment *v1alpha1.InstanceDeployment, rollbackToIS *v1alpha1.InstanceSet) {
	deployment.Annotations = getSkippedAnnotations(deployment.Annotations)
	for k, v := range rollbackToIS.Annotations {
		if !skipCopyAnnotation(k) {
			deployment.Annotations[k] = v
		}
	}
}

func getSkippedAnnotations(annotations map[string]string) map[string]string {
	skippedAnnotations := make(map[string]string)
	for k, v := range annotations {
		if skipCopyAnnotation(k) {
			skippedAnnotations[k] = v
		}
	}
	return skippedAnnotations
}

// FindActiveOrLatest returns the only active or the latest replica set in case there is at most one active
// replica set. If there are more active replica sets, then we should proportionally scale them.
func FindActiveOrLatest(newIS *v1alpha1.InstanceSet, oldISs []*v1alpha1.InstanceSet) *v1alpha1.InstanceSet {
	if newIS == nil && len(oldISs) == 0 {
		return nil
	}

	sort.Sort(sort.Reverse(InstanceSetsByCreationTimestamp(oldISs)))
	allISs := FilterActiveInstanceSets(append(oldISs, newIS))

	switch len(allISs) {
	case 0:
		// If there is no active replica set then we should return the newest.
		if newIS != nil {
			return newIS
		}
		return oldISs[0]
	case 1:
		return allISs[0]
	default:
		return nil
	}
}

// GetDesiredReplicasAnnotation returns the number of desired replicas
func GetDesiredReplicasAnnotation(is *v1alpha1.InstanceSet) (int32, bool) {
	return getIntFromAnnotation(is, DesiredReplicasAnnotation)
}

func getMaxReplicasAnnotation(is *v1alpha1.InstanceSet) (int32, bool) {
	return getIntFromAnnotation(is, MaxReplicasAnnotation)
}

func getIntFromAnnotation(is *v1alpha1.InstanceSet, annotationKey string) (int32, bool) {
	annotationValue, ok := is.Annotations[annotationKey]
	if !ok {
		return int32(0), false
	}
	intValue, err := strconv.Atoi(annotationValue)
	if err != nil {
		glog.V(2).Infof("Cannot convert the value %q with annotation key %q for the instance set %q", annotationValue, annotationKey, is.Name)
		return int32(0), false
	}
	return int32(intValue), true
}

// SetReplicasAnnotations sets the desiredReplicas and maxReplicas into the annotations
func SetReplicasAnnotations(is *v1alpha1.InstanceSet, desiredReplicas, maxReplicas int32) bool {
	updated := false
	if is.Annotations == nil {
		is.Annotations = make(map[string]string)
	}
	desiredString := fmt.Sprintf("%d", desiredReplicas)
	if hasString := is.Annotations[DesiredReplicasAnnotation]; hasString != desiredString {
		is.Annotations[DesiredReplicasAnnotation] = desiredString
		updated = true
	}
	maxString := fmt.Sprintf("%d", maxReplicas)
	if hasString := is.Annotations[MaxReplicasAnnotation]; hasString != maxString {
		is.Annotations[MaxReplicasAnnotation] = maxString
		updated = true
	}
	return updated
}

// MaxUnavailable returns the maximum unavailable instances a rolling deployment can take.
func MaxUnavailable(deployment v1alpha1.InstanceDeployment) int32 {
	if !IsRollingUpdate(&deployment) || (deployment.Spec.Replicas) == 0 {
		return int32(0)
	}
	// Error caught by validation
	_, maxUnavailable, _ := ResolveFenceposts(deployment.Spec.Strategy.RollingUpdate.MaxSurge, deployment.Spec.Strategy.RollingUpdate.MaxUnavailable, (deployment.Spec.Replicas))
	if maxUnavailable > deployment.Spec.Replicas {
		return deployment.Spec.Replicas
	}
	return maxUnavailable
}

// MinAvailable returns the minimum available instances of a given deployment
func MinAvailable(deployment *v1alpha1.InstanceDeployment) int32 {
	if !IsRollingUpdate(deployment) {
		return int32(0)
	}
	return (deployment.Spec.Replicas) - MaxUnavailable(*deployment)
}

// MaxSurge returns the maximum surge instances a rolling deployment can take.
func MaxSurge(deployment v1alpha1.InstanceDeployment) int32 {
	if !IsRollingUpdate(&deployment) {
		return int32(0)
	}
	// Error caught by validation
	maxSurge, _, _ := ResolveFenceposts(deployment.Spec.Strategy.RollingUpdate.MaxSurge, deployment.Spec.Strategy.RollingUpdate.MaxUnavailable, (deployment.Spec.Replicas))
	return maxSurge
}

// GetProportion will estimate the proportion for the provided replica set using 1. the current size
// of the parent deployment, 2. the replica count that needs be added on the replica sets of the
// deployment, and 3. the total replicas added in the replica sets of the deployment so far.
func GetProportion(is *v1alpha1.InstanceSet, d v1alpha1.InstanceDeployment, deploymentReplicasToAdd, deploymentReplicasAdded int32) int32 {
	if is == nil || (is.Spec.Replicas) == 0 || deploymentReplicasToAdd == 0 || deploymentReplicasToAdd == deploymentReplicasAdded {
		return int32(0)
	}

	isFraction := getInstanceSetFraction(*is, d)
	allowed := deploymentReplicasToAdd - deploymentReplicasAdded

	if deploymentReplicasToAdd > 0 {
		// Use the minimum between the replica set fraction and the maximum allowed replicas
		// when scaling up. This way we ensure we will not scale up more than the allowed
		// replicas we can add.
		return integer.Int32Min(isFraction, allowed)
	}
	// Use the maximum between the replica set fraction and the maximum allowed replicas
	// when scaling down. This way we ensure we will not scale down more than the allowed
	// replicas we can remove.
	return integer.Int32Max(isFraction, allowed)
}

// getInstanceSetFraction estimates the fraction of replicas a replica set can have in
// 1. a scaling event during a rollout or 2. when scaling a paused deployment.
func getInstanceSetFraction(is v1alpha1.InstanceSet, d v1alpha1.InstanceDeployment) int32 {
	// If we are scaling down to zero then the fraction of this replica set is its whole size (negative)
	if (d.Spec.Replicas) == int32(0) {
		return -(is.Spec.Replicas)
	}

	deploymentReplicas := (d.Spec.Replicas) + MaxSurge(d)
	annotatedReplicas, ok := getMaxReplicasAnnotation(&is)
	if !ok {
		// If we cannot find the annotation then fallback to the current deployment size. Note that this
		// will not be an accurate proportion estimation in case other replica sets have different values
		// which means that the deployment was scaled at some point but we at least will stay in limits
		// due to the min-max comparisons in getProportion.
		annotatedReplicas = d.Status.Replicas
	}

	// We should never proportionally scale up from zero which means rs.spec.replicas and annotatedReplicas
	// will never be zero here.
	newISsize := (float64((is.Spec.Replicas) * deploymentReplicas)) / float64(annotatedReplicas)
	return integer.RoundToInt32(newISsize) - (is.Spec.Replicas)
}

// GetAllInstanceSets returns the old and new replica sets targeted by the given Deployment. It gets InstanceList and InstanceSetList from client interface.
// Note that the first set of old replica sets doesn't include the ones with no instances, and the second set of old replica sets include all old replica sets.
// The third returned value is the new replica set, and it may be nil if it doesn't exist yet.
func GetAllInstanceSets(deployment *v1alpha1.InstanceDeployment, c v1alpha1client.NodeV1alpha1Interface) ([]*v1alpha1.InstanceSet, []*v1alpha1.InstanceSet, *v1alpha1.InstanceSet, error) {
	isList, err := ListInstanceSets(deployment, IsListFromClient(c))
	if err != nil {
		return nil, nil, nil, err
	}
	oldISes, allOldISes := FindOldInstanceSets(deployment, isList)
	newIS := FindNewInstanceSet(deployment, isList)
	return oldISes, allOldISes, newIS, nil
}

// GetOldInstanceSets returns the old replica sets targeted by the given Deployment; get InstanceList and InstanceSetList from client interface.
// Note that the first set of old replica sets doesn't include the ones with no instances, and the second set of old replica sets include all old replica sets.
func GetOldInstanceSets(deployment *v1alpha1.InstanceDeployment, c v1alpha1client.NodeV1alpha1Interface) ([]*v1alpha1.InstanceSet, []*v1alpha1.InstanceSet, error) {
	rsList, err := ListInstanceSets(deployment, IsListFromClient(c))
	if err != nil {
		return nil, nil, err
	}
	oldRSes, allOldRSes := FindOldInstanceSets(deployment, rsList)
	return oldRSes, allOldRSes, nil
}

// GetNewInstanceSet returns a replica set that matches the intent of the given deployment; get InstanceSetList from client interface.
// Returns nil if the new replica set doesn't exist yet.
func GetNewInstanceSet(deployment *v1alpha1.InstanceDeployment, c v1alpha1client.NodeV1alpha1Interface) (*v1alpha1.InstanceSet, error) {
	rsList, err := ListInstanceSets(deployment, IsListFromClient(c))
	if err != nil {
		return nil, err
	}
	return FindNewInstanceSet(deployment, rsList), nil
}

// RsListFromClient returns an rsListFunc that wraps the given client.
func IsListFromClient(c v1alpha1client.NodeV1alpha1Interface) IsListFunc {
	return func(namespace string, options metav1.ListOptions) ([]*v1alpha1.InstanceSet, error) {
		isList, err := c.InstanceSets().List(options)
		if err != nil {
			return nil, err
		}
		var ret []*v1alpha1.InstanceSet
		for i := range isList.Items {
			ret = append(ret, &isList.Items[i])
		}
		return ret, err
	}
}

// TODO: switch this to full namespacers
type IsListFunc func(string, metav1.ListOptions) ([]*v1alpha1.InstanceSet, error)
type instanceListFunc func(string, metav1.ListOptions) (*v1alpha1.InstanceList, error)

// ListInstanceSets returns a slice of RSes the given deployment targets.
// Note that this does NOT attempt to reconcile ControllerRef (adopt/orphan),
// because only the controller itself should do that.
// However, it does filter out anything whose ControllerRef doesn't match.
func ListInstanceSets(deployment *v1alpha1.InstanceDeployment, getISList IsListFunc) ([]*v1alpha1.InstanceSet, error) {
	// TODO: Right now we list replica sets by their labels. We should list them by selector, i.e. the replica set's selector
	//       should be a superset of the deployment's selector, see https://github.com/kubernetes/kubernetes/issues/19830.
	namespace := deployment.Namespace
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{LabelSelector: selector.String()}
	all, err := getISList(namespace, options)
	if err != nil {
		return nil, err
	}
	// Only include those whose ControllerRef matches the Deployment.
	owned := make([]*v1alpha1.InstanceSet, 0, len(all))
	for _, is := range all {
		if metav1.IsControlledBy(is, deployment) {
			owned = append(owned, is)
		}
	}
	return owned, nil
}

// ListInstanceSetsInternal is ListInstanceSets for v1alpha1.
// TODO: Remove the duplicate when call sites are updated to ListInstanceSets.
func ListInstanceSetsInternal(deployment *v1alpha1.InstanceDeployment, getISList func(string, metav1.ListOptions) ([]*v1alpha1.InstanceSet, error)) ([]*v1alpha1.InstanceSet, error) {
	namespace := deployment.Namespace
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{LabelSelector: selector.String()}
	all, err := getISList(namespace, options)
	if err != nil {
		return nil, err
	}
	// Only include those whose ControllerRef matches the Deployment.
	filtered := make([]*v1alpha1.InstanceSet, 0, len(all))
	for _, is := range all {
		if metav1.IsControlledBy(is, deployment) {
			filtered = append(filtered, is)
		}
	}
	return filtered, nil
}


func ListInstances(deployment *v1alpha1.InstanceDeployment, isList []*v1alpha1.InstanceSet, getInstanceList instanceListFunc) (*v1alpha1.InstanceList, error) {
	namespace := deployment.Namespace
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{LabelSelector: selector.String()}
	all, err := getInstanceList(namespace, options)
	if err != nil {
		return all, err
	}
	// Only include those whose ControllerRef points to a InstanceSet that is in
	// turn owned by this Deployment.
	isMap := make(map[types.UID]bool, len(isList))
	for _, is := range isList {
		isMap[is.UID] = true
	}
	owned := &v1alpha1.InstanceList{Items: make([]v1alpha1.Instance, 0, len(all.Items))}
	for i := range all.Items {
		instance := &all.Items[i]
		controllerRef := metav1.GetControllerOf(instance)
		if controllerRef != nil && isMap[controllerRef.UID] {
			owned.Items = append(owned.Items, *instance)
		}
	}
	return owned, nil
}

// EqualIgnoreHash returns true if two given instanceTemplateSpec are equal, ignoring the diff in value of Labels[instance-template-hash]
// We ignore instance-template-hash because the hash result would be different upon instanceTemplateSpec API changes
// (e.g. the addition of a new field will cause the hash code to change)
// Note that we assume input instanceTemplateSpecs contain non-empty labels
func EqualIgnoreHash(template1, template2 *v1alpha1.InstanceTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	// First, compare template.Labels (ignoring hash)
	labels1, labels2 := t1Copy.Labels, t2Copy.Labels
	if len(labels1) > len(labels2) {
		labels1, labels2 = labels2, labels1
	}
	// We make sure len(labels2) >= len(labels1)
	for k, v := range labels2 {
		if labels1[k] != v && k != v1alpha1.DefaultInstanceDeploymentUniqueLabelKey {
			return false
		}
	}
	// Then, compare the templates without comparing their labels
	t1Copy.Labels, t2Copy.Labels = nil, nil
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}

// FindNewInstanceSet returns the new RS this given deployment targets (the one with the same instance template).
func FindNewInstanceSet(deployment *v1alpha1.InstanceDeployment, isList []*v1alpha1.InstanceSet) *v1alpha1.InstanceSet {
	sort.Sort(InstanceSetsByCreationTimestamp(isList))
	for i := range isList {
		if EqualIgnoreHash(&isList[i].Spec.Template, &deployment.Spec.Template) {
			// In rare cases, such as after cluster upgrades, Deployment may end up with
			// having more than one new InstanceSets that have the same template as its template,
			// see https://github.com/kubernetes/kubernetes/issues/40415
			// We deterministically choose the oldest new InstanceSet.
			return isList[i]
		}
	}
	// new InstanceSet does not exist.
	return nil
}

// FindOldInstanceSets returns the old replica sets targeted by the given Deployment, with the given slice of RSes.
// Note that the first set of old replica sets doesn't include the ones with no instances, and the second set of old replica sets include all old replica sets.
func FindOldInstanceSets(deployment *v1alpha1.InstanceDeployment, isList []*v1alpha1.InstanceSet) ([]*v1alpha1.InstanceSet, []*v1alpha1.InstanceSet) {
	var requiredISs []*v1alpha1.InstanceSet
	var allISs []*v1alpha1.InstanceSet
	newIS := FindNewInstanceSet(deployment, isList)
	for _, is := range isList {
		// Filter out new replica set
		if newIS != nil && is.UID == newIS.UID {
			continue
		}
		allISs = append(allISs, is)
		if (is.Spec.Replicas) != 0 {
			requiredISs = append(requiredISs, is)
		}
	}
	return requiredISs, allISs
}

// WaitForInstanceSetUpdated polls the replica set until it is updated.
func WaitForInstanceSetUpdated(c v1alpha1listers.InstanceSetLister, desiredGeneration int64, namespace, name string) error {
	return wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		is, err := c.Get(name)
		if err != nil {
			return false, err
		}
		return is.Status.ObservedGeneration >= desiredGeneration, nil
	})
}

// WaitForInstancesHashPopulated polls the replica set until updated and fully labeled.
func WaitForInstancesHashPopulated(c v1alpha1listers.InstanceSetLister, desiredGeneration int64, namespace, name string) error {
	return wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		is, err := c.Get(name)
		if err != nil {
			return false, err
		}
		return is.Status.ObservedGeneration >= desiredGeneration &&
			is.Status.FullyLabeledReplicas == (is.Spec.Replicas), nil
	})
}

// LabelInstancesWithHash labels all instances in the given instanceList with the new hash label.
func LabelInstancesWithHash(instanceList *v1alpha1.InstanceList, c v1alpha1client.NodeV1alpha1Interface , instanceLister v1alpha1listers.InstanceLister, namespace, name, hash string) error {
	for _, instance := range instanceList.Items {
		// Ignore inactive Instances.
		if !IsInstanceActive(&instance) {
			continue
		}
		// Only label the instance that doesn't already have the new hash
		if instance.Labels[v1alpha1.DefaultInstanceDeploymentUniqueLabelKey] != hash {
			_, err := UpdateInstanceWithRetries(c.Instances(), instanceLister, instance.Namespace, instance.Name,
				func(instanceToUpdate *v1alpha1.Instance) error {
					// Precondition: the instance doesn't contain the new hash in its label.
					if instanceToUpdate.Labels[v1alpha1.DefaultInstanceDeploymentUniqueLabelKey] == hash {
						return errors.ErrPreconditionViolated
					}
					instanceToUpdate.Labels = labelsutil.AddLabel(instanceToUpdate.Labels, v1alpha1.DefaultInstanceDeploymentUniqueLabelKey, hash)
					return nil
				})
			if err != nil {
				return fmt.Errorf("error in adding template hash label %s to instance %q: %v", hash, instance.Name, err)
			}
			glog.V(4).Infof("Labeled instance %s/%s of InstanceSet %s/%s with hash %s.", instance.Namespace, instance.Name, namespace, name, hash)
		}
	}
	return nil
}

// SetFromInstanceSetTemplate sets the desired InstanceTemplateSpec from a replica set template to the given deployment.
func SetFromInstanceSetTemplate(deployment *v1alpha1.InstanceDeployment, template v1alpha1.InstanceTemplateSpec) *v1alpha1.InstanceDeployment {
	deployment.Spec.Template.ObjectMeta = template.ObjectMeta
	deployment.Spec.Template.Spec = template.Spec
	deployment.Spec.Template.ObjectMeta.Labels = labelsutil.CloneAndRemoveLabel(
		deployment.Spec.Template.ObjectMeta.Labels,
		v1alpha1.DefaultInstanceDeploymentUniqueLabelKey)
	return deployment
}

// GetReplicaCountForInstanceSets returns the sum of Replicas of the given replica sets.
func GetReplicaCountForInstanceSets(InstanceSets []*v1alpha1.InstanceSet) int32 {
	totalReplicas := int32(0)
	for _, is := range InstanceSets {
		if is != nil {
			totalReplicas += (is.Spec.Replicas)
		}
	}
	return totalReplicas
}

// GetActualReplicaCountForInstanceSets returns the sum of actual replicas of the given replica sets.
func GetActualReplicaCountForInstanceSets(InstanceSets []*v1alpha1.InstanceSet) int32 {
	totalActualReplicas := int32(0)
	for _, is := range InstanceSets {
		if is != nil {
			totalActualReplicas += is.Status.Replicas
		}
	}
	return totalActualReplicas
}

// GetReadyReplicaCountForInstanceSets returns the number of ready instances corresponding to the given replica sets.
func GetReadyReplicaCountForInstanceSets(InstanceSets []*v1alpha1.InstanceSet) int32 {
	totalReadyReplicas := int32(0)
	for _, is := range InstanceSets {
		if is != nil {
			totalReadyReplicas += is.Status.ReadyReplicas
		}
	}
	return totalReadyReplicas
}

// GetAvailableReplicaCountForInstanceSets returns the number of available instances corresponding to the given replica sets.
func GetAvailableReplicaCountForInstanceSets(InstanceSets []*v1alpha1.InstanceSet) int32 {
	totalAvailableReplicas := int32(0)
	for _, is := range InstanceSets {
		if is != nil {
			totalAvailableReplicas += is.Status.AvailableReplicas
		}
	}
	return totalAvailableReplicas
}

// IsRollingUpdate returns true if the strategy type is a rolling update.
func IsRollingUpdate(deployment *v1alpha1.InstanceDeployment) bool {
	return deployment.Spec.Strategy.Type == v1alpha1.RollingUpdateInstanceDeploymentStrategyType
}

// DeploymentComplete considers a deployment to be complete once all of its desired replicas
// are updated and available, and no old instances are running.
func InstanceDeploymentComplete(deployment *v1alpha1.InstanceDeployment, newStatus *v1alpha1.InstanceDeploymentStatus) bool {
	return newStatus.UpdatedReplicas == (deployment.Spec.Replicas) &&
		newStatus.Replicas == (deployment.Spec.Replicas) &&
		newStatus.AvailableReplicas == (deployment.Spec.Replicas) &&
		newStatus.ObservedGeneration >= deployment.Generation
}

// DeploymentProgressing reports progress for a deployment. Progress is estimated by comparing the
// current with the new status of the deployment that the controller is observing. More specifically,
// when new instances are scaled up or become ready or available, or old instances are scaled down, then we
// consider the deployment is progressing.
func InstanceDeploymentProgressing(deployment *v1alpha1.InstanceDeployment, newStatus *v1alpha1.InstanceDeploymentStatus) bool {
	oldStatus := deployment.Status

	// Old replicas that need to be scaled down
	oldStatusOldReplicas := oldStatus.Replicas - oldStatus.UpdatedReplicas
	newStatusOldReplicas := newStatus.Replicas - newStatus.UpdatedReplicas

	return (newStatus.UpdatedReplicas > oldStatus.UpdatedReplicas) ||
		(newStatusOldReplicas < oldStatusOldReplicas) ||
		newStatus.ReadyReplicas > deployment.Status.ReadyReplicas ||
		newStatus.AvailableReplicas > deployment.Status.AvailableReplicas
}

// used for unit testing
var nowFn = func() time.Time { return time.Now() }

// DeploymentTimedOut considers a deployment to have timed out once its condition that reports progress
// is older than progressDeadlineSeconds or a Progressing condition with a TimedOutReason reason already
// exists.
func InstanceDeploymentTimedOut(deployment *v1alpha1.InstanceDeployment, newStatus *v1alpha1.InstanceDeploymentStatus) bool {
	if deployment.Spec.ProgressDeadlineSeconds == nil {
		return false
	}

	// Look for the Progressing condition. If it doesn't exist, we have no base to estimate progress.
	// If it's already set with a TimedOutReason reason, we have already timed out, no need to check
	// again.
	condition := GetInstanceDeploymentCondition(*newStatus, v1alpha1.InstanceDeploymentProgressing)
	if condition == nil {
		return false
	}
	// If the previous condition has been a successful rollout then we shouldn't try to
	// estimate any progress. Scenario:
	//
	// * progressDeadlineSeconds is smaller than the difference between now and the time
	//   the last rollout finished in the past.
	// * the creation of a new InstanceSet triggers a resync of the Deployment prior to the
	//   cached copy of the Deployment getting updated with the status.condition that indicates
	//   the creation of the new InstanceSet.
	//
	// The Deployment will be resynced and eventually its Progressing condition will catch
	// up with the state of the world.
	if condition.Reason == NewISAvailableReason {
		return false
	}
	if condition.Reason == TimedOutReason {
		return true
	}

	// Look at the difference in seconds between now and the last time we reported any
	// progress or tried to create a replica set, or resumed a paused deployment and
	// compare against progressDeadlineSeconds.
	from := condition.LastUpdateTime
	now := nowFn()
	delta := time.Duration(*deployment.Spec.ProgressDeadlineSeconds) * time.Second
	timedOut := from.Add(delta).Before(now)

	glog.V(4).Infof("InstanceDeployment %q timed out (%t) [last progress check: %v - now: %v]", deployment.Name, timedOut, from, now)
	return timedOut
}

// NewRSNewReplicas calculates the number of replicas a deployment's new RS should have.
// When one of the followings is true, we're rolling out the deployment; otherwise, we're scaling it.
// 1) The new RS is saturated: newRS's replicas == deployment's replicas
// 2) Max number of instances allowed is reached: deployment's replicas + maxSurge == all RSs' replicas
func NewISNewReplicas(deployment *v1alpha1.InstanceDeployment, allISs []*v1alpha1.InstanceSet, newIS *v1alpha1.InstanceSet) (int32, error) {
	switch deployment.Spec.Strategy.Type {
	case v1alpha1.RollingUpdateInstanceDeploymentStrategyType:
		// Check if we can scale up.
		maxSurge, err := intstrutil.GetValueFromIntOrPercent(deployment.Spec.Strategy.RollingUpdate.MaxSurge, int((deployment.Spec.Replicas)), true)
		if err != nil {
			return 0, err
		}
		// Find the total number of instances
		currentInstanceCount := GetReplicaCountForInstanceSets(allISs)
		maxTotalInstances := (deployment.Spec.Replicas) + int32(maxSurge)
		if currentInstanceCount >= maxTotalInstances {
			// Cannot scale up.
			return (newIS.Spec.Replicas), nil
		}
		// Scale up.
		scaleUpCount := maxTotalInstances - currentInstanceCount
		// Do not exceed the number of desired replicas.
		scaleUpCount = int32(integer.IntMin(int(scaleUpCount), int((deployment.Spec.Replicas)-(newIS.Spec.Replicas))))
		return (newIS.Spec.Replicas) + scaleUpCount, nil
	case v1alpha1.RecreateInstanceDeploymentStrategyType:
		return (deployment.Spec.Replicas), nil
	default:
		return 0, fmt.Errorf("instance deployment type %v isn't supported", deployment.Spec.Strategy.Type)
	}
}

// IsSaturated checks if the new replica set is saturated by comparing its size with its deployment size.
// Both the deployment and the replica set have to believe this replica set can own all of the desired
// replicas in the deployment and the annotation helps in achieving that. All instances of the InstanceSet
// need to be available.
func IsSaturated(deployment *v1alpha1.InstanceDeployment, is *v1alpha1.InstanceSet) bool {
	if is == nil {
		return false
	}
	desiredString := is.Annotations[DesiredReplicasAnnotation]
	desired, err := strconv.Atoi(desiredString)
	if err != nil {
		return false
	}
	return (is.Spec.Replicas) == (deployment.Spec.Replicas) &&
		int32(desired) == (deployment.Spec.Replicas) &&
		is.Status.AvailableReplicas == (deployment.Spec.Replicas)
}

// WaitForObservedDeployment polls for deployment to be updated so that deployment.Status.ObservedGeneration >= desiredGeneration.
// Returns error if polling timesout.
func WaitForObservedInstanceDeployment(getDeploymentFunc func() (*v1alpha1.InstanceDeployment, error), desiredGeneration int64, interval, timeout time.Duration) error {
	// TODO: This should take clientset.Interface when all code is updated to use clientset. Keeping it this way allows the function to be used by callers who have client.Interface.
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := getDeploymentFunc()
		if err != nil {
			return false, err
		}
		return deployment.Status.ObservedGeneration >= desiredGeneration, nil
	})
}

// TODO: remove the duplicate
// WaitForObservedInternalDeployment polls for deployment to be updated so that deployment.Status.ObservedGeneration >= desiredGeneration.
// Returns error if polling timesout.
func WaitForObservedDeploymentInternal(getDeploymentFunc func() (*v1alpha1.InstanceDeployment, error), desiredGeneration int64, interval, timeout time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		deployment, err := getDeploymentFunc()
		if err != nil {
			return false, err
		}
		return deployment.Status.ObservedGeneration >= desiredGeneration, nil
	})
}

// ResolveFenceposts resolves both maxSurge and maxUnavailable. This needs to happen in one
// step. For example:
//
// 2 desired, max unavailable 1%, surge 0% - should scale old(-1), then new(+1), then old(-1), then new(+1)
// 1 desired, max unavailable 1%, surge 0% - should scale old(-1), then new(+1)
// 2 desired, max unavailable 25%, surge 1% - should scale new(+1), then old(-1), then new(+1), then old(-1)
// 1 desired, max unavailable 25%, surge 1% - should scale new(+1), then old(-1)
// 2 desired, max unavailable 0%, surge 1% - should scale new(+1), then old(-1), then new(+1), then old(-1)
// 1 desired, max unavailable 0%, surge 1% - should scale new(+1), then old(-1)
func ResolveFenceposts(maxSurge, maxUnavailable *intstrutil.IntOrString, desired int32) (int32, int32, error) {
	surge, err := intstrutil.GetValueFromIntOrPercent(maxSurge, int(desired), true)
	if err != nil {
		return 0, 0, err
	}
	unavailable, err := intstrutil.GetValueFromIntOrPercent(maxUnavailable, int(desired), false)
	if err != nil {
		return 0, 0, err
	}

	if surge == 0 && unavailable == 0 {
		// Validation should never allow the user to explicitly use zero values for both maxSurge
		// maxUnavailable. Due to rounding down maxUnavailable though, it may resolve to zero.
		// If both fenceposts resolve to zero, then we should set maxUnavailable to 1 on the
		// theory that surge might not work due to quota.
		unavailable = 1
	}

	return int32(surge), int32(unavailable), nil
}

