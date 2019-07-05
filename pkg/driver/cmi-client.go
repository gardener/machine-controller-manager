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

package driver

import (
	"context"
	"io"
	"net"
	"strings"
	"time"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	cmipb "github.com/gardener/machine-spec/lib/go/cmi"
	corev1 "k8s.io/api/core/v1"

	"github.com/golang/glog"
	"google.golang.org/grpc"
)

// CMIClient is the client used to communicate with the CMIServer
type CMIClient interface {
	CreateMachine() (string, string, error)
	GetMachine(string) (bool, error)
	DeleteMachine(string) error
	ListMachines() (map[string]string, error)
	ShutDownMachine(string) error
	GetMachineID() string
}

// CMIDriverClient is the struct used to create a generic driver to make gRPC calls
type CMIDriverClient struct {
	DriverName           string
	MachineClientCreator machineClientCreator
	MachineClass         *v1alpha1.MachineClass
	Secret               *corev1.Secret
	UserData             string
	MachineID            string
	MachineName          string
}

type machineClientCreator func(driverName string) (
	machineClient cmipb.MachineClient,
	closer io.Closer,
	err error,
)

// VMs contains the map from Machine-ID to Machine-Name
type VMs map[string]string

func newMachineClient(driverName string) (machineClient cmipb.MachineClient, closer io.Closer, err error) {
	var conn *grpc.ClientConn
	conn, err = newGrpcConn(driverName)
	if err != nil {
		return nil, nil, err
	}

	machineClient = cmipb.NewMachineClient(conn)
	return machineClient, conn, nil
}

// NewCMIDriverClient returns a new cmi client
func NewCMIDriverClient(machineID string, driverName string, secret *corev1.Secret, machineClass interface{}, machineName string) *CMIDriverClient {
	c := &CMIDriverClient{
		DriverName:           driverName,
		MachineClientCreator: newMachineClient,
		MachineClass:         machineClass.(*v1alpha1.MachineClass),
		Secret:               secret,
		UserData:             string(secret.Data["userData"]),
		MachineID:            machineID,
		MachineName:          machineName,
	}
	return c
}

// CreateMachine makes a gRPC call to the driver to create the machine.
func (c *CMIDriverClient) CreateMachine() (string, string, error) {
	glog.V(4).Infof("Calling CreateMachine rpc for %q", c.MachineName)

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return "", "", err
	}
	defer closer.Close()

	req := &cmipb.CreateMachineRequest{
		ProviderSpec: c.MachineClass.ProviderSpec.Raw,
		Name:         c.MachineName,
		Secrets:      c.Secret.Data,
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

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return err
	}
	defer closer.Close()

	req := &cmipb.DeleteMachineRequest{
		MachineID: c.MachineID,
		Secrets:   c.Secret.Data,
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

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return false, err
	}
	defer closer.Close()

	req := &cmipb.GetMachineRequest{
		MachineID: MachineID,
		Secrets:   c.Secret.Data,
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

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	req := &cmipb.ListMachinesRequest{
		Secrets:      c.Secret.Data,
		ProviderSpec: c.MachineClass.ProviderSpec.Raw,
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

	machineClient, closer, err := c.MachineClientCreator(c.DriverName)
	if err != nil {
		return err
	}
	defer closer.Close()

	req := &cmipb.ShutDownMachineRequest{
		MachineID: MachineID,
		Secrets:   c.Secret.Data,
	}

	ctx := context.Background()
	_, err = machineClient.ShutDownMachine(ctx, req)
	if err != nil {
		return err
	}

	glog.V(4).Infof("ShutDownMachine successful for %q", c.MachineName)
	return nil
}

// GetMachineID returns the machineID
func (c *CMIDriverClient) GetMachineID() string {
	return c.MachineID
}

func newGrpcConn(driverName string) (*grpc.ClientConn, error) {

	var name, addr string
	driverInfo := strings.Split(driverName, "//")
	if driverName != "" && len(driverInfo) == 2 {
		name = driverInfo[0]
		addr = driverInfo[1]
	} else {
		name = "grpc-default-driver"
		addr = "127.0.0.1:8080"
	}

	network := "tcp"

	glog.V(5).Infof("Creating new gRPC connection for [%s://%s] for driver: %s", network, addr, name)

	return grpc.Dial(
		addr,
		grpc.WithInsecure(),
		grpc.WithDialer(func(target string, timeout time.Duration) (net.Conn, error) {
			return net.Dial(network, target)
		}),
	)
}
