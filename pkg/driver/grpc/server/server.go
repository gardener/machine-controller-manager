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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path"
	"sync"

	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pb "github.com/gardener/machine-controller-manager/pkg/driver/grpc/service"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
)

// ExternalDriverManager manages the registered drivers.
type ExternalDriverManager struct {
	// a map of machine class type to the corresponding driver.
	drivers      map[metav1.TypeMeta]*driver
	Port         uint16
	Options      []grpc.ServerOption
	grpcServer   *grpc.Server
	Client       kubernetes.Interface
	SecretLister corelisters.SecretLister
}

//GetDriver gets a registered and working driver stream for the given machine class type.
func (s *ExternalDriverManager) GetDriver(machineClassType metav1.TypeMeta) (Driver, error) {
	driver := s.drivers[machineClassType]
	if driver == nil {
		return nil, fmt.Errorf("No driver available for machine class type %s", machineClassType)
	}

	stream := driver.stream
	if stream == nil {
		return nil, fmt.Errorf("No valid driver available for machine class type %s", machineClassType)
	}
	err := stream.Context().Err()
	if err != nil {
		return nil, err
	}

	return driver, nil
}

func (s *ExternalDriverManager) registerDriver(machineClassType metav1.TypeMeta, stream pb.Servicegrpc_RegisterServer) (*driver, error) {
	if stream == nil {
		return nil, fmt.Errorf("Cannot register invalid driver stream for machine class type %v", machineClassType)
	}

	err := stream.Context().Err()
	if err != nil {
		return nil, err
	}

	d, err := s.GetDriver(machineClassType)
	if err == nil && d != nil {
		return nil, fmt.Errorf("Driver for machineClassType %v already registered", machineClassType)
	}

	var sm sync.Map

	glog.Infof("Registering new driver")

	stopCh := make(chan interface{})
	newDriver := &driver{
		machineClassType: machineClassType,
		stream:           stream,
		stopCh:           stopCh,
		pendingRequests:  &sm,
	}

	if s.drivers == nil {
		s.drivers = make(map[metav1.TypeMeta]*driver)
	}
	s.drivers[machineClassType] = newDriver

	glog.Infof("Registered new driver %v", machineClassType)

	go func() {
		<-stopCh

		glog.Infof("Driver for machine class type %s is closed.", machineClassType)
		latest := s.drivers[machineClassType]
		if newDriver == latest {
			delete(s.drivers, machineClassType)
			glog.Infof("Driver for machine class type %s is unregistered.", machineClassType)
		}
	}()

	return newDriver, nil
}

// Stop the ExternalDriverManager and all the active drivers.
func (s *ExternalDriverManager) Stop() {
	for _, driver := range s.drivers {
		driver.close()
	}
	s.grpcServer.Stop()
}

// Register Requests driver to send it's details, and sets up stream
func (s *ExternalDriverManager) Register(stream pb.Servicegrpc_RegisterServer) error {
	regReq := pb.MCMside{
		OperationID:   1,
		OperationType: "register",
	}

	err := stream.Send(&regReq)
	if err != nil {
		// return will close the stream
		return err
	}

	msg, err := stream.Recv()
	if err == io.EOF {
		// return will close stream from server side
		glog.Warning("Driver closed before registration is completed.")
		return err
	}
	if err != nil {
		glog.Warningf("received error %v", err)
		return err
	}

	driverDetails := msg.GetRegisterResp()
	driver, err := s.registerDriver(metav1.TypeMeta{
		Kind:       driverDetails.Kind,
		APIVersion: path.Join(driverDetails.Group, driverDetails.Version),
	}, stream)

	if err != nil {
		return err
	}

	go driver.receiveAndDispatch()

	driver.wait()
	return nil
}

//GetSecret share metadata
func (s *ExternalDriverManager) GetSecret(ctx context.Context, in *pb.SecretMeta) (*pb.Secret, error) {
	secretName := in.GetSecretName()
	if secretName == "" {
		return nil, fmt.Errorf("Secret name not provided")
	}

	secretNS := in.GetNameSpace()
	if secretNS == "" {
		return nil, fmt.Errorf("Secret namespace not provided")
	}

	secretRef, err := s.SecretLister.Secrets(secretNS).Get(secretName)
	if err != nil {
		glog.Errorf("Error getting secret %s/%s: %v", secretNS, secretName, err)
		return nil, err
	}

	/*
		if data, ok := secretRef.Data["userData"]; ok {
			userData = data
		} else {
			return nil, fmt.Errorf("Cloud config not found in the provided secret")
		}
	*/
	encodedData, err := json.Marshal(secretRef)
	if err != nil {
		glog.Errorf("Error occurred while encoding secret data into json")
		return nil, err
	}

	cloudConfig := &pb.Secret{
		Data:  string(encodedData),
		Error: "",
	}

	return cloudConfig, nil
}

//GetMachineClass share metadata
func (s *ExternalDriverManager) GetMachineClass(ctx context.Context, in *pb.MachineClassMeta) (*pb.MachineClass, error) {
	machineClassName := in.GetName()
	if machineClassName == "" {
		return nil, fmt.Errorf("Machine class name not provided")
	}

	MachineClass, err := s.Client.Core().RESTClient().Get().AbsPath(machineClassName).Do().Raw()
	if err != nil {
		return nil, err
	}

	machineClassData := &pb.MachineClass{
		Data:  string(MachineClass),
		Error: "",
	}
	return machineClassData, nil
}

// Start start the grpc server
func (s *ExternalDriverManager) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", s.Port))
	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
	}

	glog.Infof("Starting grpc server...")
	s.grpcServer = grpc.NewServer(s.Options...)
	pb.RegisterServicegrpcServer(s.grpcServer, s)

	go func() {
		s.grpcServer.Serve(lis)
	}()
}
