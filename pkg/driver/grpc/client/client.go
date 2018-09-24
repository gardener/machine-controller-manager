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

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	pb "github.com/gardener/machine-controller-manager/pkg/driver/grpc/service"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExternalDriver structure mediates the communication with the machine-controller-manager
type ExternalDriver struct {
	serverAddr       string
	options          []grpc.DialOption
	provider         ExternalDriverProvider
	machineClassType *metav1.TypeMeta
	connection       *grpc.ClientConn
	client           pb.ServicegrpcClient
	stream           pb.Servicegrpc_RegisterClient
}

// NewExternalDriver creates a new Driver instance.
func NewExternalDriver(serverAddr string, options []grpc.DialOption, provider ExternalDriverProvider, machineClassType *metav1.TypeMeta) *ExternalDriver {
	return &ExternalDriver{
		serverAddr:       serverAddr,
		options:          options,
		provider:         provider,
		machineClassType: machineClassType,
	}
}

// Start calls internal function to start external driver
func (d *ExternalDriver) Start() error {
	for {
		d.StartDriver()
		fmt.Println("Retrying in 5 seconds")
		time.Sleep(5 * time.Second)
	}
}

// StartDriver starts the external driver.
func (d *ExternalDriver) StartDriver() error {
	conn, err := grpc.Dial(d.serverAddr, d.options...)
	if err != nil {
		fmt.Println("Error in dialing: ", err)
		return err
	}
	d.connection = conn
	client := pb.NewServicegrpcClient(conn)
	d.client = client

	d.serveMCM(client)

	return nil
}

// Stop stops the external driver.
func (d *ExternalDriver) Stop() error {
	stream := d.stream
	//connection := d.connection

	d.stream = nil
	d.connection = nil

	if stream != nil && stream.Context().Err() == nil {
		stream.Send(&pb.DriverSide{
			OperationType: "unregister",
		})
		stream.CloseSend()
	}
	var err error
	/*
		if connection != nil {
			err = connection.Close()
		}
	*/

	return err
}

func (d *ExternalDriver) serveMCM(client pb.ServicegrpcClient) error {
	glog.Infof("Registering with MCM...")
	ctx := context.Background()

	stream, err := client.Register(ctx)
	if err != nil {
		fmt.Println("Error in registering: ", err)
		return err
	}

	d.stream = stream

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			return err
		}
		if err != nil {
			fmt.Println("Failed to receive from stream: ", err)
			return err
		}

		glog.Infof("Operation %s", in.OperationType)
		opParams := in.GetOperationparams()
		glog.Infof("parameters received: %v", opParams)

		resp := pb.DriverSide{}
		resp.OperationID = in.OperationID
		resp.OperationType = in.OperationType

		switch in.OperationType {
		case "register":
			pMachineClassType := d.machineClassType
			gvk := pMachineClassType.GroupVersionKind()
			resp.Response = &pb.DriverSide_RegisterResp{
				RegisterResp: &pb.DriverSideRegisterationResp{
					Name:    "externalDriver",
					Kind:    gvk.Kind,
					Group:   gvk.Group,
					Version: gvk.Version,
				},
			}
		case "create":
			var machineClass *MachineClassMeta
			if opParams.MachineClassMetaData != nil {
				machineClass = &MachineClassMeta{
					Name:     opParams.MachineClassMetaData.Name,
					Revision: opParams.MachineClassMetaData.Revision,
				}
			}
			providerID, nodename, err := d.provider.Create(machineClass, opParams.Credentials, opParams.MachineID, opParams.MachineName)

			var sErr string
			if err != nil {
				sErr = err.Error()
			}
			resp.Response = &pb.DriverSide_Createresponse{
				Createresponse: &pb.DriverSideCreateResp{
					ProviderID: providerID,
					Nodename:   nodename,
					Error:      sErr,
				},
			}
		case "delete":
			var machineClass *MachineClassMeta
			if opParams.MachineClassMetaData != nil {
				machineClass = &MachineClassMeta{
					Name:     opParams.MachineClassMetaData.Name,
					Revision: opParams.MachineClassMetaData.Revision,
				}
			}
			err := d.provider.Delete(machineClass, opParams.Credentials, opParams.MachineID)
			var sErr string
			if err != nil {
				sErr = err.Error()
			}
			resp.Response = &pb.DriverSide_Deleteresponse{
				Deleteresponse: &pb.DriverSideDeleteResp{
					Error: sErr,
				},
			}
		case "list":
			var machineClass *MachineClassMeta
			if opParams.MachineClassMetaData != nil {
				machineClass = &MachineClassMeta{
					Name:     opParams.MachineClassMetaData.Name,
					Revision: opParams.MachineClassMetaData.Revision,
				}
			}
			vms, err := d.provider.List(machineClass, opParams.Credentials, opParams.MachineID)
			var list []*pb.DriverSideMachine
			var machine *pb.DriverSideMachine

			var sErr string
			if err == nil {
				size := len(vms)
				list = make([]*pb.DriverSideMachine, size)
				i := 0
				for machineID, machineName := range vms {
					machine = new(pb.DriverSideMachine)
					machine.MachineID = machineID
					machine.MachineName = machineName
					list[i] = machine
					i++
				}
			} else {
				sErr = err.Error()
			}
			resp.Response = &pb.DriverSide_Listresponse{
				Listresponse: &pb.DriverSideListResp{
					List:  list,
					Error: sErr,
				},
			}
		}

		stream.Send(&resp)
	}
}

// GetMachineClass contacts the grpc server to get the machine class.
func (d *ExternalDriver) GetMachineClass(machineClassMeta *MachineClassMeta) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resp, err := d.client.GetMachineClass(ctx, &pb.MachineClassMeta{
		Name:     machineClassMeta.Name,
		Revision: machineClassMeta.Revision,
	})

	if err != nil {
		return nil, err
	}

	sErr := resp.Error
	if sErr != "" {
		return nil, errors.New(sErr)
	}

	return resp.Data, nil
}

// GetSecret contacts the grpc server to get the secret
func (d *ExternalDriver) GetSecret(secretMeta *SecretMeta) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resp, err := d.client.GetSecret(ctx, &pb.SecretMeta{
		SecretName: secretMeta.SecretName,
		NameSpace:  secretMeta.SecretNameSpace,
		Revision:   secretMeta.Revision,
	})

	if err != nil {
		return "", err
	}

	sErr := resp.Error
	if sErr != "" {
		return "", errors.New(sErr)
	}

	return resp.Data, nil
}
