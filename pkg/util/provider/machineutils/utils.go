/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package machineutils contains the consts and global vaariables for machine operation
package machineutils

const (
	// GetVMStatus sets machine status to terminating and specifies next step as getting VMs
	GetVMStatus = "Set machine status to termination. Now, getting VM Status"
	// InitiateDrain specifies next step as initiate node drain
	InitiateDrain = "Initiate node drain"
	// InitiateVMDeletion specifies next step as initiate VM deletion
	InitiateVMDeletion = "Initiate VM deletion"
	// InitiateNodeDeletion specifies next step as node object deletion
	InitiateNodeDeletion = "Initiate node object deletion"
	// InitiateFinalizerRemoval specifies next step as machine finalizer removal
	InitiateFinalizerRemoval = "Initiate machine object finalizer removal"

	// LastAppliedALTAnnotation contains the last configuration of annotations, labels & taints applied on the node object
	LastAppliedALTAnnotation = "node.machine.sapcloud.io/last-applied-anno-labels-taints"
	// MachinePriority is the annotation used to specify priority
	// associated with a machine while deleting it. The less its
	// priority the more likely it is to be deleted first
	// Default priority for a machine is set to 3
	MachinePriority = "machinepriority.machine.sapcloud.io"
)

// Retry is a label for retrying operation
type Retry bool

// These are the valid values for Retry.
const (
	// RetryOp tells the controller to retry
	RetryOp Retry = true
	// DoNotRetryOp tells the controller to not retry for now. Resync after re-sync period
	DoNotRetryOp Retry = false
)
