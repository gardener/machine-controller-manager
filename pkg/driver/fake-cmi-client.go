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
	corev1 "k8s.io/api/core/v1"
)

// FakeCMIDriverClient is a fake driver returned when none of the actual drivers match
type FakeCMIDriverClient struct {
	MachineID string
	NodeName  string
	Err       error
}

// NewFakeDriver returns a new fakedriver object
func NewFakeDriver(machineID string, nodeName string, err error) *FakeCMIDriverClient {
	return &FakeCMIDriverClient{
		MachineID: machineID,
		NodeName:  nodeName,
		Err:       err,
	}
}

// CreateMachine makes a gRPC call to the driver to create the machine.
func (c *FakeCMIDriverClient) CreateMachine() (string, string, error) {
	return c.MachineID, c.NodeName, c.Err
}

// DeleteMachine make a grpc call to the driver to delete the machine.
func (c *FakeCMIDriverClient) DeleteMachine(MachineID string) error {
	return c.Err
}

// GetMachine makes a gRPC call to the driver to check existance of machine
func (c *FakeCMIDriverClient) GetMachine(MachineID string) (bool, error) {
	if c.Err == nil {
		return true, nil
	}

	return false, c.Err
}

// ListMachines have to list machines
func (c *FakeCMIDriverClient) ListMachines() (map[string]string, error) {
	var mapOfMachines map[string]string
	return mapOfMachines, c.Err
}

// ShutDownMachine implements shutdownmachine
func (c *FakeCMIDriverClient) ShutDownMachine(MachineID string) error {
	return c.Err
}

// GetMachineID returns the machineID
func (c *FakeCMIDriverClient) GetMachineID() string {
	return c.MachineID
}

// GetListOfVolumeIDsForExistingPVs returns a list of VolumeIDs for the PV spec list supplied
func (c *FakeCMIDriverClient) GetListOfVolumeIDsForExistingPVs(pvSpecs []*corev1.PersistentVolumeSpec) ([]string, error) {
	return []string{}, c.Err
}
