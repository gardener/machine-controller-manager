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
	CreateMachine() (string, string, string, error)
	GetMachineStatus() (string, string, string, error)
	DeleteMachine() (string, error)
	ListMachines() (map[string]string, error)
	ShutDownMachine() error
	GetProviderID() string
	GetVolumeIDs([]*corev1.PersistentVolumeSpec) ([]string, error)
}

// CMIPluginClient is the struct used to create a generic driver to make gRPC calls
type CMIPluginClient struct {
	DriverName           string
	ProviderID           string
	MachineName          string
	UserData             string
	LastKnownState       string
	MachineClientCreator machineClientCreator
	MachineClass         *v1alpha1.MachineClass
	Secret               *corev1.Secret
	Capabilities         []*cmipb.PluginCapability
}

// NewCMIPluginClient return a CMIPluginClient object
// It also initializes the capabilities the Plugin supports
func NewCMIPluginClient(
	ProviderID string,
	driverName string,
	secret *corev1.Secret,
	machineClass interface{},
	machineName string,
	lastKnownState string,
) (*CMIPluginClient, error) {
	var (
		userData string
		ctx      = context.Background()
	)
	if secret != nil && secret.Data != nil {
		userData = string(secret.Data["userData"])
	}

	c := &CMIPluginClient{
		DriverName:           driverName,
		MachineClientCreator: newMachineClient,
		MachineClass:         machineClass.(*v1alpha1.MachineClass),
		Secret:               secret,
		UserData:             userData,
		ProviderID:           ProviderID,
		MachineName:          machineName,
		LastKnownState:       lastKnownState,
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
func (c *CMIPluginClient) CreateMachine() (string, string, string, error) {
	glog.V(4).Infof("Calling CreateMachine rpc for %q", c.MachineName)

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_CREATE_MACHINE)
	if err != nil {
		return "", "", "", err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return "", "", "", err
	}
	defer closer.Close()

	req := &cmipb.CreateMachineRequest{
		MachineName:    c.MachineName,
		ProviderSpec:   c.MachineClass.ProviderSpec.Raw,
		Secrets:        getSecretData(c.Secret),
		LastKnownState: []byte(c.LastKnownState),
	}
	ctx := context.Background()
	resp, err := machineClient.CreateMachine(ctx, req)
	if err != nil {
		return "", "", string(resp.LastKnownState), err
	}

	glog.V(4).Info("Machine Successfully Created, ProviderID:", resp.ProviderID)
	return resp.ProviderID, resp.NodeName, string(resp.LastKnownState), err
}

// DeleteMachine make a grpc call to the driver to delete the machine.
func (c *CMIPluginClient) DeleteMachine() (string, error) {
	glog.V(4).Info("Calling DeleteMachine rpc", c.MachineName)

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_DELETE_MACHINE)
	if err != nil {
		return "", err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return "", err
	}
	defer closer.Close()

	req := &cmipb.DeleteMachineRequest{
		MachineName:    c.MachineName,
		ProviderSpec:   c.MachineClass.ProviderSpec.Raw,
		Secrets:        getSecretData(c.Secret),
		LastKnownState: []byte(c.LastKnownState),
	}
	glog.Infof("DECODED B:%s, S:%s)", []byte(c.LastKnownState), c.LastKnownState)
	ctx := context.Background()
	response, err := machineClient.DeleteMachine(ctx, req)
	if err != nil {
		return "", err
	}

	glog.V(4).Info("Machine deletion is initiated successfully. ProviderID", c.ProviderID)
	return string(response.LastKnownState), nil
}

// GetMachineStatus makes a gRPC call to the driver to check existance of machine
func (c *CMIPluginClient) GetMachineStatus() (string, string, string, error) {
	glog.V(4).Infof("Calling GetMachine rpc for %q", c.MachineName)

	err := c.validatePluginRequest(cmi.PluginCapability_RPC_GET_MACHINE_STATUS)
	if err != nil {
		return "", "", "", err
	}

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return "", "", "", err
	}
	defer closer.Close()

	req := &cmipb.GetMachineStatusRequest{
		MachineName:  c.MachineName,
		ProviderSpec: c.MachineClass.ProviderSpec.Raw,
		Secrets:      getSecretData(c.Secret),
	}
	ctx := context.Background()
	response, err := machineClient.GetMachineStatus(ctx, req)
	if err != nil {
		return "", "", "", err
	}

	glog.V(4).Info("Get call successful for ", c.MachineName)
	return response.ProviderID, response.NodeName, c.LastKnownState, nil
}

// ListMachines have to list machines
func (c *CMIPluginClient) ListMachines() (map[string]string, error) {
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
func (c *CMIPluginClient) ShutDownMachine() error {
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
		MachineName:  c.MachineName,
		ProviderSpec: c.MachineClass.ProviderSpec.Raw,
		Secrets:      getSecretData(c.Secret),
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
func (c *CMIPluginClient) GetVolumeIDs(pvSpecs []*corev1.PersistentVolumeSpec) ([]string, error) {
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

// GetProviderID returns the ProviderID
func (c *CMIPluginClient) GetProviderID() string {
	return c.ProviderID
}
