// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package driver contains a fake driver implementation
package driver

import (
	"context"
	"fmt"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
)

// VMs is the map to hold the VM data
type VMs map[string]string

// FakeDriver is a fake driver returned when none of the actual drivers match
type FakeDriver struct {
	VMExists       bool
	ProviderID     string
	NodeName       string
	LastKnownState string
	Err            error
	fakeVMs        VMs
}

// NewFakeDriver returns a new fakedriver object
func NewFakeDriver(vmExists bool, providerID, nodeName, lastKnownState string, err error, fakeVMs VMs) Driver {
	fakeDriver := &FakeDriver{
		VMExists:       vmExists,
		ProviderID:     providerID,
		NodeName:       nodeName,
		LastKnownState: lastKnownState,
		Err:            err,
		fakeVMs:        make(VMs),
	}
	if providerID != "" && nodeName != "" {
		_ = fakeDriver.AddMachine(providerID, nodeName)
	}
	return fakeDriver
}

// AddMachine makes a call to the driver to create the machine.
func (d *FakeDriver) AddMachine(machineID, machineName string) error {
	d.fakeVMs[machineID] = machineName
	return nil
}

// CreateMachine makes a call to the driver to create the machine.
func (d *FakeDriver) CreateMachine(ctx context.Context, createMachineRequest *CreateMachineRequest) (*CreateMachineResponse, error) {
	if d.Err == nil {
		d.VMExists = true
		return &CreateMachineResponse{
			ProviderID:     d.ProviderID,
			NodeName:       d.NodeName,
			LastKnownState: d.LastKnownState,
		}, nil
	}

	return nil, d.Err
}

// InitializeMachine makes a call to the driver to initialize the VM instance of machine.
func (d *FakeDriver) InitializeMachine(ctx context.Context, initMachineRequest *InitializeMachineRequest) (*InitializeMachineResponse, error) {
	sErr, ok := status.FromError(d.Err)
	if ok && sErr != nil {
		switch sErr.Code() {
		case codes.NotFound:
			d.VMExists = false
			return nil, d.Err
		case codes.Unimplemented:
			break
		default:
			return nil, d.Err
		}
	}
	d.VMExists = true
	return &InitializeMachineResponse{
		ProviderID: d.ProviderID,
		NodeName:   d.NodeName,
	}, d.Err
}

// DeleteMachine make a call to the driver to delete the machine.
func (d *FakeDriver) DeleteMachine(ctx context.Context, deleteMachineRequest *DeleteMachineRequest) (*DeleteMachineResponse, error) {
	d.VMExists = false
	delete(d.fakeVMs, deleteMachineRequest.Machine.Spec.ProviderID)
	return &DeleteMachineResponse{
		LastKnownState: d.LastKnownState,
	}, d.Err
}

// GetMachineStatus makes a gRPC call to the driver to check existance of machine
func (d *FakeDriver) GetMachineStatus(ctx context.Context, getMachineStatusRequest *GetMachineStatusRequest) (*GetMachineStatusResponse, error) {
	switch {
	case !d.VMExists:
		errMessage := fmt.Sprintf("Fake plugin is returning no VM instances backing this machine object")
		return nil, status.Error(codes.NotFound, errMessage)
	case d.Err != nil:
		return nil, d.Err
	}
	return &GetMachineStatusResponse{
		ProviderID: d.ProviderID,
		NodeName:   d.NodeName,
	}, nil
}

// ListMachines have to list machines
func (d *FakeDriver) ListMachines(ctx context.Context, listMachinesRequest *ListMachinesRequest) (*ListMachinesResponse, error) {
	return &ListMachinesResponse{
		MachineList: d.fakeVMs,
	}, d.Err
}

// GetVolumeIDs returns a list of VolumeIDs for the PV spec list supplied
func (d *FakeDriver) GetVolumeIDs(ctx context.Context, getVolumeIDs *GetVolumeIDsRequest) (*GetVolumeIDsResponse, error) {
	return &GetVolumeIDsResponse{
		VolumeIDs: []string{},
	}, d.Err
}

// GenerateMachineClassForMigration converts providerMachineClass to (generic)MachineClass
func (d *FakeDriver) GenerateMachineClassForMigration(ctx context.Context, req *GenerateMachineClassForMigrationRequest) (*GenerateMachineClassForMigrationResponse, error) {
	req.MachineClass.Provider = "FakeProvider"
	return &GenerateMachineClassForMigrationResponse{}, d.Err
}
