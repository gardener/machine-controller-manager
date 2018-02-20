/*
Copyright 2017 The Gardener Authors.

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
	}

	runResult, err := svc.RunInstances(&inputConfig)
	if err != nil {
		return "Error", "Error", err
	}

	// Add tags to the created machine
	tagList := []*ec2.Tag{}
	for idx, element := range d.AWSMachineClass.Spec.Tags {
		newTag := ec2.Tag{
			Key:   aws.String(idx),
			Value: aws.String(element),
		}
		tagList = append(tagList, &newTag)
	}

	// If no "Name" tag has been specified in the MachineClass definition we set the "Name" tag's
	// value to "name-of-machine-object".
	if _, ok := d.AWSMachineClass.Spec.Tags["Name"]; !ok {
		newTag := ec2.Tag{
			Key:   aws.String("Name"),
			Value: aws.String(d.MachineName),
		}
		tagList = append(tagList, &newTag)
	}

	_, errtag := svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{runResult.Instances[0].InstanceId},
		Tags:      tagList,
	})
	if errtag != nil {
		return "Error", "Error", errtag
	}

	return d.encodeMachineID(d.AWSMachineClass.Spec.Region, *runResult.Instances[0].InstanceId), *runResult.Instances[0].PrivateDnsName, nil
}

// Delete method is used to delete a AWS machine
func (d *AWSDriver) Delete() error {
	var (
		err       error
		machineID = d.decodeMachineID(d.MachineID)
	)

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

// Helper function to create SVC
func (d *AWSDriver) createSVC() *ec2.EC2 {

	accessKeyID := strings.TrimSpace(string(d.CloudConfig.Data["providerAccessKeyId"]))
	secretAccessKey := strings.TrimSpace(string(d.CloudConfig.Data["providerSecretAccessKey"]))

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
