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

	"github.com/golang/glog"
)

// ExternalDriver implements the support for out-of-tree drivers
type ExternalDriver struct {
	machineClass interface{}
	credentials  *corev1.Secret
	userData     string
	machineID    string
	machineName  string
}

// NewExternalDriver returns an empty AWSDriver object
func NewExternalDriver(machineClass interface{}, credentials *corev1.Secret, userData, machineID, machineName string) Driver {
	return &ExternalDriver{}
}

// Create method is used to create a machine
func (d *ExternalDriver) Create() (string, string, error) {
	return "", "", nil
}

// Delete method is used to delete a AWS machine
func (d *ExternalDriver) Delete() error {
	result, err := d.GetVMs(d.machineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machine-ID
		glog.V(2).Infof("No VM matching the machine-ID found on the provider %q", d.machineID)
		return nil
	}

	glog.Errorf("Could not terminate machine: %s", err.Error())
	return err
}

// GetExisting method is used to get machineID for existing AWS machine
func (d *ExternalDriver) GetExisting() (string, error) {
	return d.machineID, nil
}

// GetVMs returns a VM matching the machineID
// If machineID is an empty string then it returns all matching instances
func (d *ExternalDriver) GetVMs(machineID string) (VMs, error) {
	return nil, nil
}
