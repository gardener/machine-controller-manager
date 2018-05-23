/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package driver contains the cloud provider specific implementations to manage machines
package driver

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

// AWSDriver is the driver struct for holding AWS machine information
type AWSDriver struct {
	AWSMachineClass *v1alpha1.AWSMachineClass
	CloudConfig     *corev1.Secret
	UserData        string
	MachineID       string
	MachineName     string
}

// NewAWSDriver returns an empty AWSDriver object
func NewAWSDriver(create func() (string, error), delete func() error, existing func() (string, error)) Driver {
	return &AWSDriver{}
}

// Create method is used to create a AWS machine
func (d *AWSDriver) Create() (string, string, error) {

	svc := d.createSVC()
	UserDataEnc := base64.StdEncoding.EncodeToString([]byte(d.UserData))

	var imageIds []*string
	imageID := aws.String(d.AWSMachineClass.Spec.AMI)
	imageIds = append(imageIds, imageID)

	describeImagesRequest := ec2.DescribeImagesInput{
		ImageIds: imageIds,
	}
	output, err := svc.DescribeImages(&describeImagesRequest)
	if err != nil {
		return "Error", "Error", err
	}

	var blkDeviceMappings []*ec2.BlockDeviceMapping
	deviceName := output.Images[0].RootDeviceName
	volumeSize := d.AWSMachineClass.Spec.BlockDevices[0].Ebs.VolumeSize
	volumeType := d.AWSMachineClass.Spec.BlockDevices[0].Ebs.VolumeType
	blkDeviceMapping := ec2.BlockDeviceMapping{
		DeviceName: deviceName,
		Ebs: &ec2.EbsBlockDevice{
			VolumeSize: &volumeSize,
			VolumeType: &volumeType,
		},
	}
	blkDeviceMappings = append(blkDeviceMappings, &blkDeviceMapping)

	// Add tags to the created machine
	tagList := []*ec2.Tag{}
	for idx, element := range d.AWSMachineClass.Spec.Tags {
		if idx == "Name" {
			// Name tag cannot be set, as its used to identify backing machine object
			continue
		}
		newTag := ec2.Tag{
			Key:   aws.String(idx),
			Value: aws.String(element),
		}
		tagList = append(tagList, &newTag)
	}
	nameTag := ec2.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(d.MachineName),
	}
	tagList = append(tagList, &nameTag)

	tagInstance := &ec2.TagSpecification{
		ResourceType: aws.String("instance"),
		Tags:         tagList,
	}

	// Specify the details of the machine that you want to create.
	inputConfig := ec2.RunInstancesInput{
		// An Amazon Linux AMI ID for t2.micro machines in the us-west-2 region
		ImageId:      aws.String(d.AWSMachineClass.Spec.AMI),
		InstanceType: aws.String(d.AWSMachineClass.Spec.MachineType),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		UserData:     &UserDataEnc,
		KeyName:      aws.String(d.AWSMachineClass.Spec.KeyName),
		SubnetId:     aws.String(d.AWSMachineClass.Spec.NetworkInterfaces[0].SubnetID),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: &(d.AWSMachineClass.Spec.IAM.Name),
		},
		SecurityGroupIds:    []*string{aws.String(d.AWSMachineClass.Spec.NetworkInterfaces[0].SecurityGroupIDs[0])},
		BlockDeviceMappings: blkDeviceMappings,
		TagSpecifications:   []*ec2.TagSpecification{tagInstance},
	}

	runResult, err := svc.RunInstances(&inputConfig)
	if err != nil {
		return "Error", "Error", err
	}

	return d.encodeMachineID(d.AWSMachineClass.Spec.Region, *runResult.Instances[0].InstanceId), *runResult.Instances[0].PrivateDnsName, nil
}

// Delete method is used to delete a AWS machine
func (d *AWSDriver) Delete() error {

	result, err := d.GetVMs(d.MachineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machine-ID
		glog.V(2).Infof("No VM matching the machine-ID found on the provider %q", d.MachineID)
		return nil
	}

	machineID := d.decodeMachineID(d.MachineID)

	svc := d.createSVC()
	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			aws.String(machineID),
		},
		DryRun: aws.Bool(true),
	}
	_, err = svc.TerminateInstances(input)
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "DryRunOperation" {
		input.DryRun = aws.Bool(false)
		output, err := svc.TerminateInstances(input)
		if err != nil {
			glog.Errorf("Could not terminate machine: %s", err.Error())
			return err
		}

		vmState := output.TerminatingInstances[0]
		//glog.Info(vmState.PreviousState, vmState.CurrentState)

		if *vmState.CurrentState.Name == "shutting-down" ||
			*vmState.CurrentState.Name == "terminated" {
			return nil
		}

		err = errors.New("Machine already terminated")
	}

	glog.Errorf("Could not terminate machine: %s", err.Error())
	return err
}

// GetExisting method is used to get machineID for existing AWS machine
func (d *AWSDriver) GetExisting() (string, error) {
	return d.MachineID, nil
}

// GetVMs returns a VM matching the machineID
// If machineID is an empty string then it returns all matching instances
func (d *AWSDriver) GetVMs(machineID string) (VMs, error) {
	listOfVMs := make(map[string]string)

	clusterName := ""
	nodeRole := ""

	for key := range d.AWSMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io/cluster/") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes.io/role/") {
			nodeRole = key
		}
	}

	if clusterName == "" || nodeRole == "" {
		return listOfVMs, nil
	}

	svc := d.createSVC()
	input := ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String("tag-key"),
				Values: []*string{
					&clusterName,
				},
			},
			&ec2.Filter{
				Name: aws.String("tag-key"),
				Values: []*string{
					&nodeRole,
				},
			},
			&ec2.Filter{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("pending"),
					aws.String("running"),
					aws.String("stopping"),
					aws.String("stopped"),
				},
			},
		},
	}

	// When targetting particular VM
	if machineID != "" {
		machineID = d.decodeMachineID(machineID)
		instanceFilter := &ec2.Filter{
			Name: aws.String("instance-id"),
			Values: []*string{
				&machineID,
			},
		}
		input.Filters = append(input.Filters, instanceFilter)
	}

	runResult, err := svc.DescribeInstances(&input)
	if err != nil {
		glog.Errorf("AWS driver is returning error while describe instances request is sent: %s", err)
		return listOfVMs, err
	}

	for _, reservation := range runResult.Reservations {
		for _, instance := range reservation.Instances {

			machineName := ""
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" {
					machineName = *tag.Value
					break
				}
			}
			listOfVMs[d.encodeMachineID(d.AWSMachineClass.Spec.Region, *instance.InstanceId)] = machineName
		}
	}

	return listOfVMs, nil
}

// Helper function to create SVC
func (d *AWSDriver) createSVC() *ec2.EC2 {

	accessKeyID := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AWSAccessKeyID]))
	secretAccessKey := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AWSSecretAccessKey]))

	if accessKeyID != "" && secretAccessKey != "" {
		return ec2.New(session.New(&aws.Config{
			Region: aws.String(d.AWSMachineClass.Spec.Region),
			Credentials: credentials.NewStaticCredentialsFromCreds(credentials.Value{
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
			}),
		}))
	}

	return ec2.New(session.New(&aws.Config{
		Region: aws.String(d.AWSMachineClass.Spec.Region),
	}))
}

func (d *AWSDriver) encodeMachineID(region, machineID string) string {
	return fmt.Sprintf("aws:///%s/%s", region, machineID)
}

func (d *AWSDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}
