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

package core

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Project holds certain properties about a Gardener project.
type Project struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the project properties.
	Spec ProjectSpec
	// Most recently observed status of the Project.
	Status ProjectStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectList is a collection of Projects.
type ProjectList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Projects.
	Items []Project
}

// ProjectSpec is the specification of a Project.
type ProjectSpec struct {
	// CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
	// who created the project.
	CreatedBy *rbacv1.Subject
	// Description is a human-readable description of what the project is used for.
	Description *string
	// Owner is a subject representing a user name, an email address, or any other identifier of a user owning
	// the project.
	Owner *rbacv1.Subject
	// Purpose is a human-readable explanation of the project's purpose.
	Purpose *string
	// Members is a list of subjects representing a user name, an email address, or any other identifier of a user,
	// group, or service account that has a certain role.
	Members []ProjectMember
	// Namespace is the name of the namespace that has been created for the Project object.
	// A nil value means that Gardener will determine the name of the namespace.
	Namespace *string
	// Tolerations contains the default tolerations and a whitelist for taints on seed clusters.
	Tolerations *ProjectTolerations
}

// ProjectStatus holds the most recently observed status of the project.
type ProjectStatus struct {
	// ObservedGeneration is the most recent generation observed for this project.
	ObservedGeneration int64
	// Phase is the current phase of the project.
	Phase ProjectPhase
	// StaleSinceTimestamp contains the timestamp when the project was first discovered to be stale/unused.
	StaleSinceTimestamp *metav1.Time
	// StaleAutoDeleteTimestamp contains the timestamp when the project will be garbage-collected/automatically deleted
	// because it's stale/unused.
	StaleAutoDeleteTimestamp *metav1.Time
}

// ProjectMember is a member of a project.
type ProjectMember struct {
	// Subject is representing a user name, an email address, or any other identifier of a user, group, or service
	// account that has a certain role.
	rbacv1.Subject
	// Roles is a list of roles of this member.
	Roles []string
}

// ProjectTolerations contains the tolerations for taints on seed clusters.
type ProjectTolerations struct {
	// Defaults contains a list of tolerations that are added to the shoots in this project by default.
	Defaults []Toleration
	// Whitelist contains a list of tolerations that are allowed to be added to the shoots in this project. Please note
	// that this list may only be added by users having the `spec-tolerations-whitelist` verb for project resources.
	Whitelist []Toleration
}

// Toleration is a toleration for a seed taint.
type Toleration struct {
	// Key is the toleration key to be applied to a project or shoot.
	Key string
	// Value is the toleration value corresponding to the toleration key.
	Value *string
}

const (
	// ProjectMemberAdmin is a const for a role that provides full admin access.
	ProjectMemberAdmin = "admin"
	// ProjectMemberOwner is a const for a role that provides full owner access.
	ProjectMemberOwner = "owner"
	// ProjectMemberViewer is a const for a role that provides limited permissions to only view some resources.
	ProjectMemberViewer = "viewer"
	// ProjectMemberUserAccessManager is a const for a role that provides permissions to manage human user(s, (groups)).
	ProjectMemberUserAccessManager = "uam"
	// ProjectMemberExtensionPrefix is a prefix for custom roles that are not known by Gardener.
	ProjectMemberExtensionPrefix = "extension:"
)

// ProjectPhase is a label for the condition of a project at the current time.
type ProjectPhase string

const (
	// ProjectPending indicates that the project reconciliation is pending.
	ProjectPending ProjectPhase = "Pending"
	// ProjectReady indicates that the project reconciliation was successful.
	ProjectReady ProjectPhase = "Ready"
	// ProjectFailed indicates that the project reconciliation failed.
	ProjectFailed ProjectPhase = "Failed"
	// ProjectTerminating indicates that the project is in termination process.
	ProjectTerminating ProjectPhase = "Terminating"

	// ProjectEventNamespaceReconcileFailed indicates that the namespace reconciliation has failed.
	ProjectEventNamespaceReconcileFailed = "NamespaceReconcileFailed"
	// ProjectEventNamespaceReconcileSuccessful indicates that the namespace reconciliation has succeeded.
	ProjectEventNamespaceReconcileSuccessful = "NamespaceReconcileSuccessful"
	// ProjectEventNamespaceDeletionFailed indicates that the namespace deletion failed.
	ProjectEventNamespaceDeletionFailed = "NamespaceDeletionFailed"
	// ProjectEventNamespaceMarkedForDeletion indicates that the namespace has been successfully marked for deletion.
	ProjectEventNamespaceMarkedForDeletion = "NamespaceMarkedForDeletion"
)
