package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"path"
	"time"

	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"

	pb "github.com/gardener/machine-controller-manager/pkg/grpc/infrapb"
	"github.com/golang/glog"
)

var (
	tls      = false //"Connection uses TLS if true, else plain TCP"
	certFile = ""    //"The TLS cert file"
	keyFile  = ""    //"The TLS key file"
	port     = 10000 //"The server port"
)

// ExternalDriverManager manages the registered drivers.
type ExternalDriverManager struct {
	// a map of machine class type to the corresponding driver.
	drivers map[metav1.TypeMeta]*driver
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

func (s *ExternalDriverManager) registerDriver(machineClassType metav1.TypeMeta, stream pb.Infragrpc_RegisterServer) (*driver, error) {
	if stream == nil {
		return nil, fmt.Errorf("Cannot register invalid driver stream for machine class type %s", machineClassType)
	}

	err := stream.Context().Err()
	if err != nil {
		return nil, err
	}

	_, err = s.GetDriver(machineClassType)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan interface{})
	driver := &driver{
		machineClassType: machineClassType,
		stream:           stream,
		stopCh:           stopCh,
		pendingRequests:  make(map[int32](chan *pb.DriverSide)),
	}

	s.drivers[machineClassType] = driver

	go func() {
		<-stopCh

		glog.Infof("Driver for machine class type %s is closed.", machineClassType)
		latest := s.drivers[machineClassType]
		if driver == latest {
			delete(s.drivers, machineClassType)
			glog.Infof("Driver for machine class type %s is unregistered.", machineClassType)
		}
	}()

	return driver, nil
}

// Stop the ExternalDriverManager and all the active drivers.
func (s *ExternalDriverManager) Stop() {
	for _, driver := range s.drivers {
		driver.close()
	}
}

// Register Requests driver to send it's details, and sets up stream
func (s *ExternalDriverManager) Register(stream pb.Infragrpc_RegisterServer) error {
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
		log.Printf("received error %v", err)
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

//ShareMeta share metadata
func (s *ExternalDriverManager) ShareMeta(ctx context.Context, in *pb.Metadata) (*pb.ErrorResp, error) {
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

	driverManager := &ExternalDriverManager{}
	// Test go routine to test end to end flow
	go func() {
		for {
			for _, driver := range driverManager.drivers {
				time.Sleep(5 * time.Second)
				log.Printf("Calling valid create")
				driver.Create("fakeDriver", "a", "b")

				time.Sleep(5 * time.Second)
				log.Printf("Calling invalid create")
				driver.Create("someDriver", "a", "b")

				time.Sleep(5 * time.Second)
				log.Printf("Calling valid delete")
				driver.Delete("fakeDriver", "a", "b")
			}
		}
	}()

	log.Printf("Starting grpc server")
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterInfragrpcServer(grpcServer, driverManager)
	grpcServer.Serve(lis)
}
