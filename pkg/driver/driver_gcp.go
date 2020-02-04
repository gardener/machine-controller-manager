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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

// GCPDriver is the driver struct for holding GCP machine information
type GCPDriver struct {
	GCPMachineClass *v1alpha1.GCPMachineClass
	CloudConfig     *corev1.Secret
	UserData        string
	MachineID       string
	MachineName     string
}

// Create method is used to create a GCP machine
func (d *GCPDriver) Create() (string, string, error) {
	ctx, computeService, err := d.createComputeService()
	if err != nil {
		return "Error", "Error", err
	}

	project, err := extractProject(d.CloudConfig.Data[v1alpha1.GCPServiceAccountJSON])
	if err != nil {
		return "Error", "Error", err
	}

	var (
		zone = d.GCPMachineClass.Spec.Zone

		instance = &compute.Instance{
			CanIpForward:       d.GCPMachineClass.Spec.CanIpForward,
			DeletionProtection: d.GCPMachineClass.Spec.DeletionProtection,
			Labels:             d.GCPMachineClass.Spec.Labels,
			MachineType:        fmt.Sprintf("zones/%s/machineTypes/%s", zone, d.GCPMachineClass.Spec.MachineType),
			Name:               d.MachineName,
			Scheduling: &compute.Scheduling{
				AutomaticRestart:  &d.GCPMachineClass.Spec.Scheduling.AutomaticRestart,
				OnHostMaintenance: d.GCPMachineClass.Spec.Scheduling.OnHostMaintenance,
				Preemptible:       d.GCPMachineClass.Spec.Scheduling.Preemptible,
			},
			Tags: &compute.Tags{
				Items: d.GCPMachineClass.Spec.Tags,
			},
		}
	)

	if d.GCPMachineClass.Spec.Description != nil {
		instance.Description = *d.GCPMachineClass.Spec.Description
	}

	var disks = []*compute.AttachedDisk{}
	for _, disk := range d.GCPMachineClass.Spec.Disks {
		disks = append(disks, &compute.AttachedDisk{
			AutoDelete: disk.AutoDelete,
			Boot:       disk.Boot,
			InitializeParams: &compute.AttachedDiskInitializeParams{
				DiskSizeGb:  disk.SizeGb,
				DiskType:    fmt.Sprintf("zones/%s/diskTypes/%s", zone, disk.Type),
				Labels:      disk.Labels,
				SourceImage: disk.Image,
			},
		})
	}
	instance.Disks = disks

	var metadataItems = []*compute.MetadataItems{}
	metadataItems = append(metadataItems, d.getUserData())

	for _, metadata := range d.GCPMachineClass.Spec.Metadata {
		metadataItems = append(metadataItems, &compute.MetadataItems{
			Key:   metadata.Key,
			Value: metadata.Value,
		})
	}
	instance.Metadata = &compute.Metadata{
		Items: metadataItems,
	}

	var networkInterfaces = []*compute.NetworkInterface{}
	for _, nic := range d.GCPMachineClass.Spec.NetworkInterfaces {
		computeNIC := &compute.NetworkInterface{}

		if nic.DisableExternalIP == false {
			// When DisableExternalIP is false, implies Attach an external IP to VM
			computeNIC.AccessConfigs = []*compute.AccessConfig{{}}
		}
		if len(nic.Network) != 0 {
			computeNIC.Network = fmt.Sprintf("projects/%s/global/networks/%s", project, nic.Network)
		}
		if len(nic.Subnetwork) != 0 {
			computeNIC.Subnetwork = fmt.Sprintf("regions/%s/subnetworks/%s", d.GCPMachineClass.Spec.Region, nic.Subnetwork)
		}
		networkInterfaces = append(networkInterfaces, computeNIC)
	}
	instance.NetworkInterfaces = networkInterfaces

	var serviceAccounts = []*compute.ServiceAccount{}
	for _, sa := range d.GCPMachineClass.Spec.ServiceAccounts {
		serviceAccounts = append(serviceAccounts, &compute.ServiceAccount{
			Email:  sa.Email,
			Scopes: sa.Scopes,
		})
	}
	instance.ServiceAccounts = serviceAccounts

	operation, err := computeService.Instances.Insert(project, zone, instance).Context(ctx).Do()
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "gcp", "service": "compute"}).Inc()
		return "Error", "Error", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "gcp", "service": "compute"}).Inc()

	if err := waitUntilOperationCompleted(computeService, project, zone, operation.Name); err != nil {
		return "Error", "Error", err
	}

	return d.encodeMachineID(project, zone, d.MachineName), d.MachineName, nil
}

// Delete method is used to delete a GCP machine
func (d *GCPDriver) Delete(machineID string) error {

	result, err := d.GetVMs(machineID)
	if err != nil {
		return err
	} else if len(result) == 0 {
		// No running instance exists with the given machine-ID
		klog.V(2).Infof("No VM matching the machine-ID found on the provider %q", machineID)
		return nil
	}

	ctx, computeService, err := d.createComputeService()
	if err != nil {
		klog.Error(err)
		return err
	}

	project, zone, name, err := d.decodeMachineID(machineID)
	if err != nil {
		klog.Error(err)
		return err
	}

	operation, err := computeService.Instances.Delete(project, zone, name).Context(ctx).Do()
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "gcp", "service": "compute"}).Inc()
		if ae, ok := err.(*googleapi.Error); ok && ae.Code == http.StatusNotFound {
			return nil
		}
		klog.Error(err)
		return err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "gcp", "service": "compute"}).Inc()

	return waitUntilOperationCompleted(computeService, project, zone, operation.Name)
}

func (d *GCPDriver) getUserData() *compute.MetadataItems {
	if strings.HasPrefix(d.UserData, "#cloud-config") {
		return &compute.MetadataItems{
			Key:   "user-data",
			Value: &d.UserData,
		}
	}

	return &compute.MetadataItems{
		Key:   "startup-script",
		Value: &d.UserData,
	}
}

// GetExisting method is used to get machineID for existing GCP machine
func (d *GCPDriver) GetExisting() (string, error) {
	return d.MachineID, nil
}

// GetVMs returns a list of VMs
func (d *GCPDriver) GetVMs(machineID string) (VMs, error) {
	listOfVMs := make(map[string]string)

	searchClusterName := ""
	searchNodeRole := ""

	for _, key := range d.GCPMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes-io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes-io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" || searchNodeRole == "" {
		return listOfVMs, nil
	}

	ctx, computeService, err := d.createComputeService()
	if err != nil {
		klog.Error(err)
		return listOfVMs, err
	}

	project, err := extractProject(d.CloudConfig.Data[v1alpha1.GCPServiceAccountJSON])
	if err != nil {
		klog.Error(err)
		return listOfVMs, err
	}

	zone := d.GCPMachineClass.Spec.Zone

	req := computeService.Instances.List(project, zone)
	if err := req.Pages(ctx, func(page *compute.InstanceList) error {
		for _, server := range page.Items {
			clusterName := ""
			nodeRole := ""

			for _, key := range server.Tags.Items {
				if strings.Contains(key, "kubernetes-io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes-io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {
				instanceID := d.encodeMachineID(project, zone, server.Name)

				if machineID == "" {
					listOfVMs[instanceID] = server.Name
				} else if machineID == instanceID {
					listOfVMs[instanceID] = server.Name
					klog.V(3).Infof("Found machine with name: %q", server.Name)
					break
				}
			}
		}
		return nil
	}); err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "gcp", "service": "compute"}).Inc()
		klog.Error(err)
		return listOfVMs, err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "gcp", "service": "compute"}).Inc()

	return listOfVMs, nil
}

func (d *GCPDriver) createComputeService() (context.Context, *compute.Service, error) {
	ctx := context.Background()

	jwt, err := google.JWTConfigFromJSON(d.CloudConfig.Data[v1alpha1.GCPServiceAccountJSON], compute.CloudPlatformScope)
	if err != nil {
		return nil, nil, err
	}

	oauthClient := oauth2.NewClient(ctx, jwt.TokenSource(ctx))
	computeService, err := compute.New(oauthClient)
	if err != nil {
		return nil, nil, err
	}

	return ctx, computeService, nil
}

func waitUntilOperationCompleted(computeService *compute.Service, project, zone, operationName string) error {
	return wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		op, err := computeService.ZoneOperations.Get(project, zone, operationName).Do()
		if err != nil {
			return false, err
		}
		klog.V(3).Infof("Waiting for operation to be completed... (status: %s)", op.Status)
		if op.Status == "DONE" {
			if op.Error == nil {
				return true, nil
			}
			var err []error
			for _, opErr := range op.Error.Errors {
				err = append(err, fmt.Errorf("%s", *opErr))
			}
			return false, fmt.Errorf("The following errors occurred: %+v", err)
		}
		return false, nil
	})
}

func (d *GCPDriver) encodeMachineID(project, zone, name string) string {
	return fmt.Sprintf("gce:///%s/%s/%s", project, zone, name)
}

func (d *GCPDriver) decodeMachineID(id string) (string, string, string, error) {
	gceSplit := strings.Split(id, "gce:///")
	if len(gceSplit) != 2 {
		return "", "", "", fmt.Errorf("Invalid format of machine id: %s", id)
	}

	gce := strings.Split(gceSplit[1], "/")
	if len(gce) != 3 {
		return "", "", "", fmt.Errorf("Invalid format of machine id: %s", id)
	}

	return gce[0], gce[1], gce[2], nil
}

func extractProject(serviceaccount []byte) (string, error) {
	var j struct {
		Project string `json:"project_id"`
	}
	if err := json.Unmarshal(serviceaccount, &j); err != nil {
		return "Error", err
	}
	return j.Project, nil
}

// GetVolNames parses volume names from pv specs
func (d *GCPDriver) GetVolNames(specs []corev1.PersistentVolumeSpec) ([]string, error) {
	names := []string{}
	for i := range specs {
		spec := &specs[i]
		if spec.GCEPersistentDisk == nil {
			// Not a GCE volume
			continue
		}
		name := spec.GCEPersistentDisk.PDName
		names = append(names, name)
	}
	return names, nil
}

//GetUserData return the used data whit which the VM will be booted
func (d *GCPDriver) GetUserData() string {
	return d.UserData
}

//SetUserData set the used data whit which the VM will be booted
func (d *GCPDriver) SetUserData(userData string) {
	d.UserData = userData
}
