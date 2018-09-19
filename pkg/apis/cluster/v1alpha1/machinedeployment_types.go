/*
Copyright 2018 The Kubernetes Authors.

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

package v1alpha1

import (
	"log"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	cluster "github.com/gardener/machine-controller-manager/pkg/apis/cluster"
	"github.com/gardener/machine-controller-manager/pkg/apis/cluster/common"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineDeployment
// +k8s:openapi-gen=true
// +resource:path=machinedeployments,strategy=MachineDeploymentValidationStrategy
type MachineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineDeploymentSpec   `json:"spec,omitempty"`
	Status MachineDeploymentStatus `json:"status,omitempty"`
}

// MachineDeploymentSpec defines the desired state of MachineDeployment
type MachineDeploymentSpec struct {
	// Number of desired machines. Defaults to 1.
	// This is a pointer to distinguish between explicit zero and not specified.
	Replicas *int32 `json:"replicas,omitempty"`

	// Label selector for machines. Existing MachineSets whose machines are
	// selected by this will be the ones affected by this deployment.
	// It must match the machine template's labels.
	Selector *metav1.LabelSelector `json:"selector"`

	// Template describes the machines that will be created.
	Template MachineTemplateSpec `json:"template"`

	// The deployment strategy to use to replace existing machines with
	// new ones.
	// +optional
	Strategy MachineDeploymentStrategy `json:"strategy,omitempty"`

	// Minimum number of seconds for which a newly created machine should
	// be ready.
	// Defaults to 0 (machine will be considered available as soon as it
	// is ready)
	// +optional
	MinReadySeconds *int32 `json:"minReadySeconds,omitempty"`

	// The number of old MachineSets to retain to allow rollback.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 1.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// Indicates that the deployment is paused.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// DEPRECATED.
	// The config this MachineDeployment is rolling back to. Will be cleared after rollback is done.
	// +optional
	RollbackTo *RollbackConfig `json:"rollbackTo,omitempty" protobuf:"bytes,8,opt,name=rollbackTo"`

	// The maximum time in seconds for a deployment to make progress before it
	// is considered to be failed. The deployment controller will continue to
	// process failed deployments and a condition with a ProgressDeadlineExceeded
	// reason will be surfaced in the deployment status. Note that progress will
	// not be estimated during the time a deployment is paused. Defaults to 600s.
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DEPRECATED.
// MachineDeploymentRollback stores the information required to rollback a MachineDeployment.
type MachineDeploymentRollback struct {
	metav1.TypeMeta `json:",inline"`

	// Required: This must match the Name of a MachineDeployment.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// The annotations to be updated to a MachineDeployment
	// +optional
	UpdatedAnnotations map[string]string `json:"updatedAnnotations,omitempty" protobuf:"bytes,2,rep,name=updatedAnnotations"`

	// The config of this MachineDeployment rollback.
	RollbackTo RollbackConfig `json:"rollbackTo" protobuf:"bytes,3,opt,name=rollbackTo"`
}

type RollbackConfig struct {
	// The revision to rollback to. If set to 0, rollback to the last revision.
	// +optional
	Revision int64 `json:"revision,omitempty" protobuf:"varint,1,opt,name=revision"`
}

const (
	// DefaultDeploymentUniqueLabelKey is the default key of the selector that is added
	// to existing MCs (and label key that is added to its machines) to prevent the existing MCs
	// to select new machines (and old machines being select by new MC).
	DefaultMachineDeploymentUniqueLabelKey string = "machine-template-hash"
)

// MachineDeploymentStrategy describes how to replace existing machines
// with new ones.
type MachineDeploymentStrategy struct {
	// Type of deployment. Currently the only supported strategy is
	// "RollingUpdate".
	// Default is RollingUpdate.
	// +optional
	Type common.MachineDeploymentStrategyType `json:"type,omitempty"`

	// Rolling update config params. Present only if
	// MachineDeploymentStrategyType = RollingUpdate.
	// +optional
	RollingUpdate *MachineRollingUpdateDeployment `json:"rollingUpdate,omitempty"`
}

// Spec to control the desired behavior of rolling update.
type MachineRollingUpdateDeployment struct {
	// The maximum number of machines that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// Defaults to 0.
	// Example: when this is set to 30%, the old MachineSet can be scaled
	// down to 70% of desired machines immediately when the rolling update
	// starts. Once new machines are ready, old MachineSet can be scaled
	// down further, followed by scaling up the new MachineSet, ensuring
	// that the total number of machines available at all times
	// during the update is at least 70% of desired machines.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,1,opt,name=maxUnavailable"`

	// The maximum number of machines that can be scheduled above the
	// desired number of machines.
	// Value can be an absolute number (ex: 5) or a percentage of
	// desired machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// Defaults to 1.
	// Example: when this is set to 30%, the new MachineSet can be scaled
	// up immediately when the rolling update starts, such that the total
	// number of old and new machines do not exceed 130% of desired
	// machines. Once old machines have been killed, new MachineSet can
	// be scaled up further, ensuring that total number of machines running
	// at any time during the update is at most 130% of desired machines.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty" protobuf:"bytes,2,opt,name=maxSurge"`
}

// MachineDeploymentStatus defines the observed state of MachineDeployment
type MachineDeploymentStatus struct {
	// The generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`

	// Total number of non-terminated machines targeted by this deployment
	// (their labels match the selector).
	// +optional
	Replicas int32 `json:"replicas,omitempty" protobuf:"varint,2,opt,name=replicas"`

	// Total number of non-terminated machines targeted by this deployment
	// that have the desired template spec.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty" protobuf:"varint,3,opt,name=updatedReplicas"`

	// Total number of ready machines targeted by this deployment.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty" protobuf:"varint,7,opt,name=readyReplicas"`

	// Total number of available machines (ready for at least minReadySeconds)
	// targeted by this deployment.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty" protobuf:"varint,4,opt,name=availableReplicas"`

	// Total number of unavailable machines targeted by this deployment.
	// This is the total number of machines that are still required for
	// the deployment to have 100% available capacity. They may either
	// be machines that are running but not yet available or machines
	// that still have not been created.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty" protobuf:"varint,5,opt,name=unavailableReplicas"`

	// Represents the latest available observations of a MachineDeployment's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []MachineDeploymentCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,6,rep,name=conditions"`

	// Count of hash collisions for the MachineDeployment. The MachineDeployment controller uses this
	// field as a collision avoidance mechanism when it needs to create the name for the
	// newest MachineSet.
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty" protobuf:"varint,8,opt,name=collisionCount"`

	// FailedMachines has summary of machines on which lastOperation Failed
	// +optional
	FailedMachines []*MachineSummary `json:"failedMachines,omitempty" protobuf:"bytes,9,rep,name=failedMachines"`
}

type MachineDeploymentConditionType string

// These are valid conditions of a MachineDeployment.
const (
	// Available means the MachineDeployment is available, ie. at least the minimum available
	// replicas required are up and running for at least minReadySeconds.
	MachineDeploymentAvailable MachineDeploymentConditionType = "Available"

	// Progressing means the MachineDeployment is progressing. Progress for a MachineDeployment is
	// considered when a new machine set is created or adopted, and when new machines scale
	// up or old machines scale down. Progress is not estimated for paused MachineDeployments or
	// when progressDeadlineSeconds is not specified.
	MachineDeploymentProgressing MachineDeploymentConditionType = "Progressing"

	// ReplicaFailure is added in a MachineDeployment when one of its machines fails to be created
	// or deleted.
	MachineDeploymentReplicaFailure MachineDeploymentConditionType = "ReplicaFailure"

	// MachineDeploymentFrozen is added in a MachineDeployment when one of its machines fails to be created
	// or deleted.
	MachineDeploymentFrozen MachineDeploymentConditionType = "Frozen"
)

// MachineDeploymentCondition describes the state of a MachineDeployment at a certain point.
type MachineDeploymentCondition struct {
	// Type of MachineDeployment condition.
	Type MachineDeploymentConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=MachineDeploymentConditionType"`

	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=corev1.ConditionStatus"`

	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,6,opt,name=lastUpdateTime"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,7,opt,name=lastTransitionTime"`

	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`

	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// Validate checks that an instance of MachineDeployment is well formed
func (MachineDeploymentValidationStrategy) Validate(ctx request.Context, obj runtime.Object) field.ErrorList {
	md := obj.(*cluster.MachineDeployment)
	log.Printf("Validating fields for MachineDeployment %s\n", md.Name)
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateMachineDeploymentSpec(&md.Spec, field.NewPath("spec"))...)
	return allErrs
}

func ValidateMachineDeploymentSpec(spec *cluster.MachineDeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, metav1validation.ValidateLabelSelector(spec.Selector, fldPath.Child("selector"))...)
	if len(spec.Selector.MatchLabels)+len(spec.Selector.MatchExpressions) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), spec.Selector, "empty selector is not valid for MachineSet."))
	}
	selector, err := metav1.LabelSelectorAsSelector(spec.Selector)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), spec.Selector, "invalid label selector."))
	} else {
		labels := labels.Set(spec.Template.Labels)
		if !selector.Matches(labels) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("template", "metadata", "labels"), spec.Template.Labels, "`selector` does not match template `labels`"))
		}
	}

	if spec.Replicas == nil || *spec.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), *spec.Replicas, "replicas must be specified and can not be negative"))
	}

	allErrs = append(allErrs, ValidateMachineDeploymentStrategy(&spec.Strategy, fldPath.Child("strategy"))...)
	return allErrs
}

func ValidateMachineDeploymentStrategy(strategy *cluster.MachineDeploymentStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	switch strategy.Type {
	case common.RollingUpdateMachineDeploymentStrategyType:
		if strategy.RollingUpdate != nil {
			allErrs = append(allErrs, ValidateMachineRollingUpdateDeployment(strategy.RollingUpdate, fldPath.Child("rollingUpdate"))...)
		}
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Type"), strategy.Type, "is an invalid type"))
	}
	return allErrs
}

func ValidateMachineRollingUpdateDeployment(rollingUpdate *cluster.MachineRollingUpdateDeployment, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	var maxUnavailable int
	var maxSurge int

	if rollingUpdate.MaxUnavailable != nil {
		allErrs = append(allErrs, ValidatePositiveIntOrPercent(rollingUpdate.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
		maxUnavailable, _ = getIntOrPercent(rollingUpdate.MaxUnavailable, false)

		// Validate that MaxUnavailable is not more than 100%.
		if len(utilvalidation.IsValidPercent(rollingUpdate.MaxUnavailable.StrVal)) == 0 && maxUnavailable > 100 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdate.MaxUnavailable, "should not be more than 100%"))
		}
	}

	if rollingUpdate.MaxSurge != nil {
		allErrs = append(allErrs, ValidatePositiveIntOrPercent(rollingUpdate.MaxSurge, fldPath.Child("maxSurge"))...)
		maxSurge, _ = getIntOrPercent(rollingUpdate.MaxSurge, true)
	}

	if rollingUpdate.MaxUnavailable != nil && rollingUpdate.MaxSurge != nil && maxUnavailable == 0 && maxSurge == 0 {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdate.MaxUnavailable, "may not be 0 when `maxSurge` is 0"))
	}

	return allErrs
}

func ValidatePositiveIntOrPercent(s *intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if x, err := getIntOrPercent(s, false); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, s.StrVal, "value should be int(5) or percentage(5%)"))
	} else if x < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, x, "value should not be negative"))
	}
	return allErrs
}

func getIntOrPercent(s *intstr.IntOrString, roundUp bool) (int, error) {
	return intstr.GetValueFromIntOrPercent(s, 100, roundUp)
}

// DefaultingFunction sets default MachineDeployment field values
func (MachineDeploymentSchemeFns) DefaultingFunction(o interface{}) {
	obj := o.(*MachineDeployment)
	// set default field values here
	log.Printf("Defaulting fields for MachineDeployment %s\n", obj.Name)
	if obj.Spec.Replicas == nil {
		obj.Spec.Replicas = new(int32)
		*obj.Spec.Replicas = 1
	}

	if obj.Spec.MinReadySeconds == nil {
		obj.Spec.MinReadySeconds = new(int32)
		*obj.Spec.MinReadySeconds = 0
	}

	if obj.Spec.RevisionHistoryLimit == nil {
		obj.Spec.RevisionHistoryLimit = new(int32)
		*obj.Spec.RevisionHistoryLimit = 1
	}

	if obj.Spec.ProgressDeadlineSeconds == nil {
		obj.Spec.ProgressDeadlineSeconds = new(int32)
		*obj.Spec.ProgressDeadlineSeconds = 600
	}

	if obj.Spec.Strategy.Type == "" {
		obj.Spec.Strategy.Type = common.RollingUpdateMachineDeploymentStrategyType
	}

	// Default RollingUpdate strategy only if strategy type is RollingUpdate.
	if obj.Spec.Strategy.Type == common.RollingUpdateMachineDeploymentStrategyType {
		if obj.Spec.Strategy.RollingUpdate == nil {
			obj.Spec.Strategy.RollingUpdate = &MachineRollingUpdateDeployment{}
		}

		if obj.Spec.Strategy.RollingUpdate.MaxSurge == nil {
			x := intstr.FromInt(1)
			obj.Spec.Strategy.RollingUpdate.MaxSurge = &x
		}

		if obj.Spec.Strategy.RollingUpdate.MaxUnavailable == nil {
			x := intstr.FromInt(0)
			obj.Spec.Strategy.RollingUpdate.MaxUnavailable = &x
		}
	}

	if len(obj.Namespace) == 0 {
		obj.Namespace = metav1.NamespaceDefault
	}
}
