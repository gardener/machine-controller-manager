/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package driver contains the cloud provider specific implementations to manage machines
package driver

import (
	"strings"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

// Driver is the common interface for creation/deletion of the VMs over different cloud-providers.
type Driver interface {
	Create() (string, string, error)
	Delete(string) error
	GetExisting() (string, error)
	GetVMs(string) (VMs, error)
	GetVolNames([]corev1.PersistentVolumeSpec) ([]string, error)
	GetUserData() string
	SetUserData(string)
}

// VMs maintains a list of VM returned by the provider
// Key refers to the machine-id on the cloud provider
// value refers to the machine-name of the machine object
type VMs map[string]string

// NewDriver creates a new driver object based on the classKind
func NewDriver(machineID string, secretData map[string][]byte, classKind string, machineClass interface{}, machineName string) Driver {

	switch classKind {
	case "OpenStackMachineClass":
		return &OpenStackDriver{
			OpenStackMachineClass: machineClass.(*v1alpha1.OpenStackMachineClass),
			CredentialsData:       secretData,
			UserData:              string(secretData["userData"]),
			MachineID:             machineID,
			MachineName:           machineName,
		}

	case "AWSMachineClass":
		return &AWSDriver{
			AWSMachineClass: machineClass.(*v1alpha1.AWSMachineClass),
			CredentialsData: secretData,
			UserData:        string(secretData["userData"]),
			MachineID:       machineID,
			MachineName:     machineName,
		}

	case "AzureMachineClass":
		return &AzureDriver{
			AzureMachineClass: machineClass.(*v1alpha1.AzureMachineClass),
			CredentialsData:   secretData,
			UserData:          string(secretData["userData"]),
			MachineID:         machineID,
			MachineName:       machineName,
		}

	case "GCPMachineClass":
		return &GCPDriver{
			GCPMachineClass: machineClass.(*v1alpha1.GCPMachineClass),
			CredentialsData: secretData,
			UserData:        string(secretData["userData"]),
			MachineID:       machineID,
			MachineName:     machineName,
		}

	case "AlicloudMachineClass":
		return &AlicloudDriver{
			AlicloudMachineClass: machineClass.(*v1alpha1.AlicloudMachineClass),
			CredentialsData:      secretData,
			UserData:             string(secretData["userData"]),
			MachineID:            machineID,
			MachineName:          machineName,
		}
	case "PacketMachineClass":
		return &PacketDriver{
			PacketMachineClass: machineClass.(*v1alpha1.PacketMachineClass),
			CredentialsData:    secretData,
			UserData:           string(secretData["userData"]),
			MachineID:          machineID,
			MachineName:        machineName,
		}
	}

	return NewFakeDriver(
		func() (string, string, error) {
			fakeVMs["fake"] = "fake_ip"
			return "fake", "fake_ip", nil
		},
		func(machineID string, machineName string) error {
			fakeVMs[machineID] = machineName
			return nil
		},
		func(machineID string) error {
			// delete(fakeVMs, "fake")
			delete(fakeVMs, machineID)
			return nil
		},
		func() (string, error) {
			return "", nil
		},
		func() (VMs, error) {
			return fakeVMs, nil
		},
		func([]corev1.PersistentVolumeSpec) ([]string, error) {
			return []string{}, nil
		},
		func() string {
			return ""
		},
		func(string) {
			return
		},
	)
}

// ExtractCredentialsFromData extracts and trims a value from the given data map. The first key that exists is being
// returned, otherwise, the next key is tried, etc. If no key exists then an empty string is returned.
func ExtractCredentialsFromData(data map[string][]byte, keys ...string) string {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			return strings.TrimSpace(string(val))
		}
	}
	return ""
}
