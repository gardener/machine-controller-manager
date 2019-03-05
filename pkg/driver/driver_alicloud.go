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

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
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

// Create is used to create a VM
func (c *AlicloudDriver) Create() (string, string, error) {
	client, err := c.getEcsClient()
	if err != nil {
		return "", "", err
	}

	request := ecs.CreateRunInstancesRequest()
	//request.DryRun = requests.NewBoolean(true)

	request.ImageId = c.AlicloudMachineClass.Spec.ImageID
	request.InstanceType = c.AlicloudMachineClass.Spec.InstanceType
	request.RegionId = c.AlicloudMachineClass.Spec.Region
	request.ZoneId = c.AlicloudMachineClass.Spec.ZoneID
	request.SecurityGroupId = c.AlicloudMachineClass.Spec.SecurityGroupID
	request.VSwitchId = c.AlicloudMachineClass.Spec.VSwitchID
	request.PrivateIpAddress = c.AlicloudMachineClass.Spec.PrivateIPAddress
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
		request.SystemDiskSize = fmt.Sprintf("%d", c.AlicloudMachineClass.Spec.SystemDisk.Size)
	}

	tags, err := c.toInstanceTags(c.AlicloudMachineClass.Spec.Tags)
	if err != nil {
		return "", "", err
	}
	request.Tag = &tags
	request.InstanceName = c.MachineName
	request.ClientToken = utils.GetUUIDV4()
	request.UserData = base64.StdEncoding.EncodeToString([]byte(c.UserData))

	response, err := client.RunInstances(request)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()
		return "", "", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()

	instanceID := response.InstanceIdSets.InstanceIdSet[0]
	machineID := c.encodeMachineID(c.AlicloudMachineClass.Spec.Region, instanceID)

	// Hostname can't be fetched immediately from Alicloud API.
	// Even using DescribeInstances by Instance ID, it will return empty.
	// However, for Alicloud hostname, it can be transformed by Instance ID by default
	// Please be noted that returned node name should be in LOWER case
	return machineID, strings.ToLower(c.idToName(instanceID)), nil
}

// Delete method is used to delete an alicloud machine
func (c *AlicloudDriver) Delete() error {
	result, err := c.getVMDetails(c.MachineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machineID
		glog.V(2).Infof("No VM matching the machineID found on the provider %q", c.MachineID)
		return nil
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
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()

	return err
}

func (c *AlicloudDriver) deleteInstance(client *ecs.Client, machineID string) error {
	request := ecs.CreateDeleteInstanceRequest()
	request.InstanceId = machineID
	request.Force = requests.NewBoolean(true)

	_, err := client.DeleteInstance(request)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()
	return err
}

// GetExisting method is used to get machineID for existing Alicloud machine
func (c *AlicloudDriver) GetExisting() (string, error) {
	return c.MachineID, nil
}

func (c *AlicloudDriver) getVMDetails(machineID string) ([]ecs.Instance, error) {
	client, err := c.getEcsClient()
	if err != nil {
		return nil, err
	}

	request := ecs.CreateDescribeInstancesRequest()

	if machineID != "" {
		machineID = c.decodeMachineID(machineID)
		request.InstanceIds = "[\"" + machineID + "\"]"
	} else {
		searchClusterName := ""
		searchNodeRole := ""
		searchClusterNameValue := ""
		searchNodeRoleValue := ""

		for k, v := range c.AlicloudMachineClass.Spec.Tags {
			if strings.Contains(k, "kubernetes.io/cluster/") {
				searchClusterName = k
				searchClusterNameValue = v
			} else if strings.Contains(k, "kubernetes.io/role/") {
				searchNodeRole = k
				searchNodeRoleValue = v
			}
		}

		if searchClusterName == "" || searchNodeRole == "" {
			return nil, fmt.Errorf("Can't find VMs with none of machineID/Tag[kubernetes.io/cluster/*]/Tag[kubernetes.io/role/*]")
		}

		request.Tag = &[]ecs.DescribeInstancesTag{
			{Key: searchClusterName, Value: searchClusterNameValue},
			{Key: searchNodeRole, Value: searchNodeRoleValue},
		}
	}

	response, err := client.DescribeInstances(request)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()
		return nil, err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "alicloud", "service": "ecs"}).Inc()

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
	return fmt.Sprintf("%s.%s", region, machineID)
}

func (c *AlicloudDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, ".")
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
		err = errors.New("alicloudAccessKeyID or alicloudAccessKeySecret can't be empty")
	}
	return ecsClient, err
}

// Host name in Alicloud has relationship with Instance ID
// i-uf69zddmom11ci7est12 => iZuf69zddmom11ci7est12Z
func (c *AlicloudDriver) idToName(instanceID string) string {
	return strings.Replace(instanceID, "-", "Z", 1) + "Z"
}

func (c *AlicloudDriver) toInstanceTags(tags map[string]string) ([]ecs.RunInstancesTag, error) {
	result := []ecs.RunInstancesTag{{}, {}}
	hasCluster := false
	hasRole := false

	for k, v := range tags {
		if strings.Contains(k, "kubernetes.io/cluster/") {
			hasCluster = true
			result[0].Key = k
			result[0].Value = v
		} else if strings.Contains(k, "kubernetes.io/role/") {
			hasRole = true
			result[1].Key = k
			result[1].Value = v
		} else {
			result = append(result, ecs.RunInstancesTag{Key: k, Value: v})
		}
	}

	if !hasCluster || !hasRole {
		err := fmt.Errorf("Tags should at least contains 2 keys, which are prefixed with kubernetes.io/cluster and kubernetes.io/role")
		return nil, err
	}

	return result, nil
}
