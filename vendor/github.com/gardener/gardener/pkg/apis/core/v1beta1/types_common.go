// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ErrorCode is a string alias.
type ErrorCode string

const (
	// ErrorInfraUnauthorized indicates that the last error occurred due to invalid infrastructure credentials.
	ErrorInfraUnauthorized ErrorCode = "ERR_INFRA_UNAUTHORIZED"
	// ErrorInfraInsufficientPrivileges indicates that the last error occurred due to insufficient infrastructure privileges.
	ErrorInfraInsufficientPrivileges ErrorCode = "ERR_INFRA_INSUFFICIENT_PRIVILEGES"
	// ErrorInfraQuotaExceeded indicates that the last error occurred due to infrastructure quota limits.
	ErrorInfraQuotaExceeded ErrorCode = "ERR_INFRA_QUOTA_EXCEEDED"
	// ErrorInfraDependencies indicates that the last error occurred due to dependent objects on the infrastructure level.
	ErrorInfraDependencies ErrorCode = "ERR_INFRA_DEPENDENCIES"
	// ErrorInfraResourcesDepleted indicates that the last error occurred due to depleted resource in the infrastructure.
	ErrorInfraResourcesDepleted ErrorCode = "ERR_INFRA_RESOURCES_DEPLETED"
	// ErrorCleanupClusterResources indicates that the last error occurred due to resources in the cluster that are stuck in deletion.
	ErrorCleanupClusterResources ErrorCode = "ERR_CLEANUP_CLUSTER_RESOURCES"
	// ErrorConfigurationProblem indicates that the last error occurred due to a configuration problem.
	ErrorConfigurationProblem ErrorCode = "ERR_CONFIGURATION_PROBLEM"
)

// LastError indicates the last occurred error for an operation on a resource.
type LastError struct {
	// A human readable message indicating details about the last error.
	Description string `json:"description" protobuf:"bytes,1,opt,name=description"`
	// ID of the task which caused this last error
	// +optional
	TaskID *string `json:"taskID,omitempty" protobuf:"bytes,2,opt,name=taskID"`
	// Well-defined error codes of the last error(s).
	// +optional
	Codes []ErrorCode `json:"codes,omitempty" protobuf:"bytes,3,rep,name=codes,casttype=ErrorCode"`
	// Last time the error was reported
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,4,opt,name=lastUpdateTime"`
}

// LastOperationType is a string alias.
type LastOperationType string

const (
	// LastOperationTypeCreate indicates a 'create' operation.
	LastOperationTypeCreate LastOperationType = "Create"
	// LastOperationTypeReconcile indicates a 'reconcile' operation.
	LastOperationTypeReconcile LastOperationType = "Reconcile"
	// LastOperationTypeDelete indicates a 'delete' operation.
	LastOperationTypeDelete LastOperationType = "Delete"
	// LastOperationTypeMigrate indicates a 'migrate' operation.
	LastOperationTypeMigrate LastOperationType = "Migrate"
	// LastOperationTypeRestore indicates a 'restore' operation.
	LastOperationTypeRestore LastOperationType = "Restore"
)

// LastOperationState is a string alias.
type LastOperationState string

const (
	// LastOperationStateProcessing indicates that an operation is ongoing.
	LastOperationStateProcessing LastOperationState = "Processing"
	// LastOperationStateSucceeded indicates that an operation has completed successfully.
	LastOperationStateSucceeded LastOperationState = "Succeeded"
	// LastOperationStateError indicates that an operation is completed with errors and will be retried.
	LastOperationStateError LastOperationState = "Error"
	// LastOperationStateFailed indicates that an operation is completed with errors and won't be retried.
	LastOperationStateFailed LastOperationState = "Failed"
	// LastOperationStatePending indicates that an operation cannot be done now, but will be tried in future.
	LastOperationStatePending LastOperationState = "Pending"
	// LastOperationStateAborted indicates that an operation has been aborted.
	LastOperationStateAborted LastOperationState = "Aborted"
)

// LastOperation indicates the type and the state of the last operation, along with a description
// message and a progress indicator.
type LastOperation struct {
	// A human readable message indicating details about the last operation.
	Description string `json:"description" protobuf:"bytes,1,opt,name=description"`
	// Last time the operation state transitioned from one to another.
	LastUpdateTime metav1.Time `json:"lastUpdateTime" protobuf:"bytes,2,opt,name=lastUpdateTime"`
	// The progress in percentage (0-100) of the last operation.
	Progress int32 `json:"progress" protobuf:"varint,3,opt,name=progress"`
	// Status of the last operation, one of Aborted, Processing, Succeeded, Error, Failed.
	State LastOperationState `json:"state" protobuf:"bytes,4,opt,name=state,casttype=LastOperationState"`
	// Type of the last operation, one of Create, Reconcile, Delete.
	Type LastOperationType `json:"type" protobuf:"bytes,5,opt,name=type,casttype=LastOperationType"`
}

// Gardener holds the information about the Gardener version that operated a resource.
type Gardener struct {
	// ID is the Docker container id of the Gardener which last acted on a resource.
	ID string `json:"id" protobuf:"bytes,1,opt,name=id"`
	// Name is the hostname (pod name) of the Gardener which last acted on a resource.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// Version is the version of the Gardener which last acted on a resource.
	Version string `json:"version" protobuf:"bytes,3,opt,name=version"`
}

const (
	// GardenerName is the value in a Garden resource's `.metadata.finalizers[]` array on which the Gardener will react
	// when performing a delete request on a resource.
	GardenerName = "gardener"
	// ExternalGardenerName is the value in a Kubernetes core resources `.metadata.finalizers[]` array on which the
	// Gardener will react when performing a delete request on a resource.
	ExternalGardenerName = "gardener.cloud/gardener"
	// ExternalGardenerNameDeprecated is the value in a Kubernetes core resources `.metadata.finalizers[]` array on which the
	// Gardener will react when performing a delete request on a resource.
	//
	// Deprecated: Use `ExternalGardenerName` instead.
	ExternalGardenerNameDeprecated = "garden.sapcloud.io/gardener"
)

const (
	// EventReconciling indicates that the a Reconcile operation started.
	EventReconciling = "Reconciling"
	// EventReconciled indicates that the a Reconcile operation was successful.
	EventReconciled = "Reconciled"
	// EventReconcileError indicates that the a Reconcile operation failed.
	EventReconcileError = "ReconcileError"
	// EventDeleting indicates that the a Delete operation started.
	EventDeleting = "Deleting"
	// EventDeleted indicates that the a Delete operation was successful.
	EventDeleted = "Deleted"
	// EventDeleteError indicates that the a Delete operation failed.
	EventDeleteError = "DeleteError"
	// EventPrepareMigration indicates that a Prepare Migration operation started.
	EventPrepareMigration = "PrepareMigration"
	// EventMigrationPrepared indicates that Migration preparation was successful.
	EventMigrationPrepared = "MigrationPrepared"
	// EventMigrationPreparationFailed indicates that Migration preparation failed.
	EventMigrationPreparationFailed = "MigrationPreparationFailed"
)
