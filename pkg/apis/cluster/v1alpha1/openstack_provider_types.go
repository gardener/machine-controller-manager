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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/********************** OpenStackMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenStackMachineClass TODO
type OpenStackMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec OpenStackMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenStackMachineClassList is a collection of OpenStackMachineClasses.
type OpenStackMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []OpenStackMachineClass `json:"items"`
}

// OpenStackMachineClassSpec is the specification of a cluster.
type OpenStackMachineClassSpec struct {
	ImageName        string                  `json:"imageName"`
	Region           string                  `json:"region"`
	AvailabilityZone string                  `json:"availabilityZone"`
	FlavorName       string                  `json:"flavorName"`
	KeyName          string                  `json:"keyName"`
	SecurityGroups   []string                `json:"securityGroups"`
	Tags             map[string]string       `json:"tags,omitempty"`
	NetworkID        string                  `json:"networkID"`
	SecretRef        *corev1.SecretReference `json:"secretRef,omitempty"`
	PodNetworkCidr   string                  `json:"podNetworkCidr"`
}

const (
	// OpenStackAuthURL is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackAuthURL string = "authURL"
	// OpenStackCACert is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackCACert string = "caCert"
	// OpenStackInsecure is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackInsecure string = "insecure"
	// OpenStackDomainName is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackDomainName string = "domainName"
	// OpenStackTenantName is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackTenantName string = "tenantName"
	// OpenStackUsername is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackUsername string = "username"
	// OpenStackPassword is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackPassword string = "password"
	// OpenStackClientCert is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackClientCert string = "clientCert"
	// OpenStackClientKey is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackClientKey string = "clientKey"
)
