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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Quota struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec defines the Quota constraints.
	// +optional
	Spec QuotaSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuotaList is a collection of Quotas.
type QuotaList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of Quotas.
	Items []Quota `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// QuotaSpec is the specification of a Quota.
type QuotaSpec struct {
	// ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.
	// +optional
	ClusterLifetimeDays *int32 `json:"clusterLifetimeDays,omitempty" protobuf:"varint,1,opt,name=clusterLifetimeDays"`
	// Metrics is a list of resources which will be put under constraints.
	Metrics corev1.ResourceList `json:"metrics" protobuf:"bytes,2,rep,name=metrics,casttype=k8s.io/api/core/v1.ResourceList,castkey=k8s.io/api/core/v1.ResourceName"`
	// Scope is the scope of the Quota object, either 'project' or 'secret'.
	Scope corev1.ObjectReference `json:"scope" protobuf:"bytes,3,opt,name=scope"`
}
