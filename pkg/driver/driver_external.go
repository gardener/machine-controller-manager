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

// Package driver contains the cloud provider specific implementations to manage machines
package driver

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/machine-controller-manager/pkg/driver/grpc/server"
	"github.com/golang/glog"
)

// ExternalDriver implements the support for out-of-tree drivers
type ExternalDriver struct {
	driver       server.Driver
	machineClass string
	credentials  *corev1.Secret
	userData     string
	machineID    string
	machineName  string
}

// NewExternalDriver returns an empty AWSDriver object
func NewExternalDriver(driver server.Driver, machineClass string, credentials *corev1.Secret, userData, machineID, machineName string) Driver {
	return &ExternalDriver{
		driver:       driver,
		machineClass: machineClass,
		credentials:  nil, //TODO
		userData:     userData,
		machineID:    machineID,
		machineName:  machineName,
	}
}

// Create method is used to create a machine
func (d *ExternalDriver) Create() (string, string, error) {
	meta := server.MachineClassMeta{
		Name:     d.machineClass,
		Revision: 1,
		//TODO
		//Revision: machineclass.Metadata.ResourceVersion,
	}
	machineID, machineName, err := d.driver.Create(&meta, "", d.machineID, d.machineName)
	if err == nil {
		d.machineID = machineID
		d.machineName = machineName
	}
	return machineID, machineName, err
}

// Delete method is used to delete a AWS machine
func (d *ExternalDriver) Delete() error {
	meta := server.MachineClassMeta{
		Name:     d.machineClass,
		Revision: 1,
		//TODO
		//Revision: machineclass.Metadata.ResourceVersion,
	}
	result, err := d.GetVMs(d.machineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machine-ID
		glog.V(2).Infof("No VM matching the machine-ID found on the provider %q", d.machineID)
		return nil
	}

	//TODO
	err = d.driver.Delete(&meta, "", d.machineID)
	if err != nil {
		glog.Errorf("Could not terminate machine: %s", err.Error())
	}
	return err
}

// GetExisting method is used to get machineID for existing AWS machine
func (d *ExternalDriver) GetExisting() (string, error) {
	return d.machineID, nil
}

// GetVMs returns a VM matching the machineID
// If machineID is an empty string then it returns all matching instances
func (d *ExternalDriver) GetVMs(machineID string) (VMs, error) {
	meta := server.MachineClassMeta{
		Name:     d.machineClass,
		Revision: 1,
		//TODO
		//Revision: machineclass.Metadata.ResourceVersion,
	}
	result, err := d.driver.GetVMs(&meta, "", machineID)
	if err != nil {
		glog.Errorf("Could not get list of VMs. Error: %s", err.Error())
		return nil, err
	}
	return result, nil
}
