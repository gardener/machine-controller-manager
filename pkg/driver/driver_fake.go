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

import corev1 "k8s.io/api/core/v1"

// FakeDriver is a fake driver returned when none of the actual drivers match
type FakeDriver struct {
	create func() (string, string, error)
	delete func(string) error
	//existing func() (string, v1alpha1.MachinePhase, error)
	existing    func() (string, error)
	getVolNames func([]corev1.PersistentVolumeSpec) ([]string, error)
}

// NewFakeDriver returns a new fakedriver object
func NewFakeDriver(create func() (string, string, error), delete func(string) error, existing func() (string, error)) Driver {
	return &FakeDriver{
		create:   create,
		delete:   delete,
		existing: existing,
	}
}

// Create returns a newly created fake driver
func (d *FakeDriver) Create() (string, string, error) {
	return d.create()
}

// Delete deletes a fake driver
func (d *FakeDriver) Delete(machineID string) error {
	return d.delete(machineID)
}

// GetExisting returns the existing fake driver
func (d *FakeDriver) GetExisting() (string, error) {
	return d.existing()
}

// GetVMs returns a list of VMs
func (d *FakeDriver) GetVMs(name string) (VMs, error) {
	listOfVMs := make(map[string]string)
	return listOfVMs, nil
}

// GetVolNames parses volume names from pv specs
func (d *FakeDriver) GetVolNames(specs []corev1.PersistentVolumeSpec) ([]string, error) {
	volNames := []string{}
	return volNames, nil
}

//GetUserData return the used data whit which the VM will be booted
func (d *FakeDriver) GetUserData() string {
	return ""
}

//SetUserData set the used data whit which the VM will be booted
func (d *FakeDriver) SetUserData(userData string) {
	return
}
