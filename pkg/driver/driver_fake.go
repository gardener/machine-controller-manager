// /*
// Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

// // Package driver contains the cloud provider specific implementations to manage machines
package driver

// import (
// 	"context"
// 	cmipb "github.com/gardener/machine-spec/lib/go/cmi"
// )

// // FakeDriver is a fake driver returned when none of the actual drivers match
// type FakeCmiDriverClient struct {
// 	CreateMachine(ctx context.Context) (string, string, error)
// 	DeleteMachine(ctx context.Context) error
// 	ListMachines(ctx context.Context) (string, error)
// }

// // NewFakeDriver returns a new fakedriver object
// func NewFakeDriver(create func() (string, string, error), delete func() error, existing func() (string, error)) Driver {
// 	return &FakeDriver{
// 		create:   create,
// 		delete:   delete,
// 		existing: existing,
// 	}
// }

// func (cs *DefaultMachineServer) CreateMachine(ctx context.Context, req *cmi.CreateMachineRequest) (*cmi.CreateMachineResponse, error) {
// 	return nil, status.Error(codes.Unimplemented, "")
// }

// func (cs *DefaultMachineServer) DeleteMachine(ctx context.Context, req *cmi.DeleteMachineRequest) (*cmi.DeleteMachineResponse, error) {
// 	return nil, status.Error(codes.Unimplemented, "")
// }

// func (cs *DefaultMachineServer) ListMachines(ctx context.Context, req *cmi.ListMachinesRequest) (*cmi.ListMachinesResponse, error) {
// 	return nil, status.Error(codes.Unimplemented, "")
// }
