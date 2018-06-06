package main

import (
	"context"
	"io"
	"log"
	"time"

	pb "github.com/gardener/machine-controller-manager/pkg/grpc/infrapb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"
)

var (
	tls                = false                //"Connection uses TLS if true, else plain TCP"
	caFile             = ""                   //"The file containning the CA root cert file"
	serverAddr         = "127.0.0.1:10000"    //"The server address in the format of host:port"
	serverHostOverride = "x.test.youtube.com" //"The server name use to verify the hostname returned by TLS handshake"
)

func registerwithMCM(client pb.InfragrpcClient) {
	log.Printf("Calling register")
	ctx, cancel := context.WithTimeout(context.Background(), 100000000*time.Second)
	defer cancel()

	stream, err := client.Register(ctx)
	if err != nil {
		log.Fatalf("%v.Register(_) = _, %v: ", client, err)
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			return
		}
		if err != nil {
			log.Fatalf("Failed to receive: %v", err)
		}

		resp := pb.DriverSide{}
		resp.OperationID = in.OperationID
		resp.OperationType = in.OperationType

		if in.OperationType == "register" {
			log.Printf("Operation %s", in.OperationType)
			message := pb.DriverSideRegisterationResp{
				Name:    "fakeDriver",
				Kind:    "driver",
				Group:   "poc",
				Version: "alpha",
			}
			resp.Response = &pb.DriverSide_RegisterResp{
				RegisterResp: &message,
			}
			stream.Send(&resp)
		} else if in.OperationType == "create" {
			log.Printf("Operation %s", in.OperationType)
			opParams := in.GetOperationparams()

			// Do actual create based on operation parameters passed

			log.Printf("create parameters: %v", opParams)
			message := pb.DriverSideCreateResp{
				ProviderID: "fakeID",
				Nodename:   "fakename",
				Error:      "",
			}
			resp.Response = &pb.DriverSide_Createresponse{
				Createresponse: &message,
			}
			stream.Send(&resp)
		} else if in.OperationType == "delete" {
			log.Printf("Operation %s", in.OperationType)
			opParams := in.GetOperationparams()

			// Do actual delete based on operation parameters passed

			log.Printf("delete parameters: %v", opParams)
			message := pb.DriverSideDeleteResp{
				Error: "",
			}
			resp.Response = &pb.DriverSide_Deleteresponse{
				Deleteresponse: &message,
			}
			stream.Send(&resp)
		} else if in.OperationType == "list" {
			log.Printf("%s", in.OperationType)
			opParams := in.GetOperationparams()

			// Get actual list based on operation parameters passed

			log.Printf("list parameters: %v", opParams)
			message := pb.DriverSideListResp{
				List:  []string{"fakemach1", "fakemach2"},
				Error: "",
			}
			resp.Response = &pb.DriverSide_Listresponse{
				Listresponse: &message,
			}
			stream.Send(&resp)
		}
	}
}

func main() {
	var opts []grpc.DialOption
	if tls {
		if caFile == "" {
			caFile = testdata.Path("ca.pem")
		}
		creds, err := credentials.NewClientTLSFromFile(caFile, serverHostOverride)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewInfragrpcClient(conn)

	registerwithMCM(client)
}
