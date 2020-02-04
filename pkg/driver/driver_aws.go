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
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/klog"
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
	svc, err := d.createSVC()
	if err != nil {
		return "Error", "Error", err
	}

	UserDataEnc := base64.StdEncoding.EncodeToString([]byte(d.UserData))

	var imageIds []*string
	imageID := aws.String(d.AWSMachineClass.Spec.AMI)
	imageIds = append(imageIds, imageID)

	describeImagesRequest := ec2.DescribeImagesInput{
		ImageIds: imageIds,
	}
	output, err := svc.DescribeImages(&describeImagesRequest)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()
		return "Error", "Error", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()

	if len(output.Images) < 1 {
		return "Error", "Error", fmt.Errorf("Image %s not found", *imageID)
	}

	blkDeviceMappings, err := d.generateBlockDevices(d.AWSMachineClass.Spec.BlockDevices, output.Images[0].RootDeviceName)
	if err != nil {
		return "Error", "Error", err
	}

	tagInstance, err := d.generateTags(d.AWSMachineClass.Spec.Tags)
	if err != nil {
		return "Error", "Error", err
	}

	securityGroupIDs, err := d.generateSecurityGroups(d.AWSMachineClass.Spec.NetworkInterfaces[0].SecurityGroupIDs)
	if err != nil {
		return "Error", "Error", err
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
		SecurityGroupIds:    securityGroupIDs,
		BlockDeviceMappings: blkDeviceMappings,
		TagSpecifications:   []*ec2.TagSpecification{tagInstance},
	}

	runResult, err := svc.RunInstances(&inputConfig)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()
		return "Error", "Error", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()

	return d.encodeMachineID(d.AWSMachineClass.Spec.Region, *runResult.Instances[0].InstanceId), *runResult.Instances[0].PrivateDnsName, nil
}

func (d *AWSDriver) generateSecurityGroups(sgIDs []string) ([]*string, error) {

	var securityGroupIDs []*string
	for _, sg := range sgIDs {
		securityGroupIDs = append(securityGroupIDs, aws.String(sg))
	}
	return securityGroupIDs, nil
}

func (d *AWSDriver) generateTags(tags map[string]string) (*ec2.TagSpecification, error) {

	// Add tags to the created machine
	tagList := []*ec2.Tag{}
	for idx, element := range tags {
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
	return tagInstance, nil
}

func (d *AWSDriver) generateBlockDevices(blockDevices []v1alpha1.AWSBlockDeviceMappingSpec, rootDeviceName *string) ([]*ec2.BlockDeviceMapping, error) {

	var blkDeviceMappings []*ec2.BlockDeviceMapping
	// if blockDevices is empty, AWS will automatically create a root partition
	for _, disk := range blockDevices {

		deviceName := disk.DeviceName
		if disk.DeviceName == "/root" || len(blockDevices) == 1 {
			deviceName = *rootDeviceName
		}
		volumeSize := disk.Ebs.VolumeSize
		volumeType := disk.Ebs.VolumeType
		deleteOnTermination := disk.Ebs.DeleteOnTermination
		encrypted := disk.Ebs.Encrypted

		blkDeviceMapping := ec2.BlockDeviceMapping{
			DeviceName: aws.String(deviceName),
			Ebs: &ec2.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(deleteOnTermination),
				Encrypted:           aws.Bool(encrypted),
				VolumeSize:          aws.Int64(volumeSize),
				VolumeType:          aws.String(volumeType),
			},
		}
		if volumeType == "io1" {
			blkDeviceMapping.Ebs.Iops = aws.Int64(disk.Ebs.Iops)
		}
		blkDeviceMappings = append(blkDeviceMappings, &blkDeviceMapping)
	}

	return blkDeviceMappings, nil
}

// Delete method is used to delete a AWS machine
func (d *AWSDriver) Delete(machineID string) error {

	result, err := d.GetVMs(machineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machine-ID
		klog.V(2).Infof("No VM matching the machine-ID found on the provider %q", machineID)
		return nil
	}

	instanceID := d.decodeMachineID(machineID)

	svc, err := d.createSVC()
	if err != nil {
		return err
	}

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
		DryRun: aws.Bool(true),
	}
	_, err = svc.TerminateInstances(input)
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "DryRunOperation" {
		input.DryRun = aws.Bool(false)
		output, err := svc.TerminateInstances(input)
		if err != nil {
			metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()
			klog.Errorf("Could not terminate machine: %s", err.Error())
			return err
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()

		vmState := output.TerminatingInstances[0]
		//klog.Info(vmState.PreviousState, vmState.CurrentState)

		if *vmState.CurrentState.Name == "shutting-down" ||
			*vmState.CurrentState.Name == "terminated" {
			return nil
		}

		err = errors.New("Machine already terminated")
	}

	klog.Errorf("Could not terminate machine: %s", err.Error())
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

	svc, err := d.createSVC()
	if err != nil {
		return listOfVMs, err
	}

	input := ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []*string{
					&clusterName,
				},
			},
			{
				Name: aws.String("tag-key"),
				Values: []*string{
					&nodeRole,
				},
			},
			{
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

	// When targeting particular VM
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
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()
		klog.Errorf("AWS driver is returning error while describe instances request is sent: %s", err)
		return listOfVMs, err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "aws", "service": "ecs"}).Inc()

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
func (d *AWSDriver) createSVC() (*ec2.EC2, error) {
	var (
		config = &aws.Config{
			Region: aws.String(d.AWSMachineClass.Spec.Region),
		}

		accessKeyID     = strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AWSAccessKeyID]))
		secretAccessKey = strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.AWSSecretAccessKey]))
	)

	if accessKeyID != "" && secretAccessKey != "" {
		config.Credentials = credentials.NewStaticCredentialsFromCreds(credentials.Value{
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
		})
	}

	s, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}

	return ec2.New(s), nil
}

func (d *AWSDriver) encodeMachineID(region, machineID string) string {
	return fmt.Sprintf("aws:///%s/%s", region, machineID)
}

func (d *AWSDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}

// GetVolNames parses volume names from pv specs
func (d *AWSDriver) GetVolNames(specs []corev1.PersistentVolumeSpec) ([]string, error) {
	names := []string{}
	for i := range specs {
		spec := &specs[i]
		if spec.AWSElasticBlockStore == nil {
			// Not an aws volume
			continue
		}
		name := spec.AWSElasticBlockStore.VolumeID
		names = append(names, name)
	}
	return names, nil
}

//GetUserData return the used data whit which the VM will be booted
func (d *AWSDriver) GetUserData() string {
	return d.UserData
}

//SetUserData set the used data whit which the VM will be booted
func (d *AWSDriver) SetUserData(userData string) {
	d.UserData = userData
}
