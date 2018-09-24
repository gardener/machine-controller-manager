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

package main

import (
	"log"

	"github.com/gardener/machine-controller-manager/pkg/driver/grpc/client"
	aws "github.com/gardener/machine-controller-manager/pkg/external/driver/aws"
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
	awsDriver := aws.NewAWSDriverProvider()

	externalDriver := client.NewExternalDriver(serverAddr, []grpc.DialOption{
		grpc.WithInsecure(),
	}, awsDriver, &meta)

	awsDriver.(*aws.AwsDriverProvider).MachineClassDataProvider = externalDriver

	defer externalDriver.Stop()
	go externalDriver.Start()
	log.Printf("Started external aws client")
	<-stopCh
}
