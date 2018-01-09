/*
Copyright 2017 The Gardener Authors.

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
package driver

// FakeDriver TODO
type FakeDriver struct {
	create   func() (string, string, error)
	delete   func() error
	//existing func() (string, v1alpha1.MachinePhase, error)
	existing func() (string, error)
}

// NewFakeDriver TODO
func NewFakeDriver(create func() (string, string, error), delete func() error, existing func() (string, error)) Driver {
	return &FakeDriver{
		create:   create,
		delete:   delete,
		existing: existing,
	}
}

// Create TODO
func (d *FakeDriver) Create() (string, string, error) {
	return d.create()
}

// Delete TODO
func (d *FakeDriver) Delete() error {
	return d.delete()
}

// GetExisting TODO
func (d *FakeDriver) GetExisting() (string, error) {
	return d.existing()
}
