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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
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
	podNetworkCidr := d.OpenStackMachineClass.Spec.PodNetworkCidr

	var createOpts servers.CreateOptsBuilder

	imageRef, err := d.recentImageIDFromName(client, imageName)
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		return "", "", fmt.Errorf("failed to get image id for image name %s: %s", imageName, err)
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()

	createOpts = &servers.CreateOpts{
		ServiceClient:    client,
		Name:             d.MachineName,
		FlavorName:       flavorName,
		ImageRef:         imageRef,
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
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		return "", "", fmt.Errorf("error creating the server: %s", err)
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()

	d.MachineID = d.encodeMachineID(d.OpenStackMachineClass.Spec.Region, server.ID)

	nwClient, err := d.createNeutronClient()
	if err != nil {
		return "", "", err
	}

	err = waitForStatus(client, server.ID, []string{"BUILD"}, []string{"ACTIVE"}, 600)
	if err != nil {
		return "", "", fmt.Errorf("error waiting for the %q server status: %s", server.ID, err)
	}

	listOpts := &ports.ListOpts{
		NetworkID: networkID,
		DeviceID:  server.ID,
	}

	allPages, err := ports.List(nwClient, listOpts).AllPages()
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()
		return "", "", fmt.Errorf("failed to get ports for network ID %s: %s", networkID, err)
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()

	allPorts, err := ports.ExtractPorts(allPages)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract ports for network ID %s: %s", networkID, err)
	}

	if len(allPorts) == 0 {
		return "", "", fmt.Errorf("got an empty port list for network ID %s and server ID %s", networkID, server.ID)
	}

	port, err := ports.Update(nwClient, allPorts[0].ID, ports.UpdateOpts{
		AllowedAddressPairs: &[]ports.AddressPair{{IPAddress: podNetworkCidr}},
	}).Extract()
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()
		return "", "", fmt.Errorf("failed to update allowed address pair for port ID %s: %s", port.ID, err)
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()

	return d.MachineID, d.MachineName, nil
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
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		glog.V(3).Infof("Deleted machine with ID: %s", d.MachineID)
	} else {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
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
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		glog.Errorf("Could not list instances. Error Message - %s", err)
		return listOfVMs, err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()

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

	region := d.OpenStackMachineClass.Spec.Region

	client, err := d.createOpenStackClient()
	if err != nil {
		return nil, err
	}

	return openstack.NewComputeV2(client, gophercloud.EndpointOpts{
		Region:       strings.TrimSpace(region),
		Availability: gophercloud.AvailabilityPublic,
	})
}

// createOpenStackClient creates and authenticates a base OpenStack client
func (d *OpenStackDriver) createOpenStackClient() (*gophercloud.ProviderClient, error) {
	config := &tls.Config{}

	authURL, ok := d.CloudConfig.Data[v1alpha1.OpenStackAuthURL]
	if !ok {
		return nil, fmt.Errorf("missing %s in secret", v1alpha1.OpenStackAuthURL)
	}
	username, ok := d.CloudConfig.Data[v1alpha1.OpenStackUsername]
	if !ok {
		return nil, fmt.Errorf("missing %s in secret", v1alpha1.OpenStackUsername)
	}
	password, ok := d.CloudConfig.Data[v1alpha1.OpenStackPassword]
	if !ok {
		return nil, fmt.Errorf("missing %s in secret", v1alpha1.OpenStackPassword)
	}

	// optional OS_USER_DOMAIN_NAME
	userDomainName := d.CloudConfig.Data[v1alpha1.OpenStackUserDomainName]
	// optional OS_USER_DOMAIN_ID
	userDomainID := d.CloudConfig.Data[v1alpha1.OpenStackUserDomainID]

	domainName, ok := d.CloudConfig.Data[v1alpha1.OpenStackDomainName]
	domainID, ok2 := d.CloudConfig.Data[v1alpha1.OpenStackDomainID]
	if !ok && !ok2 {
		return nil, fmt.Errorf("missing %s or %s in secret", v1alpha1.OpenStackDomainName, v1alpha1.OpenStackDomainID)
	}
	tenantName, ok := d.CloudConfig.Data[v1alpha1.OpenStackTenantName]
	tenantID, ok2 := d.CloudConfig.Data[v1alpha1.OpenStackTenantID]
	if !ok && !ok2 {
		return nil, fmt.Errorf("missing %s or %s in secret", v1alpha1.OpenStackTenantName, v1alpha1.OpenStackTenantID)
	}

	caCert, ok := d.CloudConfig.Data[v1alpha1.OpenStackCACert]
	if !ok {
		caCert = nil
	}

	insecure, ok := d.CloudConfig.Data[v1alpha1.OpenStackInsecure]
	if ok && strings.TrimSpace(string(insecure)) == "true" {
		config.InsecureSkipVerify = true
	}

	if caCert != nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(caCert))
		config.RootCAs = caCertPool
	}

	clientCert, ok := d.CloudConfig.Data[v1alpha1.OpenStackClientCert]
	if ok {
		clientKey, ok := d.CloudConfig.Data[v1alpha1.OpenStackClientKey]
		if ok {
			cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
			if err != nil {
				return nil, err
			}
			config.Certificates = []tls.Certificate{cert}
			config.BuildNameToCertificate()
		} else {
			return nil, fmt.Errorf("%s missing in secret", v1alpha1.OpenStackClientKey)
		}
	}

	clientOpts := new(clientconfig.ClientOpts)
	authInfo := &clientconfig.AuthInfo{
		AuthURL:        strings.TrimSpace(string(authURL)),
		Username:       strings.TrimSpace(string(username)),
		Password:       strings.TrimSpace(string(password)),
		DomainName:     strings.TrimSpace(string(domainName)),
		DomainID:       strings.TrimSpace(string(domainID)),
		ProjectName:    strings.TrimSpace(string(tenantName)),
		ProjectID:      strings.TrimSpace(string(tenantID)),
		UserDomainName: strings.TrimSpace(string(userDomainName)),
		UserDomainID:   strings.TrimSpace(string(userDomainID)),
	}
	clientOpts.AuthInfo = authInfo

	ao, err := clientconfig.AuthOptions(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create client auth options: %+v", err)
	}

	client, err := openstack.NewClient(ao.IdentityEndpoint)
	if err != nil {
		return nil, err
	}

	// Set UserAgent
	client.UserAgent.Prepend("Machine Controller 08/15")

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: config}
	client.HTTPClient = http.Client{
		Transport: transport,
	}

	err = openstack.Authenticate(client, *ao)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// createNeutronClient is used to create a Neutron client
func (d *OpenStackDriver) createNeutronClient() (*gophercloud.ServiceClient, error) {

	region := d.OpenStackMachineClass.Spec.Region

	client, err := d.createOpenStackClient()
	if err != nil {
		return nil, err
	}

	return openstack.NewNetworkV2(client, gophercloud.EndpointOpts{
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

func (d *OpenStackDriver) recentImageIDFromName(client *gophercloud.ServiceClient, imageName string) (string, error) {
	allPages, err := images.ListDetail(client, nil).AllPages()
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		return "", err
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
	all, err := images.ExtractImages(allPages)
	if err != nil {
		return "", err
	}
	for _, f := range all {
		if f.Name == imageName {
			return f.ID, nil
		}
	}
	return "", fmt.Errorf("could not find an image id for image name %s", imageName)
}

// GetVolNames parses volume names from pv specs
func (d *OpenStackDriver) GetVolNames(specs []corev1.PersistentVolumeSpec) ([]string, error) {
	names := []string{}
	for i := range specs {
		spec := &specs[i]
		if spec.Cinder == nil {
			// Not a openStack volume
			continue
		}
		name := spec.Cinder.VolumeID
		names = append(names, name)
	}
	return names, nil
}

func waitForStatus(c *gophercloud.ServiceClient, id string, pending []string, target []string, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		current, err := servers.Get(c, id).Extract()
		if err != nil {
			return false, err
		}

		if strSliceContains(target, current.Status) {
			return true, nil
		}

		if strSliceContains(pending, current.Status) {
			return false, nil
		}

		return false, fmt.Errorf("unexpected status %q, wanted target %q", current.Status, strings.Join(target, ", "))
	})
}

func strSliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
