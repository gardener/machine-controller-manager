/*
Copyright 2017 The Kubernetes Authors.

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

package cmiclient

import (
	"context"
	"encoding/json"
	"io"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-spec/lib/go/cmi"
	cmipb "github.com/gardener/machine-spec/lib/go/cmi"
	corev1 "k8s.io/api/core/v1"

	"github.com/golang/glog"
)

// machineClientCreator is a helper struct
type machineClientCreator func(driverName string) (
	machineClient cmipb.MachineClient,
	closer io.Closer,
	err error,
)

// VMs contains the map from Machine-ID to Machine-Name
type VMs map[string]string

// CMIClient is the client used to communicate with the CMIServer
type CMIClient interface {
	CreateMachine() (string, string, error)
	GetMachine(string) (bool, error)
	DeleteMachine(string) error
	ListMachines() (map[string]string, error)
	ShutDownMachine(string) error
	GetMachineID() string
	GetVolumeIDs([]*corev1.PersistentVolumeSpec) ([]string, error)
}

// CMIDriverClient is the struct used to create a generic driver to make gRPC calls
type CMIDriverClient struct {
	DriverName           string
	MachineID            string
	MachineName          string
	UserData             string
	MachineClientCreator machineClientCreator
	MachineClass         *v1alpha1.MachineClass
	Secret               *corev1.Secret
	Capabilities         []*cmipb.PluginCapability
}

// NewCMIDriverClient returns a new cmi client
func NewCMIDriverClient(machineID string, driverName string, secret *corev1.Secret, machineClass interface{}, machineName string) (*CMIDriverClient, error) {
	var (
		userData string
		ctx      = context.Background()
	)
	if secret != nil && secret.Data != nil {
		userData = string(secret.Data["userData"])
	}

	c := &CMIDriverClient{
		DriverName:           driverName,
		MachineClientCreator: newMachineClient,
		MachineClass:         machineClass.(*v1alpha1.MachineClass),
		Secret:               secret,
		UserData:             userData,
		MachineID:            machineID,
		MachineName:          machineName,
	}

	identityClient, closer, err := newIdentityClient(c.DriverName)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := identityClient.GetPluginCapabilities(ctx, &cmipb.GetPluginCapabilitiesRequest{})
	if err != nil {
		return nil, err
	}

	c.Capabilities = resp.GetCapabilities()

	return c, nil
}

// CreateMachine makes a gRPC call to the driver to create the machine.
func (c *CMIDriverClient) CreateMachine() (string, string, error) {
	glog.V(4).Infof("Calling CreateMachine rpc for %q", c.MachineName)

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_CREATE_MACHINE)
	if err != nil {
		return "", "", err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return "", "", err
	}
	defer closer.Close()

	req := &cmipb.CreateMachineRequest{
		Name:         c.MachineName,
		ProviderSpec: c.MachineClass.ProviderSpec.Raw,
		Secrets:      getSecretData(c.Secret),
	}
	ctx := context.Background()
	resp, err := machineClient.CreateMachine(ctx, req)
	if err != nil {
		return "", "", err
	}

	glog.V(4).Info("Machine Successfully Created, MachineID:", resp.MachineID)
	return resp.MachineID, resp.NodeName, err
}

// DeleteMachine make a grpc call to the driver to delete the machine.
func (c *CMIDriverClient) DeleteMachine(MachineID string) error {
	glog.V(4).Info("Calling DeleteMachine rpc", c.MachineName)

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_DELETE_MACHINE)
	if err != nil {
		return err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return err
	}
	defer closer.Close()

	req := &cmipb.DeleteMachineRequest{
		MachineID: c.MachineID,
		Secrets:   getSecretData(c.Secret),
	}
	ctx := context.Background()
	_, err = machineClient.DeleteMachine(ctx, req)
	if err != nil {
		return err
	}

	glog.V(4).Info("Machine deletion is initiated successfully. MachineID", c.MachineID)
	return err
}

// GetMachine makes a gRPC call to the driver to check existance of machine
func (c *CMIDriverClient) GetMachine(MachineID string) (bool, error) {
	glog.V(4).Infof("Calling GetMachine rpc for %q", c.MachineName)

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_GET_MACHINE)
	if err != nil {
		return false, err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return false, err
	}
	defer closer.Close()

	req := &cmipb.GetMachineRequest{
		MachineID: MachineID,
		Secrets:   getSecretData(c.Secret),
	}
	ctx := context.Background()
	_, err = machineClient.GetMachine(ctx, req)
	if err != nil {
		return false, err
	}

	glog.V(4).Info("Get call successful for ", c.MachineName)
	return true, nil
}

// ListMachines have to list machines
func (c *CMIDriverClient) ListMachines() (map[string]string, error) {
	glog.V(4).Info("Calling ListMachine rpc")

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_LIST_MACHINES)
	if err != nil {
		return nil, err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	req := &cmipb.ListMachinesRequest{
		ProviderSpec: c.MachineClass.ProviderSpec.Raw,
		Secrets:      getSecretData(c.Secret),
	}
	ctx := context.Background()

	resp, err := machineClient.ListMachines(ctx, req)
	if err != nil {
		return nil, err
	}

	glog.V(4).Info("ListMachine rpc was processed succesfully")
	return resp.MachineList, err
}

// ShutDownMachine implements shutdownmachine
func (c *CMIDriverClient) ShutDownMachine(MachineID string) error {
	glog.V(4).Infof("Calling ShutDownMachine rpc for %q", c.MachineName)

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_SHUTDOWN_MACHINE)
	if err != nil {
		return err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return err
	}
	defer closer.Close()

	req := &cmipb.ShutDownMachineRequest{
		MachineID: MachineID,
		Secrets:   getSecretData(c.Secret),
	}

	ctx := context.Background()
	_, err = machineClient.ShutDownMachine(ctx, req)
	if err != nil {
		return err
	}

	glog.V(4).Infof("ShutDownMachine successful for %q", c.MachineName)
	return nil
}

// GetVolumeIDs returns a list of VolumeIDs for the PV spec list supplied
func (c *CMIDriverClient) GetVolumeIDs(pvSpecs []*corev1.PersistentVolumeSpec) ([]string, error) {
	glog.V(4).Info("Calling GetVolumeIDs rpc")

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_GET_VOLUME_IDS)
	if err != nil {
		return nil, err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	pvSpecsMarshalled, err := json.Marshal(pvSpecs)
	if err != nil {
		return nil, err
	}

	req := &cmipb.GetVolumeIDsRequest{
		PVSpecList: pvSpecsMarshalled,
	}
	ctx := context.Background()

	resp, err := machineClient.GetVolumeIDs(ctx, req)
	if err != nil {
		return nil, err
	}

	glog.V(4).Info("GetVolumeIDs rpc was processed succesfully")
	return resp.GetVolumeIDs(), nil
}

// GetMachineID returns the machineID
func (c *CMIDriverClient) GetMachineID() string {
	return c.MachineID
}
