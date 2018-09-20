/*
Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.

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

package infraclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MachineClassMeta has metadata about the machine class.
type MachineClassMeta struct {
	Name     string
	Revision int32
}

// SecretMeta has metadata about the machine class.
type SecretMeta struct {
	SecretName      string
	SecretNameSpace string
	Revision        int32
}

// MachineClassDataProvider is the interface an ExternalDriverProvider implementation
// can use to access machine class data.
type MachineClassDataProvider interface {
	GetMachineClass(machineClassMeta *MachineClassMeta) (interface{}, error)
	GetSecret(SecretMeta *SecretMeta) (string, error)
}

// ExternalDriverProvider interface must be implemented by the providers.
type ExternalDriverProvider interface {
	GetMachineClassType(machineClassDataProvider MachineClassDataProvider) metav1.TypeMeta
	Create(machineclass *MachineClassMeta, credentials, machineID, machineName string) (string, string, error)
	Delete(machineclass *MachineClassMeta, credentials, machineID string) error
	List(machineclass *MachineClassMeta, credentials, machineID string) (map[string]string, error)
}
