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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/pagination"
)

// OpenStackDriver is the driver struct for holding OS machine information
type OpenStackDriver struct {
	OpenStackMachineClass *v1alpha1.OpenStackMachineClass
	CloudConfig           *corev1.Secret
	UserData              string
	MachineID             string
	MachineName           string
}

// Create method is used to create an OS machine
func (d *OpenStackDriver) Create() (string, string, error) {

	client, err := d.createNovaClient()
	if err != nil {
		return "", "", err
	}

	flavorName := d.OpenStackMachineClass.Spec.FlavorName
	keyName := d.OpenStackMachineClass.Spec.KeyName
	imageName := d.OpenStackMachineClass.Spec.ImageName
	networkID := d.OpenStackMachineClass.Spec.NetworkID
	securityGroups := d.OpenStackMachineClass.Spec.SecurityGroups
	availabilityZone := d.OpenStackMachineClass.Spec.AvailabilityZone
	metadata := d.OpenStackMachineClass.Spec.Tags

	var createOpts servers.CreateOptsBuilder

	createOpts = &servers.CreateOpts{
		ServiceClient:    client,
		Name:             d.MachineName,
		FlavorName:       flavorName,
		ImageName:        imageName,
		Networks:         []servers.Network{{UUID: networkID}},
		SecurityGroups:   securityGroups,
		Metadata:         metadata,
		UserData:         []byte(d.UserData),
		AvailabilityZone: availabilityZone,
	}

	createOpts = &keypairs.CreateOptsExt{
		CreateOptsBuilder: createOpts,
		KeyName:           keyName,
	}

	glog.V(3).Infof("creating machine")
	server, err := servers.Create(client, createOpts).Extract()

	d.MachineID = d.encodeMachineID(d.OpenStackMachineClass.Spec.Region, server.ID)

	return d.MachineID, d.MachineName, err
}

// Delete method is used to delete an OS machine
func (d *OpenStackDriver) Delete() error {

	res, err := d.GetVMs(d.MachineID)
	if err != nil {
		return err
	} else if len(res) == 0 {
		// No running instance exists with the given machine-ID
		glog.V(2).Infof("No VM matching the machine-ID found on the provider %q", d.MachineID)
		return nil
	}

	machineID := d.decodeMachineID(d.MachineID)
	client, err := d.createNovaClient()
	if err != nil {
		return err
	}

	result := servers.Delete(client, machineID)
	if result.Err == nil {
		glog.V(3).Infof("Deleted machine with ID: %s", d.MachineID)
	} else {
		glog.Errorf("Failed to delete machine with ID: %s", d.MachineID)
	}

	return result.Err
}

// GetExisting method is used to get machineID for existing OS machine
func (d *OpenStackDriver) GetExisting() (string, error) {
	return d.MachineID, nil
}

// GetVMs returns a VM matching the machineID
// If machineID is an empty string then it returns all matching instances
func (d *OpenStackDriver) GetVMs(machineID string) (VMs, error) {
	listOfVMs := make(map[string]string)

	searchClusterName := ""
	searchNodeRole := ""

	for key := range d.OpenStackMachineClass.Spec.Tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			searchClusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			searchNodeRole = key
		}
	}

	if searchClusterName == "" || searchNodeRole == "" {
		return listOfVMs, nil
	}

	client, err := d.createNovaClient()
	if err != nil {
		glog.Errorf("Could not connect to NovaClient. Error Message - %s", err)
		return listOfVMs, err
	}

	// Retrieve a pager (i.e. a paginated collection)
	pager := servers.List(client, servers.ListOpts{})
	if pager.Err != nil {
		glog.Errorf("Could not list instances. Error Message - %s", err)
		return listOfVMs, err
	}

	// Define an anonymous function to be executed on each page's iteration
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := servers.ExtractServers(page)

		for _, server := range serverList {

			clusterName := ""
			nodeRole := ""

			for key := range server.Metadata {
				if strings.Contains(key, "kubernetes.io-cluster-") {
					clusterName = key
				} else if strings.Contains(key, "kubernetes.io-role-") {
					nodeRole = key
				}
			}

			if clusterName == searchClusterName && nodeRole == searchNodeRole {
				instanceID := d.encodeMachineID(d.OpenStackMachineClass.Spec.Region, server.ID)

				if machineID == "" {
					listOfVMs[instanceID] = server.Name
				} else if machineID == instanceID {
					listOfVMs[instanceID] = server.Name
					glog.V(3).Infof("Found machine with name: %q", server.Name)
					break
				}
			}

		}
		return true, err
	})

	return listOfVMs, err
}

// createNovaClient is used to create a Nova client
func (d *OpenStackDriver) createNovaClient() (*gophercloud.ServiceClient, error) {

	config := &tls.Config{}

	authURL, ok := d.CloudConfig.Data["authURL"]
	if !ok {
		return nil, fmt.Errorf("missing auth_url in secret")
	}
	username, ok := d.CloudConfig.Data["username"]
	if !ok {
		return nil, fmt.Errorf("missing username in secret")
	}
	password, ok := d.CloudConfig.Data["password"]
	if !ok {
		return nil, fmt.Errorf("missing password in secret")
	}
	domainName, ok := d.CloudConfig.Data["domainName"]
	if !ok {
		return nil, fmt.Errorf("missing domainName in secret")
	}
	region := d.OpenStackMachineClass.Spec.Region
	tenantName, ok := d.CloudConfig.Data["tenantName"]
	if !ok {
		return nil, fmt.Errorf("missing tenantName in secret")
	}

	caCert, ok := d.CloudConfig.Data["caCert"]
	if !ok {
		caCert = nil
	}

	insecure, ok := d.CloudConfig.Data["insecure"]
	if ok && strings.TrimSpace(string(insecure)) == "true" {
		config.InsecureSkipVerify = true
	}

	if caCert != nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(caCert))
		config.RootCAs = caCertPool
	}

	clientCert, ok := d.CloudConfig.Data["clientCert"]
	if ok {
		clientKey, ok := d.CloudConfig.Data["clientKey"]
		if ok {
			cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
			if err != nil {
				return nil, err
			}
			config.Certificates = []tls.Certificate{cert}
			config.BuildNameToCertificate()
		} else {
			return nil, fmt.Errorf("client key missing in secret")
		}
	}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: strings.TrimSpace(string(authURL)),
		Username:         strings.TrimSpace(string(username)),
		Password:         strings.TrimSpace(string(password)),
		DomainName:       strings.TrimSpace(string(domainName)),
		TenantName:       strings.TrimSpace(string(tenantName)),
	}

	client, err := openstack.NewClient(opts.IdentityEndpoint)
	if err != nil {
		return nil, err
	}

	// Set UserAgent
	client.UserAgent.Prepend("Machine Controller 08/15")

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: config}
	client.HTTPClient = http.Client{
		Transport: transport,
	}

	err = openstack.Authenticate(client, opts)
	if err != nil {
		return nil, err
	}

	return openstack.NewComputeV2(client, gophercloud.EndpointOpts{
		Region:       strings.TrimSpace(region),
		Availability: gophercloud.AvailabilityPublic,
	})
}

func (d *OpenStackDriver) encodeMachineID(region string, machineID string) string {
	return fmt.Sprintf("openstack:///%s/%s", region, machineID)
}

func (d *OpenStackDriver) decodeMachineID(id string) string {
	splitProviderID := strings.Split(id, "/")
	return splitProviderID[len(splitProviderID)-1]
}
