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
	"github.com/gardener/machine-controller-manager/pkg/driver/grpc/server"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Driver is the common interface for creation/deletion of the VMs over different cloud-providers.
type Driver interface {
	Create() (string, string, error)
	Delete() error
	GetExisting() (string, error)
	GetVMs(string) (VMs, error)
}

// VMs maintains a list of VM returned by the provider
// Key refers to the machine-id on the cloud provider
// value refers to the machine-name of the machine object
type VMs map[string]string

// ExternalDriverManager is the central manager of all the external drivers.
var ExternalDriverManager *server.ExternalDriverManager

// NewDriver creates a new driver object based on the classKind
func NewDriver(machineID string, secretRef *corev1.Secret, class *v1alpha1.ClassSpec, machineClass interface{}, machineName string) Driver {

	if ExternalDriverManager != nil {
		external, err := ExternalDriverManager.GetDriver(metav1.TypeMeta{
			// TODO: but here class (coming from machine.spec.class) doesn't have APIGroup populated. This needs to be fixed in Gardener
			APIVersion: class.APIGroup,
			Kind:       class.Kind,
		})
		if err == nil {
			// TODO: Currently the name of the machine class needs to be absolute path, which will be used in GetMachineClass() in server package
			// Once the machineClass object is standardized for all providers, this will not be required
			name := "/" + "apis" + "/" + class.APIGroup + "/" + "namespaces" + "/" + secretRef.GetNamespace() + "/" + strings.ToLower(class.Kind) + "es" + "/" + class.Name
			return NewExternalDriver(external, name, secretRef, string(secretRef.Data["userData"]), machineID, machineName)
		}
	}

	switch class.Kind {
	case "OpenStackMachineClass":
		return &OpenStackDriver{
			OpenStackMachineClass: machineClass.(*v1alpha1.OpenStackMachineClass),
			CloudConfig:           secretRef,
			UserData:              string(secretRef.Data["userData"]),
			MachineID:             machineID,
			MachineName:           machineName,
		}

	case "AWSMachineClass":
		return &AWSDriver{
			AWSMachineClass: machineClass.(*v1alpha1.AWSMachineClass),
			CloudConfig:     secretRef,
			UserData:        string(secretRef.Data["userData"]),
			MachineID:       machineID,
			MachineName:     machineName,
		}

	case "AzureMachineClass":
		return &AzureDriver{
			AzureMachineClass: machineClass.(*v1alpha1.AzureMachineClass),
			CloudConfig:       secretRef,
			UserData:          string(secretRef.Data["userData"]),
			MachineID:         machineID,
			MachineName:       machineName,
		}

	case "GCPMachineClass":
		return &GCPDriver{
			GCPMachineClass: machineClass.(*v1alpha1.GCPMachineClass),
			CloudConfig:     secretRef,
			UserData:        string(secretRef.Data["userData"]),
			MachineID:       machineID,
			MachineName:     machineName,
		}

	case "AlicloudMachineClass":
		return &AlicloudDriver{
			AlicloudMachineClass: machineClass.(*v1alpha1.AlicloudMachineClass),
			CloudConfig:          secretRef,
			UserData:             string(secretRef.Data["userData"]),
			MachineID:            machineID,
			MachineName:          machineName,
		}
	}

	if ExternalDriverManager != nil {
		external, err := ExternalDriverManager.GetDriver(metav1.TypeMeta{
			// TODO: but here class (coming from machine.spec.class) doesn't have APIGroup populated
			APIVersion: class.APIGroup,
			Kind:       class.Kind,
		})
		if err == nil {
			return NewExternalDriver(external, class.Name, secretRef, string(secretRef.Data["userData"]), machineID, machineName)
		}
	}

	return NewFakeDriver(
		func() (string, string, error) {
			return "fake", "fake_ip", nil
		},
		func() error {
			return nil
		},
		func() (string, error) {
			return "fake", nil
		},
	)
}
