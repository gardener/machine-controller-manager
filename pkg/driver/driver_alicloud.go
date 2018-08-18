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
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/utils"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
)

// AlicloudDriver is the driver struct for holding Alicloud machine information
type AlicloudDriver struct {
	AlicloudMachineClass *v1alpha1.AlicloudMachineClass
	CloudConfig          *corev1.Secret
	UserData             string
	MachineID            string
	MachineName          string
}

// runInstanceRequest is the request struct for api RunInstances
type runInstancesRequest struct {
	*requests.RpcRequest
	InstanceName            string           `position:"Query" name:"InstanceName"`
	Description             string           `position:"Query" name:"Description"`
	UserData                string           `position:"Query" name:"UserData"`
	ClusterID               string           `position:"Query" name:"ClusterId"`
	ClientToken             string           `position:"Query" name:"ClientToken"`
	ImageName               string           `position:"Query" name:"ImageId"`
	InstanceType            string           `position:"Query" name:"InstanceType"`
	Zone                    string           `position:"Query" name:"ZoneId"`
	SecurityGroupID         string           `position:"Query" name:"SecurityGroupId"`
	VSwitchID               string           `position:"Query" name:"VSwitchId"`
	PrivateIPAddress        string           `position:"Query" name:"PrivateIpAddress"`
	InnerIPAddress          string           `position:"Query" name:"InnerIpAddress"`
	SystemDiskSize          requests.Integer `position:"Query" name:"SystemDisk.Size"`
	SystemDiskCategory      string           `position:"Query" name:"SystemDisk.Category"`
	InstanceChargeType      string           `position:"Query" name:"InstanceChargeType"`
	InternetChargeType      string           `position:"Query" name:"InternetChargeType"`
	InternetMaxBandwidthIn  requests.Integer `position:"Query" name:"InternetMaxBandwidthIn"`
	InternetMaxBandwidthOut requests.Integer `position:"Query" name:"InternetMaxBandwidthOut"`
	SpotStrategy            string           `position:"Query" name:"SpotStrategy"`
	IoOptimized             string           `position:"Query" name:"IoOptimized"`
	Tag1Key                 string           `position:"Query" name:"Tag.1.Key"`
	Tag2Key                 string           `position:"Query" name:"Tag.2.Key"`
	Tag3Key                 string           `position:"Query" name:"Tag.3.Key"`
	Tag4Key                 string           `position:"Query" name:"Tag.4.Key"`
	Tag5Key                 string           `position:"Query" name:"Tag.5.Key"`
	Tag6Key                 string           `position:"Query" name:"Tag.6.Key"`
	Tag7Key                 string           `position:"Query" name:"Tag.7.Key"`
	Tag8Key                 string           `position:"Query" name:"Tag.8.Key"`
	Tag9Key                 string           `position:"Query" name:"Tag.9.Key"`
	Tag10Key                string           `position:"Query" name:"Tag.10.Key"`
	Tag1Value               string           `position:"Query" name:"Tag.1.Value"`
	Tag2Value               string           `position:"Query" name:"Tag.2.Value"`
	Tag3Value               string           `position:"Query" name:"Tag.3.Value"`
	Tag4Value               string           `position:"Query" name:"Tag.4.Value"`
	Tag5Value               string           `position:"Query" name:"Tag.5.Value"`
	Tag6Value               string           `position:"Query" name:"Tag.6.Value"`
	Tag7Value               string           `position:"Query" name:"Tag.7.Value"`
	Tag8Value               string           `position:"Query" name:"Tag.8.Value"`
	Tag9Value               string           `position:"Query" name:"Tag.9.Value"`
	Tag10Value              string           `position:"Query" name:"Tag.10.Value"`
	KeyPairName             string           `position:"Query" name:"KeyPairName"`
	DryRun                  requests.Boolean `position:"Query" name:"DryRun"`
}

// createRunInstancesRequest creates a request to invoke RunInstances API
func (c *AlicloudDriver) createRunInstancesRequest() (request *runInstancesRequest) {
	request = &runInstancesRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("Ecs", "2014-05-26", "RunInstances", "ecs", "openAPI")
	return
}

// RunInstances invokes the ecs.RunInstances API synchronously
// api document: https://help.aliyun.com/api/ecs/runinstances.html
func (c *AlicloudDriver) runInstances(client *ecs.Client, request *runInstancesRequest) (response *ecs.RunInstancesResponse, err error) {
	response = ecs.CreateRunInstancesResponse()
	err = client.DoAction(request, response)
	return
}

// Create is used to create a VM
func (c *AlicloudDriver) Create() (string, string, error) {
	client, err := c.getEcsClient()
	if err != nil {
		return "", "", err
	}

	request := c.createRunInstancesRequest()
	//request.DryRun = requests.NewBoolean(true)

	request.ImageName = c.AlicloudMachineClass.Spec.ImageName
	request.InstanceType = c.AlicloudMachineClass.Spec.InstanceType
	request.RegionId = c.AlicloudMachineClass.Spec.Region
	request.Zone = c.AlicloudMachineClass.Spec.Zone
	request.SecurityGroupID = c.AlicloudMachineClass.Spec.SecurityGroupID
	request.VSwitchID = c.AlicloudMachineClass.Spec.VSwitchID
	request.PrivateIPAddress = c.AlicloudMachineClass.Spec.PrivateIPAddress
	request.InstanceChargeType = c.AlicloudMachineClass.Spec.InstanceChargeType
	request.InternetChargeType = c.AlicloudMachineClass.Spec.InternetChargeType
	request.SpotStrategy = c.AlicloudMachineClass.Spec.SpotStrategy
	request.IoOptimized = c.AlicloudMachineClass.Spec.IoOptimized
	request.KeyPairName = c.AlicloudMachineClass.Spec.KeyPairName

	if c.AlicloudMachineClass.Spec.InternetMaxBandwidthIn != nil {
		request.InternetMaxBandwidthIn = requests.NewInteger(*c.AlicloudMachineClass.Spec.InternetMaxBandwidthIn)
	}

	if c.AlicloudMachineClass.Spec.InternetMaxBandwidthOut != nil {
		request.InternetMaxBandwidthOut = requests.NewInteger(*c.AlicloudMachineClass.Spec.InternetMaxBandwidthOut)
	}

	if c.AlicloudMachineClass.Spec.SystemDisk != nil {
		request.SystemDiskCategory = c.AlicloudMachineClass.Spec.SystemDisk.Category
		request.SystemDiskSize = requests.NewInteger(c.AlicloudMachineClass.Spec.SystemDisk.Size)
	}

	for key, value := range c.AlicloudMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io/cluster/") {
			request.Tag1Key = key
			request.Tag1Value = value
		} else if strings.Contains(key, "kubernetes.io/role/") {
			request.Tag2Key = key
			request.Tag2Value = value
		} else if request.Tag3Key == "" {
			request.Tag3Key = key
			request.Tag3Value = value
		} else if request.Tag4Key == "" {
			request.Tag4Key = key
			request.Tag4Value = value
		} else if request.Tag5Key == "" {
			request.Tag5Key = key
			request.Tag5Value = value
		} else if request.Tag6Key == "" {
			request.Tag6Key = key
			request.Tag6Value = value
		} else if request.Tag7Key == "" {
			request.Tag7Key = key
			request.Tag7Value = value
		} else if request.Tag8Key == "" {
			request.Tag8Key = key
			request.Tag8Value = value
		} else if request.Tag9Key == "" {
			request.Tag9Key = key
			request.Tag9Value = value
		} else if request.Tag10Key == "" {
			request.Tag10Key = key
			request.Tag10Value = value
		}
	}

	request.InstanceName = c.MachineName
	request.ClientToken = utils.GetUUIDV4()
	request.UserData = base64.StdEncoding.EncodeToString([]byte(c.UserData))

	response, err := c.runInstances(client, request)
	if err != nil {
		return "", "", err
	}

	return c.encodeMachineID(c.AlicloudMachineClass.Spec.Region, response.InstanceIdSets.InstanceIdSet[0]), c.MachineName, nil
}

// Delete method is used to delete an alicloud machine
func (c *AlicloudDriver) Delete() error {
	result, err := c.getVMDetails(c.MachineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machineID
		glog.V(2).Infof("No VM matching the machineID found on the provider %q", c.MachineID)
	}

	if result[0].Status != "Running" && result[0].Status != "Stopped" {
		return errors.New("ec2 instance not in running/stopped state")
	}

	machineID := c.decodeMachineID(c.MachineID)

	client, err := c.getEcsClient()
	if err != nil {
		return err
	}

	err = c.deleteInstance(client, machineID)
	return err
}

func (c *AlicloudDriver) stopInstance(client *ecs.Client, machineID string) error {
	request := ecs.CreateStopInstanceRequest()
	request.InstanceId = machineID
	request.ConfirmStop = requests.NewBoolean(true)
	request.ForceStop = requests.NewBoolean(true)

	_, err := client.StopInstance(request)

	return err
}

func (c *AlicloudDriver) deleteInstance(client *ecs.Client, machineID string) error {
	request := ecs.CreateDeleteInstanceRequest()
	request.InstanceId = machineID
	request.Force = requests.NewBoolean(true)

	_, err := client.DeleteInstance(request)
	return err
}

// GetExisting method is used to get machineID for existing Alicloud machine
func (c *AlicloudDriver) GetExisting() (string, error) {
	return c.MachineID, nil
}

func (c *AlicloudDriver) getVMDetails(machineID string) ([]ecs.Instance, error) {
	searchClusterName := ""
	searchNodeRole := ""

	for key, _ := range c.AlicloudMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io/cluster/") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io/role/") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" || searchNodeRole == "" {
		return nil, nil
	}

	client, err := c.getEcsClient()
	if err != nil {
		return nil, err
	}

	request := ecs.CreateDescribeInstancesRequest()
	request.Tag1Key = searchClusterName
	request.Tag2Key = searchNodeRole

	if machineID != "" {
		machineID = c.decodeMachineID(machineID)
		request.InstanceIds = "[\"" + machineID + "\"]"
	}

	response, err := client.DescribeInstances(request)
	if err != nil {
		return nil, err
	}

	return response.Instances.Instance, nil
}

// GetVMs returns a VM matching the machineID
// If machineID is an empty string then it returns all matching instances
func (c *AlicloudDriver) GetVMs(machineID string) (VMs, error) {
	listOfVMs := make(map[string]string)

	instances, err := c.getVMDetails(machineID)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		machineName := instance.InstanceName
		listOfVMs[c.encodeMachineID(c.AlicloudMachineClass.Spec.Region, instance.InstanceId)] = machineName
	}

	return listOfVMs, nil
}

func (c *AlicloudDriver) encodeMachineID(region, machineID string) string {
	return fmt.Sprintf("aliyun:///%s/%s", region, machineID)
}

func (c *AlicloudDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}

func (c *AlicloudDriver) getEcsClient() (*ecs.Client, error) {
	accessKeyID := strings.TrimSpace(string(c.CloudConfig.Data[v1alpha1.AlicloudAccessKeyID]))
	accessKeySecret := strings.TrimSpace(string(c.CloudConfig.Data[v1alpha1.AlicloudAccessKeySecret]))
	region := c.AlicloudMachineClass.Spec.Region

	var ecsClient *ecs.Client
	var err error
	if accessKeyID != "" && accessKeySecret != "" && region != "" {
		ecsClient, err = ecs.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	} else {
		ecsClient, err = ecs.NewClient()
	}
	return ecsClient, err
}
