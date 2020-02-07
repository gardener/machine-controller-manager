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
	"fmt"
	"strings"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	metalgo "github.com/metal-pod/metal-go"
	"k8s.io/klog"
)

// MetalDriver is the driver struct for holding Metal machine information
type MetalDriver struct {
	MetalMachineClass *v1alpha1.MetalMachineClass
	CloudConfig       *corev1.Secret
	UserData          string
	MachineID         string
	MachineName       string
}

// NewMetalDriver returns an empty MetalDriver object
func NewMetalDriver(create func() (string, error), delete func() error, existing func() (string, error)) Driver {
	return &MetalDriver{}
}

// GetUserData return the used data whit which the VM will be booted
func (d *MetalDriver) GetUserData() string {
	return d.UserData
}

// SetUserData set the used data whit which the VM will be booted
func (d *MetalDriver) SetUserData(userData string) {
	d.UserData = userData
}

// Create method is used to create a Metal machine
func (d *MetalDriver) Create() (string, string, error) {

	svc, err := d.createSVC()
	if err != nil {
		return "", "", err
	}
	networks := []metalgo.MachineAllocationNetwork{
		{
			Autoacquire: true,
			NetworkID:   d.MetalMachineClass.Spec.Network,
		},
	}
	createRequest := &metalgo.MachineCreateRequest{
		Description:   d.MachineName + " created by Gardener.",
		Name:          d.MachineName,
		Hostname:      d.MachineName,
		UserData:      d.UserData,
		Size:          d.MetalMachineClass.Spec.Size,
		Project:       d.MetalMachineClass.Spec.Project,
		Networks:      networks,
		Partition:     d.MetalMachineClass.Spec.Partition,
		Image:         d.MetalMachineClass.Spec.Image,
		Tags:          d.MetalMachineClass.Spec.Tags,
		SSHPublicKeys: d.MetalMachineClass.Spec.SSHKeys,
	}

	mcr, err := svc.MachineCreate(createRequest)
	if err != nil {
		klog.Errorf("Could not create machine: %v", err)
		return "Error", "Error", err
	}
	return d.encodeMachineID(*mcr.Machine.Partition.ID, *mcr.Machine.ID), *mcr.Machine.Allocation.Name, nil
}

// Delete method is used to delete a Machine machine
func (d *MetalDriver) Delete(machineID string) error {

	svc, err := d.createSVC()
	if err != nil {
		return err
	}

	hostname := d.MachineName
	project := d.MetalMachineClass.Spec.Project
	partition := d.MetalMachineClass.Spec.Partition
	machine := d.decodeMachineID(machineID)
	networkID := d.MetalMachineClass.Spec.Network

	mfr := &metalgo.MachineFindRequest{
		ID:                 &machine,
		AllocationHostname: &hostname,
		AllocationProject:  &project,
		NetworkIDs:         []string{networkID},
		PartitionID:        &partition,
	}
	m, err := svc.MachineFind(mfr)
	if err != nil {
		klog.Errorf("Error searching machine %s: in project:%s partition: %s hostname: %s %v", d.MachineID, project, partition, hostname, err)
		return err
	}
	if len(m.Machines) > 1 {
		errMsg := fmt.Sprintf("Error searching machine %s: in project: %s partition: %s hostname: %s, more than one search result %d", d.MachineID, project, partition, hostname, len(m.Machines))
		klog.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}
	if len(m.Machines) == 0 {
		klog.Infof("no machine %s: in project:%s partition: %s hostname: %s found", d.MachineID, project, partition, hostname)
		return nil
	}
	id := m.Machines[0].ID
	_, err = svc.MachineDelete(*id)
	if err != nil {
		klog.Errorf("Could not terminate machine %s: %v", d.MachineID, err)
		return err
	}
	return nil
}

// GetExisting method is used to get machineID for existing Metal machine
func (d *MetalDriver) GetExisting() (string, error) {
	return d.MachineID, nil
}

// GetVMs returns a machine matching the machineID
// If machineID is an empty string then it returns all matching instances
func (d *MetalDriver) GetVMs(machineID string) (VMs, error) {
	listOfVMs := make(map[string]string)

	clusterName := ""
	nodeRole := ""

	for _, key := range d.MetalMachineClass.Spec.Tags {
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
		return nil, err
	}
	if machineID == "" {
		findRequest := &metalgo.MachineFindRequest{
			AllocationProject: &d.MetalMachineClass.Spec.Project,
			PartitionID:       &d.MetalMachineClass.Spec.Partition,
			NetworkIDs:        []string{d.MetalMachineClass.Spec.Network},
		}
		mlr, err := svc.MachineFind(findRequest)
		if err != nil {
			klog.Errorf("Could not list machines for project %s in partition:%s networkID:%s : %v",
				d.MetalMachineClass.Spec.Project, d.MetalMachineClass.Spec.Partition, d.MetalMachineClass.Spec.Network, err)
			return nil, err
		}
		for _, m := range mlr.Machines {
			matchedCluster := false
			matchedRole := false
			for _, tag := range m.Tags {
				switch tag {
				case clusterName:
					matchedCluster = true
				case nodeRole:
					matchedRole = true
				}
			}
			if matchedCluster && matchedRole {
				listOfVMs[*m.ID] = *m.Allocation.Hostname
			}
		}
	} else {
		machineID = d.decodeMachineID(machineID)
		mgr, err := svc.MachineGet(machineID)
		if err != nil {
			klog.Errorf("Could not get machine %s: %v", machineID, err)
			return nil, err
		}
		listOfVMs[machineID] = *mgr.Machine.Allocation.Hostname
	}
	return listOfVMs, nil
}

// GetVolNames parses volume names from pv specs
func (d *MetalDriver) GetVolNames(specs []corev1.PersistentVolumeSpec) ([]string, error) {
	names := []string{}
	return names, fmt.Errorf("Not implemented yet")
}

// Helper function to create SVC
func (d *MetalDriver) createSVC() (*metalgo.Driver, error) {
	token := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.MetalAPIKey]))
	hmac := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.MetalAPIHMac]))

	u, ok := d.CloudConfig.Data[v1alpha1.MetalAPIURL]
	if !ok {
		return nil, fmt.Errorf("missing %s in secret", v1alpha1.MetalAPIURL)
	}
	url := strings.TrimSpace(string(u))

	return metalgo.NewDriver(url, token, hmac)
}

func (d *MetalDriver) encodeMachineID(partition, machineID string) string {
	return fmt.Sprintf("metal:///%s/%s", partition, machineID)
}

func (d *MetalDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}
