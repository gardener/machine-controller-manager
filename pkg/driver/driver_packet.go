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

	"github.com/golang/glog"
	"github.com/packethost/packngo"
)

// PacketDriver is the driver struct for holding Packet machine information
type PacketDriver struct {
	PacketMachineClass *v1alpha1.PacketMachineClass
	CloudConfig        *corev1.Secret
	UserData           string
	MachineID          string
	MachineName        string
}

// NewPacketDriver returns an empty PacketDriver object
func NewPacketDriver(create func() (string, error), delete func() error, existing func() (string, error)) Driver {
	return &PacketDriver{}
}

// Create method is used to create a Packet machine
func (d *PacketDriver) Create() (string, string, error) {

	svc := d.createSVC()
	// packet tags are strings only
	tags := packetTagsMapToString(d.PacketMachineClass.Spec.Tags)
	createRequest := &packngo.DeviceCreateRequest{
		UserData:     d.UserData,
		Plan:         d.PacketMachineClass.Spec.MachineType,
		ProjectID:    d.PacketMachineClass.Spec.ProjectID,
		BillingCycle: d.PacketMachineClass.Spec.BillingCycle,
		Facility:     d.PacketMachineClass.Spec.Facility,
		OS:           d.PacketMachineClass.Spec.OS,
		Tags:         tags,
	}

	device, _, err := svc.Devices.Create(createRequest)
	if err != nil {
		glog.Errorf("Could not create machine: %v", err)
		return "", "", err
	}
	return d.encodeMachineID(device.Facility.ID, device.ID), device.Hostname, nil
}

// Delete method is used to delete a Packet machine
func (d *PacketDriver) Delete() error {

	svc := d.createSVC()
	machineID := d.decodeMachineID(d.MachineID)
	resp, err := svc.Devices.Delete(machineID)
	if err != nil {
		if resp.StatusCode == 404 {
			glog.V(2).Infof("No machine matching the machine-ID found on the provider %q", d.MachineID)
			return nil
		}
		glog.Errorf("Could not terminate machine %s: %v", d.MachineID, err)
		return err
	}
	return nil
}

// GetExisting method is used to get machineID for existing Packet machine
func (d *PacketDriver) GetExisting() (string, error) {
	return d.MachineID, nil
}

// GetVMs returns a machine matching the machineID
// If machineID is an empty string then it returns all matching instances
func (d *PacketDriver) GetVMs(machineID string) (VMs, error) {
	listOfVMs := make(map[string]string)

	clusterName := ""
	nodeRole := ""

	for key := range d.PacketMachineClass.Spec.Tags {
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
	if machineID == "" {
		devices, _, err := svc.Devices.List(d.PacketMachineClass.Spec.ProjectID, &packngo.ListOptions{})
		if err != nil {
			glog.Errorf("Could not list devices for project %s: %v", d.PacketMachineClass.Spec.ProjectID, err)
			return nil, err
		}
		for _, d := range devices {
			tags := packetTagsStringToMap(d.Tags)
			if v, ok := tags[clusterName]; ok && v == nodeRole {
				listOfVMs[d.ID] = d.Hostname
			}
		}
	} else {
		machineID = d.decodeMachineID(machineID)
		device, _, err := svc.Devices.Get(machineID, &packngo.GetOptions{})
		if err != nil {
			glog.Errorf("Could not get device %s: %v", machineID, err)
			return nil, err
		}
		listOfVMs[machineID] = device.Hostname
	}
	return listOfVMs, nil
}

// Helper function to create SVC
func (d *PacketDriver) createSVC() *packngo.Client {

	token := strings.TrimSpace(string(d.CloudConfig.Data[v1alpha1.PacketAPIKey]))

	if token != "" {
		return packngo.NewClientWithAuth("gardener", token, nil)
	}

	return nil
}

func (d *PacketDriver) encodeMachineID(facility, machineID string) string {
	return fmt.Sprintf("packet:///%s/%s", facility, machineID)
}

func (d *PacketDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}

func packetTagsMapToString(tags map[string]string) []string {
	ret := make([]string, 0)
	for k, v := range tags {
		ret = append(ret, fmt.Sprintf("%s:%s", k, v))
	}
	return ret
}

func packetTagsStringToMap(tags []string) map[string]string {
	ret := map[string]string{}
	for _, t := range tags {
		parts := strings.SplitN(t, ":", 2)
		ret[parts[0]] = parts[1]
	}
	return ret
}

// GetVolNames parses volume names from pv specs
func (d *PacketDriver) GetVolNames(specs []corev1.PersistentVolumeSpec) ([]string, error) {
	names := []string{}
	return names, fmt.Errorf("Not implemented yet")
}
