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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	cluster "github.com/gardener/machine-controller-manager/pkg/apis/cluster"
	clustercommon "github.com/gardener/machine-controller-manager/pkg/apis/cluster/common"
)

// Finalizer is set on PreareForCreate callback
const MachineFinalizer string = "machine.cluster.k8s.io"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Machine
// +k8s:openapi-gen=true
// +resource:path=machines,strategy=MachineStrategy
type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

// MachineSpec defines the desired state of Machine
type MachineSpec struct {
	// This ObjectMeta will autopopulate the Node created. Use this to
	// indicate what labels, annotations, name prefix, etc., should be used
	// when creating the Node.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The full, authoritative list of taints to apply to the corresponding
	// Node. This list will overwrite any modifications made to the Node on
	// an ongoing basis.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`

	// Provider-specific configuration to use during node creation.
	// +optional
	ProviderConfig ProviderConfig `json:"providerConfig"`

	// Versions of key software to use. This field is optional at cluster
	// creation time, and omitting the field indicates that the cluster
	// installation tool should select defaults for the user. These
	// defaults may differ based on the cluster installer, but the tool
	// should populate the values it uses when persisting Machine objects.
	// A Machine spec missing this field at runtime is invalid.
	// +optional
	Versions MachineVersionInfo `json:"versions,omitempty"`

	// To populate in the associated Node for dynamic kubelet config. This
	// field already exists in Node, so any updates to it in the Machine
	// spec will be automatially copied to the linked NodeRef from the
	// status. The rest of dynamic kubelet config support should then work
	// as-is.
	// +optional
	ConfigSource *corev1.NodeConfigSource `json:"configSource,omitempty"`
}

// MachineStatus defines the observed state of Machine
type MachineStatus struct {
	// If the corresponding Node exists, this will point to its object.
	// +optional
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	// When was this status last observed
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// The current versions of software on the corresponding Node (if it
	// exists). This is provided for a few reasons:
	//
	// 1) It is more convenient than checking the NodeRef, traversing it to
	//    the Node, and finding the appropriate field in Node.Status.NodeInfo
	//    (which uses different field names and formatting).
	// 2) It removes some of the dependency on the structure of the Node,
	//    so that if the structure of Node.Status.NodeInfo changes, only
	//    machine controllers need to be updated, rather than every client
	//    of the Machines API.
	// 3) There is no other simple way to check the ControlPlane
	//    version. A client would have to connect directly to the apiserver
	//    running on the target node in order to find out its version.
	// +optional
	Versions *MachineVersionInfo `json:"versions,omitempty"`

	// In the event that there is a terminal problem reconciling the
	// Machine, both ErrorReason and ErrorMessage will be set. ErrorReason
	// will be populated with a succinct value suitable for machine
	// interpretation, while ErrorMessage will contain a more verbose
	// string suitable for logging and human consumption.
	//
	// These fields should not be set for transitive errors that a
	// controller faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconcilation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorReason *clustercommon.MachineStatusError `json:"errorReason,omitempty"`
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// Provider-specific status.
	// It is recommended that providers maintain their
	// own versioned API types that should be
	// serialized/deserialized from this field.
	ProviderStatus *runtime.RawExtension `json:"providerStatus"`

	// Addresses is a list of addresses assigned to the machine. Queried from cloud provider, if available.
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// Conditions of this machine, same as node
	// FROMMCM
	// +optional
	Conditions []corev1.NodeCondition `json:"conditions,omitempty"`

	// Last operation refers to the status of the last operation performed
	// FROMMCM
	// +optional
	LastOperation LastOperation `json:"lastOperation,omitempty"`

	// Current status of the machine object
	// +optional
	CurrentStatus CurrentStatus `json:"currentStatus,omitempty"`
}

// LastOperation suggests the last operation performed on the object
type LastOperation struct {
	// Description of the current operation
	Description string `json:"description,omitempty"`

	// Last update time of current operation
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// State of operation
	State MachineState `json:"state,omitempty"`

	// Type of operation
	Type MachineOperationType `json:"type,omitempty"`
}

// MachineState is the State of the Machine currently.
type MachineState string

// These are the valid statuses of machines.
const (
	// MachineStateProcessing means there are operations pending on this machine state
	MachineStateProcessing MachineState = "Processing"

	// MachineStateFailed means operation failed leading to machine status failure
	MachineStateFailed MachineState = "Failed"

	// MachineStateSuccessful indicates that the node is not ready at the moment
	MachineStateSuccessful MachineState = "Successful"
)

// MachineOperationType is a label for the operation performed on a machine object.
type MachineOperationType string

// These are the valid statuses of machines.
const (
	// MachineOperationCreate indicates that the operation was a create
	MachineOperationCreate MachineOperationType = "Create"

	// MachineOperationUpdate indicates that the operation was an update
	MachineOperationUpdate MachineOperationType = "Update"

	// MachineOperationHealthCheck indicates that the operation was a create
	MachineOperationHealthCheck MachineOperationType = "HealthCheck"

	// MachineOperationDelete indicates that the operation was a create
	MachineOperationDelete MachineOperationType = "Delete"
)

//type CurrentStatus
type CurrentStatus struct {
	// API group to which it belongs
	Phase MachinePhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase,casttype=MachinePhase"`

	// Name of machine class
	TimeoutActive bool `json:"timeoutActive,omitempty"`

	// Last update time of current status
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

// MachinePhase is a label for the condition of a machines at the current time.
type MachinePhase string

// These are the valid statuses of machines.
const (
	// MachinePending means that the machine is being created
	MachinePending MachinePhase = "Pending"

	// MachineAvailable means that machine is present on provider but hasn't joined cluster yet
	MachineAvailable MachinePhase = "Available"

	// MachineRunning means node is ready and running succesfully
	MachineRunning MachinePhase = "Running"

	// MachineRunning means node is terminating
	MachineTerminating MachinePhase = "Terminating"

	// MachineUnknown indicates that the node is not ready at the movement
	MachineUnknown MachinePhase = "Unknown"

	// MachineFailed means operation failed leading to machine status failure
	MachineFailed MachinePhase = "Failed"
)

type MachineVersionInfo struct {
	// Semantic version of kubelet to run
	Kubelet string `json:"kubelet"`

	// Semantic version of the Kubernetes control plane to
	// run. This should only be populated when the machine is a
	// master.
	// +optional
	ControlPlane string `json:"controlPlane,omitempty"`
}

// MachineSummary store the summary of machine.
type MachineSummary struct {
	// Name of the machine object
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// ProviderID represents the provider's unique ID given to a machine
	ProviderID string `json:"providerID,omitempty"`

	// Last operation refers to the status of the last operation performed
	LastOperation LastOperation `json:"lastOperation,omitempty"`

	// OwnerRef
	OwnerRef string `json:"ownerRef,omitempty"`
}

// Validate checks that an instance of Machine is well formed
func (MachineStrategy) Validate(ctx request.Context, obj runtime.Object) field.ErrorList {
	machine := obj.(*cluster.Machine)
	log.Printf("Validating fields for Machine %s\n", machine.Name)
	errors := field.ErrorList{}
	// perform validation here and add to errors using field.Invalid
	return errors
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (m MachineStrategy) PrepareForCreate(ctx request.Context, obj runtime.Object) {
	// Invoke the parent implementation to strip the Status
	m.DefaultStorageStrategy.PrepareForCreate(ctx, obj)

	// Cast the element and set finalizer
	o := obj.(*cluster.Machine)
	o.ObjectMeta.Finalizers = append(o.ObjectMeta.Finalizers, MachineFinalizer)
}

// DefaultingFunction sets default Machine field values
func (MachineSchemeFns) DefaultingFunction(o interface{}) {
	obj := o.(*Machine)
	// set default field values here
	log.Printf("Defaulting fields for Machine %s\n", obj.Name)
}
