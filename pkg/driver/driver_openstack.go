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

	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
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

// deleteOnFail method is used to delete the VM, which was created with an error
func (d *OpenStackDriver) deleteOnFail(err error) error {
	// this method is called after the d.MachineID has been set
	if e := d.Delete(d.MachineID); e != nil {
		return fmt.Errorf("Error deleting machine %s (%s) after unsuccessful create attempt: %s", d.MachineID, e.Error(), err.Error())
	}
	return err
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
	imageID := d.OpenStackMachineClass.Spec.ImageID
	networkID := d.OpenStackMachineClass.Spec.NetworkID
	specNetworks := d.OpenStackMachineClass.Spec.Networks
	securityGroups := d.OpenStackMachineClass.Spec.SecurityGroups
	availabilityZone := d.OpenStackMachineClass.Spec.AvailabilityZone
	metadata := d.OpenStackMachineClass.Spec.Tags
	podNetworkCidr := d.OpenStackMachineClass.Spec.PodNetworkCidr
	rootDiskSize := d.OpenStackMachineClass.Spec.RootDiskSize

	var createOpts servers.CreateOptsBuilder
	var imageRef string

	// use imageID if provided, otherwise try to resolve the imageName to an imageID
	if imageID != "" {
		imageRef = imageID
	} else {
		imageRef, err = d.recentImageIDFromName(client, imageName)
		if err != nil {
			metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
			return "", "", fmt.Errorf("failed to get image id for image name %s: %s", imageName, err)
		}
	}

	nwClient, err := d.createNeutronClient()
	if err != nil {
		return "", "", d.deleteOnFail(err)
	}

	var serverNetworks []servers.Network
	var podNetworkIds = make(map[string]struct{})

	if len(networkID) > 0 {
		serverNetworks = append(serverNetworks, servers.Network{UUID: networkID})
		podNetworkIds[networkID] = struct{}{}
	} else {
		for _, network := range specNetworks {
			var resolvedNetworkID string
			if len(network.Id) > 0 {
				resolvedNetworkID = networkID
			} else {
				resolvedNetworkID, err = networks.IDFromName(nwClient, network.Name)
				if err != nil {
					metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()
					return "", "", fmt.Errorf("failed to get uuid for network name %s: %s", network.Name, err)
				}
			}
			serverNetworks = append(serverNetworks, servers.Network{UUID: resolvedNetworkID})
			if network.PodNetwork {
				podNetworkIds[resolvedNetworkID] = struct{}{}
			}
		}
	}

	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()

	createOpts = &servers.CreateOpts{
		ServiceClient:    client,
		Name:             d.MachineName,
		FlavorName:       flavorName,
		ImageRef:         imageRef,
		Networks:         serverNetworks,
		SecurityGroups:   securityGroups,
		Metadata:         metadata,
		UserData:         []byte(d.UserData),
		AvailabilityZone: availabilityZone,
	}

	createOpts = &keypairs.CreateOptsExt{
		CreateOptsBuilder: createOpts,
		KeyName:           keyName,
	}

	if rootDiskSize > 0 {
		blockDevices, err := resourceInstanceBlockDevicesV2(rootDiskSize, imageRef)
		if err != nil {
			return "", "", err
		}

		createOpts = &bootfromvolume.CreateOptsExt{
			CreateOptsBuilder: createOpts,
			BlockDevice:       blockDevices,
		}
	}

	klog.V(3).Infof("creating machine")

	var server *servers.Server
	// If a custom block_device (root disk size is provided) we need to boot from volume
	if rootDiskSize > 0 {
		server, err = bootfromvolume.Create(client, createOpts).Extract()
	} else {
		server, err = servers.Create(client, createOpts).Extract()
	}

	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		return "", "", fmt.Errorf("error creating the server: %s", err)
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()

	d.MachineID = d.encodeMachineID(d.OpenStackMachineClass.Spec.Region, server.ID)

	err = waitForStatus(client, server.ID, []string{"BUILD"}, []string{"ACTIVE"}, 600)
	if err != nil {
		return "", "", d.deleteOnFail(fmt.Errorf("error waiting for the %q server status: %s", server.ID, err))
	}

	listOpts := &ports.ListOpts{
		DeviceID: server.ID,
	}

	allPages, err := ports.List(nwClient, listOpts).AllPages()
	if err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()
		return "", "", d.deleteOnFail(fmt.Errorf("failed to get ports: %s", err))
	}
	metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()

	allPorts, err := ports.ExtractPorts(allPages)
	if err != nil {
		return "", "", d.deleteOnFail(fmt.Errorf("failed to extract ports: %s", err))
	}

	if len(allPorts) == 0 {
		return "", "", d.deleteOnFail(fmt.Errorf("got an empty port list for server ID %s", server.ID))
	}

	for _, port := range allPorts {
		for id := range podNetworkIds {
			if port.NetworkID == id {
				_, err := ports.Update(nwClient, port.ID, ports.UpdateOpts{
					AllowedAddressPairs: &[]ports.AddressPair{{IPAddress: podNetworkCidr}},
				}).Extract()
				if err != nil {
					metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()
					return "", "", d.deleteOnFail(fmt.Errorf("failed to update allowed address pair for port ID %s: %s", port.ID, err))
				}
				metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "neutron"}).Inc()
			}
		}
	}

	return d.MachineID, d.MachineName, nil
}

// Delete method is used to delete an OS machine
func (d *OpenStackDriver) Delete(machineID string) error {
	res, err := d.GetVMs(machineID)
	if err != nil {
		return err
	} else if len(res) == 0 {
		// No running instance exists with the given machine-ID
		klog.V(2).Infof("No VM matching the machine-ID found on the provider %q", machineID)
		return nil
	}

	instanceID := d.decodeMachineID(machineID)
	client, err := d.createNovaClient()
	if err != nil {
		return err
	}

	result := servers.Delete(client, instanceID)
	if result.Err == nil {
		// waiting for the machine to be deleted to release consumed quota resources, 5 minutes should be enough
		err = waitForStatus(client, machineID, nil, []string{"DELETED", "SOFT_DELETED"}, 300)
		if err != nil {
			return fmt.Errorf("error waiting for the %q server to be deleted: %s", machineID, err)
		}
		metrics.APIRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		klog.V(3).Infof("Deleted machine with ID: %s", machineID)
	} else {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		klog.Errorf("Failed to delete machine with ID: %s", machineID)
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
		klog.Errorf("Could not connect to NovaClient. Error Message - %s", err)
		return listOfVMs, err
	}

	// Retrieve a pager (i.e. a paginated collection)
	pager := servers.List(client, servers.ListOpts{})
	if pager.Err != nil {
		metrics.APIFailedRequestCount.With(prometheus.Labels{"provider": "openstack", "service": "nova"}).Inc()
		klog.Errorf("Could not list instances. Error Message - %s", err)
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
					klog.V(3).Infof("Found machine with name: %q", server.Name)
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

//GetUserData return the used data whit which the VM will be booted
func (d *OpenStackDriver) GetUserData() string {
	return d.UserData
}

//SetUserData set the used data whit which the VM will be booted
func (d *OpenStackDriver) SetUserData(userData string) {
	d.UserData = userData
}

func waitForStatus(c *gophercloud.ServiceClient, id string, pending []string, target []string, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		current, err := servers.Get(c, id).Extract()
		if err != nil {
			if _, ok := err.(gophercloud.ErrDefault404); ok && strSliceContains(target, "DELETED") {
				return true, nil
			}
			return false, err
		}

		if strSliceContains(target, current.Status) {
			return true, nil
		}

		// if there is no pending statuses defined or current status is in the pending list, then continue polling
		if pending == nil || len(pending) == 0 || strSliceContains(pending, current.Status) {
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

func resourceInstanceBlockDevicesV2(rootDiskSize int, imageID string) ([]bootfromvolume.BlockDevice, error) {
	blockDeviceOpts := make([]bootfromvolume.BlockDevice, 1)
	blockDeviceOpts[0] = bootfromvolume.BlockDevice{
		UUID:                imageID,
		VolumeSize:          rootDiskSize,
		BootIndex:           0,
		DeleteOnTermination: true,
		SourceType:          "image",
		DestinationType:     "volume",
	}
	klog.V(2).Infof("[DEBUG] Block Device Options: %+v", blockDeviceOpts)
	return blockDeviceOpts, nil
}
