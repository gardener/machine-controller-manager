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

package aws

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/driver/grpc/client"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
)

// AwsDriverProvider implements infraclient.ExternalDriverProvider.
type AwsDriverProvider struct {
	MachineClassDataProvider client.MachineClassDataProvider
}

// NewAWSDriverProvider creates a new instance of awsDriverProvider.
func NewAWSDriverProvider() client.ExternalDriverProvider {
	d := AwsDriverProvider{}
	return &d
}

// Create creates a machine
func (d *AwsDriverProvider) Create(machineClassMeta *client.MachineClassMeta, credentials, machineID, machineName string) (string, string, error) {
	machineClass, secret, err := d.getMachineClassData(machineClassMeta)
	if err != nil {
		return "", "", err
	}
	svc, err := d.createSVC(machineClass, secret)
	if err != nil {
		return "", "", err
	}

	fmt.Println("Creating machine", machineName)

	var imageIds []*string
	imageID := aws.String(machineClass.Spec.AMI)
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
	volumeSize := machineClass.Spec.BlockDevices[0].Ebs.VolumeSize
	volumeType := machineClass.Spec.BlockDevices[0].Ebs.VolumeType
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
	for idx, element := range machineClass.Spec.Tags {
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
		Value: aws.String(machineName),
	}
	tagList = append(tagList, &nameTag)

	tagInstance := &ec2.TagSpecification{
		ResourceType: aws.String("instance"),
		Tags:         tagList,
	}

	userData := base64.StdEncoding.EncodeToString(secret.Data["userData"])

	// Specify the details of the machine that you want to create.
	inputConfig := ec2.RunInstancesInput{
		// An Amazon Linux AMI ID for t2.micro machines in the us-west-2 region
		ImageId:      aws.String(machineClass.Spec.AMI),
		InstanceType: aws.String(machineClass.Spec.MachineType),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		UserData:     &userData,
		KeyName:      aws.String(machineClass.Spec.KeyName),
		SubnetId:     aws.String(machineClass.Spec.NetworkInterfaces[0].SubnetID),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: &(machineClass.Spec.IAM.Name),
		},
		SecurityGroupIds:    []*string{aws.String(machineClass.Spec.NetworkInterfaces[0].SecurityGroupIDs[0])},
		BlockDeviceMappings: blkDeviceMappings,
		TagSpecifications:   []*ec2.TagSpecification{tagInstance},
	}

	runResult, err := svc.RunInstances(&inputConfig)
	if err != nil {
		return "Error", "Error", err
	}

	return d.encodeMachineID(machineClass.Spec.Region, *runResult.Instances[0].InstanceId), *runResult.Instances[0].PrivateDnsName, nil
}

// Delete deletes a machine
func (d *AwsDriverProvider) Delete(machineClassMeta *client.MachineClassMeta, credentials, machineID string) error {
	result, err := d.List(machineClassMeta, credentials, machineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machine-ID
		glog.V(2).Infof("No VM matching the machine-ID found on the provider %q", machineID)
		return nil
	}

	fmt.Println("Deleting machine", machineID)

	machineID = d.decodeMachineID(machineID)

	machineClass, secretData, err := d.getMachineClassData(machineClassMeta)
	if err != nil {
		return err
	}
	svc, err := d.createSVC(machineClass, secretData)
	if err != nil {
		return err
	}

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

// List lists machines
func (d *AwsDriverProvider) List(machineClassMeta *client.MachineClassMeta, credentials, machineID string) (map[string]string, error) {
	listOfVMs := make(map[string]string)

	clusterName := ""
	nodeRole := ""

	machineClass, secret, err := d.getMachineClassData(machineClassMeta)
	if err != nil {
		return listOfVMs, err
	}
	svc, err := d.createSVC(machineClass, secret)
	if err != nil {
		return listOfVMs, err
	}

	for key := range machineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io/cluster/") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes.io/role/") {
			nodeRole = key
		}
	}

	if clusterName == "" || nodeRole == "" {
		return listOfVMs, nil
	}

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
			listOfVMs[d.encodeMachineID(machineClass.Spec.Region, *instance.InstanceId)] = machineName
		}
	}

	fmt.Println("List() returned: ", listOfVMs)
	return listOfVMs, nil
}

func (d *AwsDriverProvider) getMachineClassData(machineClassMeta *client.MachineClassMeta) (*v1alpha1.AWSMachineClass, *corev1.Secret, error) {
	var (
		secret corev1.Secret
	)

	sMachineClass, err := d.MachineClassDataProvider.GetMachineClass(machineClassMeta)
	if err != nil {
		return nil, &secret, err
	}

	var machineClass v1alpha1.AWSMachineClass
	err = json.Unmarshal([]byte(sMachineClass.(string)), &machineClass)
	if err != nil {
		return nil, &secret, err
	}

	requiredSecret := client.SecretMeta{
		SecretName:      machineClass.Spec.SecretRef.Name,
		SecretNameSpace: machineClass.Spec.SecretRef.Namespace,
		Revision:        1,
	}

	encodedData, err := d.MachineClassDataProvider.GetSecret(&requiredSecret)

	if err != nil {
		return nil, &secret, err
	}

	err = json.Unmarshal([]byte(encodedData), &secret)
	if err != nil {
		glog.Errorf("Failed to unmarshal")
		return nil, &secret, err
	}

	return &machineClass, &secret, nil
}

// Helper function to create SVC
func (d *AwsDriverProvider) createSVC(machineClass *v1alpha1.AWSMachineClass, secret *corev1.Secret) (*ec2.EC2, error) {

	if secret.Data[v1alpha1.AWSAccessKeyID] != nil && secret.Data[v1alpha1.AWSSecretAccessKey] != nil {
		return ec2.New(session.New(&aws.Config{
			Region: aws.String(machineClass.Spec.Region),
			Credentials: credentials.NewStaticCredentialsFromCreds(credentials.Value{
				AccessKeyID:     string(secret.Data[v1alpha1.AWSAccessKeyID]),
				SecretAccessKey: string(secret.Data[v1alpha1.AWSSecretAccessKey]),
			}),
		})), nil
	}

	return ec2.New(session.New(&aws.Config{
		Region: aws.String(machineClass.Spec.Region),
	})), nil
}

func (d *AwsDriverProvider) encodeMachineID(region, machineID string) string {
	return fmt.Sprintf("aws:///%s/%s", region, machineID)
}

func (d *AwsDriverProvider) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}
