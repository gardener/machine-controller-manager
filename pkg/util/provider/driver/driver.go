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
	"context"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	corev1 "k8s.io/api/core/v1"
)

// Driver is the common interface for creation/deletion of the VMs over different cloud-providers.
type Driver interface {
	CreateMachine(context.Context, *CreateMachineRequest) (*CreateMachineResponse, error)
	DeleteMachine(context.Context, *DeleteMachineRequest) (*DeleteMachineResponse, error)
	GetMachineStatus(context.Context, *GetMachineStatusRequest) (*GetMachineStatusResponse, error)
	ListMachines(context.Context, *ListMachinesRequest) (*ListMachinesResponse, error)
	GetVolumeIDs(context.Context, *GetVolumeIDsRequest) (*GetVolumeIDsResponse, error)
}

// CreateMachineRequest is the create request for VM creation
type CreateMachineRequest struct {
	// Machine object from whom VM is to be created
	Machine *v1alpha1.Machine

	// MachineClass backing the machine object
	MachineClass *v1alpha1.MachineClass

	//  Secret backing the machineClass object
	Secret *corev1.Secret
}

// CreateMachineResponse is the create response for VM creation
type CreateMachineResponse struct {
	// ProviderID is the unique identification of the VM at the cloud provider.
	// ProviderID typically matches with the node.Spec.ProviderID on the node object.
	// Eg: gce://project-name/region/vm-ID
	ProviderID string

	// NodeName is the name of the node-object registered to kubernetes.
	NodeName string

	// LastKnownState represents the last state of the VM during an creation/deletion error
	LastKnownState string

	// Retry specifies if operation has to be retried
	Retry machineutils.Retry
}

// DeleteMachineRequest is the delete request for VM deletion
type DeleteMachineRequest struct {
	// Machine object from whom VM is to be deleted
	Machine *v1alpha1.Machine

	// MachineClass backing the machine object
	MachineClass *v1alpha1.MachineClass

	// Secret backing the machineClass object
	Secret *corev1.Secret
}

// DeleteMachineResponse is the delete response for VM deletion
type DeleteMachineResponse struct {
	// LastKnownState represents the last state of the VM during an creation/deletion error
	LastKnownState string

	// Retry specifies if operation has to be retried
	Retry machineutils.Retry
}

// GetMachineStatusRequest is the get request for VM info
type GetMachineStatusRequest struct {
	// Machine object from whom VM status is to be fetched
	Machine *v1alpha1.Machine

	// MachineClass backing the machine object
	MachineClass *v1alpha1.MachineClass

	//  Secret backing the machineClass object
	Secret *corev1.Secret
}

// GetMachineStatusResponse is the get response for VM info
type GetMachineStatusResponse struct {
	// ProviderID is the unique identification of the VM at the cloud provider.
	// ProviderID typically matches with the node.Spec.ProviderID on the node object.
	// Eg: gce://project-name/region/vm-ID
	ProviderID string

	// NodeName is the name of the node-object registered to kubernetes.
	NodeName string

	// Retry specifies if operation has to be retried
	Retry machineutils.Retry
}

// ListMachinesRequest is the request object to get a list of VMs belonging to a machineClass
type ListMachinesRequest struct {
	// MachineClass object
	MachineClass *v1alpha1.MachineClass

	// Secret backing the machineClass object
	Secret *corev1.Secret
}

// ListMachinesResponse is the response object of the list of VMs belonging to a machineClass
type ListMachinesResponse struct {
	// MachineList is the map of list of machines. Format for the map should be <ProviderID, MachineName>.
	MachineList map[string]string

	// Retry specifies if operation has to be retried
	Retry machineutils.Retry
}

// GetVolumeIDsRequest is the request object to get a list of VolumeIDs for a PVSpec
type GetVolumeIDsRequest struct {
	// PVSpecsList is a list of PV specs for whom volume-IDs are required
	// Plugin should parse this raw data into pre-defined list of PVSpecs
	PVSpecs []*corev1.PersistentVolumeSpec
}

// GetVolumeIDsResponse is the response object of the list of VolumeIDs for a PVSpec
type GetVolumeIDsResponse struct {
	// VolumeIDs is a list of VolumeIDs.
	VolumeIDs []string

	// Retry specifies if operation has to be retried
	Retry machineutils.Retry
}
