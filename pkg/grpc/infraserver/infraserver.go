package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
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

	// Never return from this handler, this will close the stream
	<-waitc
	return nil
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
	createReq := pb.MCMside{
		OperationID:     2,
		OperationType:   "create",
		Operationparams: &createParams,
	}

	if err := (*driver.stream).Send(&createReq); err != nil {
		log.Fatalf("Failed to send create req: %v", err)
	}

	createResp, err := (*driver.stream).Recv()
	if err != nil {
		log.Fatalf("Failed to send create req: %v", err)
	}
	response := createResp.GetCreateresponse()
	log.Printf("Create response: %s %s %d", response.ProviderID, response.Nodename, response.Error)
	return response.ProviderID, response.Nodename, response.Error
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
			log.Printf("Calling valid create after 10 seconds")
			time.Sleep(10 * time.Second)
			doCreate("fakeDriver", "a", "b")

			log.Printf("Calling invalid create after 5 seconds")
			time.Sleep(5 * time.Second)
			doCreate("someDriver", "a", "b")
		}
	}()

	log.Printf("Starting grpc server")
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterInfragrpcServer(grpcServer, &Server{})
	grpcServer.Serve(lis)
}
