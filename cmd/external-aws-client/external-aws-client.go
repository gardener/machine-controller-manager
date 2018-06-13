package main

import (
	"log"

	aws "github.com/gardener/machine-controller-manager/pkg/external/driver/aws"
	"github.com/gardener/machine-controller-manager/pkg/grpc/infraclient"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	serverAddr = "localhost:50000" //"The server address in the format of host:port"
)

func main() {
	stopCh := make(chan int32)

	meta := metav1.TypeMeta{
		Kind:       "AWSMachineClass",
		APIVersion: "machine.sapcloud.io/v1alpha1",
	}
	awsDriver := aws.NewAWSDriverProvider(&meta)

	externalDriver := infraclient.NewExternalDriver(serverAddr, []grpc.DialOption{
		grpc.WithInsecure(),
	}, awsDriver)

	defer externalDriver.Stop()
	externalDriver.Start()
	log.Printf("Started external aws client")
	<-stopCh
}
