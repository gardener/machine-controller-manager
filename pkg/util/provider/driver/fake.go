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

// Package driver contains a fake driver implementation
package driver

import (
	"context"
	"fmt"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
)

// FakeDriver is a fake driver returned when none of the actual drivers match
type FakeDriver struct {
	VMExists       bool
	ProviderID     string
	NodeName       string
	LastKnownState string
	Err            error
}

// NewFakeDriver returns a new fakedriver object
func NewFakeDriver(fakeDriver *FakeDriver) Driver {
	return fakeDriver
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

// DeleteMachine make a call to the driver to delete the machine.
func (d *FakeDriver) DeleteMachine(ctx context.Context, deleteMachineRequest *DeleteMachineRequest) (*DeleteMachineResponse, error) {
	d.VMExists = false
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
		MachineList: map[string]string{},
	}, d.Err
}

// GetVolumeIDs returns a list of VolumeIDs for the PV spec list supplied
func (d *FakeDriver) GetVolumeIDs(ctx context.Context, getVolumeIDs *GetVolumeIDsRequest) (*GetVolumeIDsResponse, error) {
	return &GetVolumeIDsResponse{
		VolumeIDs: []string{},
	}, d.Err
}
