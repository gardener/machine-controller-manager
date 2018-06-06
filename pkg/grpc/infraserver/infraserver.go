package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"

	pb "github.com/gardener/machine-controller-manager/pkg/grpc/infrapb"
)

var (
	tls      = false //"Connection uses TLS if true, else plain TCP"
	certFile = ""    //"The TLS cert file"
	keyFile  = ""    //"The TLS key file"
	port     = 10000 //"The server port"
)

// Server definition
type Server struct {
}

// Driver driver
type Driver struct {
	driver *pb.DriverSideRegisterationResp
	stream *pb.Infragrpc_RegisterServer
}

// TODO: Make this array of drivers so that multiple drivers can register
var driver Driver

// Register Requests driver to send it's details, and sets up stream
func (s *Server) Register(stream pb.Infragrpc_RegisterServer) error {

	waitc := make(chan struct{})

	regReq := pb.MCMside{
		OperationID:   1,
		OperationType: "register",
	}

	stream.Send(&regReq)

	resp, err := stream.Recv()
	if err == io.EOF {
		// return will close stream from server side
		log.Println("exit")
		return nil
	}
	if err != nil {
		log.Printf("receive error %v", err)
	}

	driver.driver = resp.GetRegisterResp()
	driver.stream = &stream

	log.Printf("driver: %v", driver)

	// Start receiving stream
	go receiveDriverStream(driver)

	// Never return from this handler, this will close the stream
	<-waitc
	return nil
}

var opID int32
var mutex = &sync.Mutex{}

var requestCache map[int32](*chan *pb.DriverSide)

func sendAndWait(params *pb.MCMsideOperationParams, opType string) (interface{}, error) {

	waitc := make(chan *pb.DriverSide)

	mutex.Lock()
	opID++
	request := pb.MCMside{
		OperationID:     opID,
		OperationType:   opType,
		Operationparams: params,
	}
	mutex.Unlock()

	if err := (*driver.stream).Send(&request); err != nil {
		log.Fatalf("Failed to send request: %v", err)
		return nil, err
	}

	if requestCache == nil {
		requestCache = make(map[int32](*chan *pb.DriverSide))
	}
	requestCache[request.OperationID] = &waitc

	// The receiveDriverStream function will receive message, read the opID, then write to corresponding waitc
	// This will make sure that the response structure is populated
	response := <-waitc

	delete(requestCache, request.OperationID)

	return response.GetResponse(), nil
}

func receiveDriverStream(newDriver Driver) {
	for {
		response, err := (*newDriver.stream).Recv()
		if err != nil {
			log.Fatalf("Failed to receive response: %v", err)
		}

		if _, ok := requestCache[response.OperationID]; ok {
			*requestCache[response.OperationID] <- response
		} else {
			log.Printf("Request ID %d missing in request cache", response.OperationID)
		}
	}
}

// doCreate sends create request to the driver over the grpc stream
func doCreate(providerName, machineclass, machineID string) (string, string, int32) {

	if driver.driver.Name != providerName {
		log.Printf("Driver not available")
		return "", "", 1
	}

	createParams := pb.MCMsideOperationParams{
		MachineClassMetaData: &pb.MCMsideMachineClassMeta{
			Name:     "fakeclass",
			Revision: 1,
		},
		CloudConfig: "fakeCloudConfig",
		UserData:    "fakeData",
		MachineID:   "fakeID",
		MachineName: "fakename",
	}

	createResp, err := sendAndWait(&createParams, "create")
	if err != nil {
		log.Fatalf("Failed to send create req: %v", err)
	}

	if createResp == nil {
		log.Printf("nil")
		return "", "", 2
	}
	response := createResp.(*pb.DriverSide_Createresponse).Createresponse
	log.Printf("Create. Return: %s %s %d", response.ProviderID, response.Nodename, response.Error)
	return response.ProviderID, response.Nodename, response.Error
}

// doDelete sends delete request to the driver over the grpc stream
func doDelete(providerName, machineclass, machineID string) int32 {

	if driver.driver.Name != providerName {
		log.Printf("Driver not available")
		return 1
	}

	deleteParams := pb.MCMsideOperationParams{
		MachineClassMetaData: &pb.MCMsideMachineClassMeta{
			Name:     "fakeclass",
			Revision: 1,
		},
		CloudConfig: "fakeCloudConfig",
		MachineID:   "fakeID",
		MachineName: "fakename",
	}

	deleteResp, err := sendAndWait(&deleteParams, "delete")
	if err != nil {
		log.Fatalf("Failed to send delete req: %v", err)
	}

	if deleteResp == nil {
		log.Printf("nil")
		return 2
	}
	response := deleteResp.(*pb.DriverSide_Deleteresponse).Deleteresponse
	log.Printf("Delete Return: %d", response.Error)
	return response.Error
}

//ShareMeta share metadata
func (s *Server) ShareMeta(ctx context.Context, in *pb.Metadata) (*pb.ErrorResp, error) {
	return nil, nil
}

func main() {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if tls {
		if certFile == "" {
			certFile = testdata.Path("server1.pem")
		}
		if keyFile == "" {
			keyFile = testdata.Path("server1.key")
		}
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	// Test go routine to test end to end flow
	go func() {
		for {
			time.Sleep(5 * time.Second)
			log.Printf("Calling valid create")
			doCreate("fakeDriver", "a", "b")

			time.Sleep(5 * time.Second)
			log.Printf("Calling invalid create")
			doCreate("someDriver", "a", "b")

			time.Sleep(5 * time.Second)
			log.Printf("Calling valid delete")
			doDelete("fakeDriver", "a", "b")
		}
	}()

	log.Printf("Starting grpc server")
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterInfragrpcServer(grpcServer, &Server{})
	grpcServer.Serve(lis)
}
