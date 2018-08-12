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

import runtime "k8s.io/apimachinery/pkg/runtime"

// ProviderConfig defines the configuration to use during node creation.
type ProviderConfig struct {

	// No more than one of the following may be specified.

	// Value is an inlined, serialized representation of the resource
	// configuration. It is recommended that providers maintain their own
	// versioned API types that should be serialized/deserialized from this
	// field, akin to component config.
	// +optional
	Value *runtime.RawExtension `json:"value,omitempty"`

	// Source for the provider configuration. Cannot be used if value is
	// not empty.
	// +optional
	ValueFrom *ProviderConfigSource `json:valueFrom,omitempty`
}

// ProviderConfigSource represents a source for the provider-specific
// resource configuration.
type ProviderConfigSource struct {
	// TODO(roberthbailey): Fill these in later
	// No more than one of the following may be specified.
	// The machine class from which the provider config should be sourced.
	MachineClass *MachineClassRef `json:machineClass,omitempty`
}

type MachineClassRef struct {
	// The name of the MachineClass.
	Name string `json:name`
	// Parameters allow basic substitution to be applied to
	// a MachineClass (where supported).
	// Keys must not be empty. The maximum number of
	// parameters is 512, with a cumulative max size of 256K.
	// +optional
	Parameters map[string]string `json:parameters,omitempty`
}

// The below types are used by kube_client and api_server.

type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition;
// "ConditionFalse" means a resource is not in the condition; "ConditionUnknown" means kubernetes
// can't decide if a resource is in the condition or not. In the future, we could add other
// intermediate conditions, e.g. ConditionDegraded.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)
