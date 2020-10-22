// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// BackupEntryForceDeletion is a constant for an annotation on a BackupEntry indicating that it should be force deleted.
	BackupEntryForceDeletion = "backupentry.core.gardener.cloud/force-deletion"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntry holds details about shoot backup.
type BackupEntry struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec contains the specification of the Backup Entry.
	Spec BackupEntrySpec
	// Status contains the most recently observed status of the Backup Entry.
	Status BackupEntryStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntryList is a list of BackupEntry objects.
type BackupEntryList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of BackupEntry.
	Items []BackupEntry
}

// BackupEntrySpec is the specification of a Backup Entry.
type BackupEntrySpec struct {
	// BucketName is the name of backup bucket for this Backup Entry.
	BucketName string
	// SeedName holds the name of the seed allocated to BackupBucket for running controller.
	SeedName *string
}

// BackupEntryStatus holds the most recently observed status of the Backup Entry.
type BackupEntryStatus struct {
	// LastOperation holds information about the last operation on the BackupEntry.
	LastOperation *LastOperation
	// LastError holds information about the last occurred error during an operation.
	LastError *LastError
	// ObservedGeneration is the most recent generation observed for this BackupEntry. It corresponds to the
	// BackupEntry's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
}
