package metalgo

import (
	"net/http"
	"time"

	"github.com/metal-pod/metal-go/api/client/firewall"
	"github.com/metal-pod/metal-go/api/models"
)

// FirewallCreateRequest contains data for a machine creation
type FirewallCreateRequest struct {
	MachineCreateRequest
}

// FirewallCreateResponse is returned when a machine was created
type FirewallCreateResponse struct {
	Firewall *models.V1FirewallResponse
}

// FirewallFindRequest contains criteria for a machine listing
type FirewallFindRequest struct {
	MachineFindRequest
}

// FirewallListResponse contains the machine list result
type FirewallListResponse struct {
	Firewalls []*models.V1FirewallResponse
}

// FirewallGetResponse contains the machine get result
type FirewallGetResponse struct {
	Firewall *models.V1FirewallResponse
}

// FirewallCreate will create a single metal machine
func (d *Driver) FirewallCreate(fcr *FirewallCreateRequest) (*FirewallCreateResponse, error) {
	response := &FirewallCreateResponse{}

	allocateRequest := &models.V1FirewallCreateRequest{
		Description: fcr.Description,
		Partitionid: &fcr.Partition,
		Hostname:    fcr.Hostname,
		Imageid:     &fcr.Image,
		Name:        fcr.Name,
		UUID:        fcr.UUID,
		Projectid:   &fcr.Project,
		Sizeid:      &fcr.Size,
		SSHPubKeys:  fcr.SSHPublicKeys,
		UserData:    fcr.UserData,
		Tags:        fcr.Tags,
		Networks:    fcr.translateNetworks(),
		Ips:         fcr.IPs,
	}

	allocFirewall := firewall.NewAllocateFirewallParams()
	allocFirewall.SetBody(allocateRequest)

	retryOnlyOnStatusNotFound := func(sc int) bool {
		return sc == http.StatusNotFound
	}
	allocFirewall.WithContext(newRetryContext(3, 5*time.Second, retryOnlyOnStatusNotFound))

	resp, err := d.firewall.AllocateFirewall(allocFirewall, d.auth)
	if err != nil {
		return response, err
	}
	response.Firewall = resp.Payload

	return response, nil
}

// FirewallList will list all machines
func (d *Driver) FirewallList() (*FirewallListResponse, error) {
	response := &FirewallListResponse{}

	listFirewall := firewall.NewListFirewallsParams()
	resp, err := d.firewall.ListFirewalls(listFirewall, d.auth)
	if err != nil {
		return response, err
	}
	response.Firewalls = resp.Payload
	return response, nil
}

// FirewallFind will search for firewalls for given criteria
func (d *Driver) FirewallFind(ffr *FirewallFindRequest) (*FirewallListResponse, error) {
	if ffr == nil {
		return d.FirewallList()
	}

	response := &FirewallListResponse{}
	var err error
	var resp *firewall.FindFirewallsOK

	req := &models.V1FirewallFindRequest{
		ID:                         ffr.ID,
		Name:                       ffr.Name,
		PartitionID:                ffr.PartitionID,
		Sizeid:                     ffr.SizeID,
		Rackid:                     ffr.RackID,
		Liveliness:                 ffr.Liveliness,
		Tags:                       ffr.Tags,
		AllocationName:             ffr.AllocationName,
		AllocationProject:          ffr.AllocationProject,
		AllocationImageID:          ffr.AllocationImageID,
		AllocationHostname:         ffr.AllocationHostname,
		AllocationSucceeded:        ffr.AllocationSucceeded,
		NetworkIds:                 ffr.NetworkIDs,
		NetworkPrefixes:            ffr.NetworkPrefixes,
		NetworkIps:                 ffr.NetworkIPs,
		NetworkDestinationPrefixes: ffr.NetworkDestinationPrefixes,
		NetworkVrfs:                ffr.NetworkVrfs,
		NetworkPrivate:             ffr.NetworkPrivate,
		NetworkAsns:                ffr.NetworkASNs,
		NetworkNat:                 ffr.NetworkNat,
		NetworkUnderlay:            ffr.NetworkUnderlay,
		HardwareMemory:             ffr.HardwareMemory,
		HardwareCPUCores:           ffr.HardwareCPUCores,
		NicsMacAddresses:           ffr.NicsMacAddresses,
		NicsNames:                  ffr.NicsNames,
		NicsVrfs:                   ffr.NicsVrfs,
		NicsNeighborMacAddresses:   ffr.NicsNeighborMacAddresses,
		NicsNeighborNames:          ffr.NicsNeighborNames,
		NicsNeighborVrfs:           ffr.NicsNeighborVrfs,
		DiskNames:                  ffr.DiskNames,
		DiskSizes:                  ffr.DiskSizes,
		StateValue:                 ffr.StateValue,
		IPMIAddress:                ffr.IpmiAddress,
		IPMIMacAddress:             ffr.IpmiMacAddress,
		IPMIUser:                   ffr.IpmiUser,
		IPMIInterface:              ffr.IpmiInterface,
		FruChassisPartNumber:       ffr.FruChassisPartNumber,
		FruChassisPartSerial:       ffr.FruChassisPartSerial,
		FruBoardMfg:                ffr.FruBoardMfg,
		FruBoardMfgSerial:          ffr.FruBoardMfgSerial,
		FruBoardPartNumber:         ffr.FruChassisPartNumber,
		FruProductManufacturer:     ffr.FruProductManufacturer,
		FruProductPartNumber:       ffr.FruProductPartNumber,
		FruProductSerial:           ffr.FruProductSerial,
	}
	findFirewalls := firewall.NewFindFirewallsParams()
	findFirewalls.SetBody(req)

	resp, err = d.firewall.FindFirewalls(findFirewalls, d.auth)
	if err != nil {
		return response, err
	}
	response.Firewalls = resp.Payload
	return response, nil
}

// FirewallGet will only return one machine
func (d *Driver) FirewallGet(machineID string) (*FirewallGetResponse, error) {
	findFirewall := firewall.NewFindFirewallParams()
	findFirewall.ID = machineID

	response := &FirewallGetResponse{}
	resp, err := d.firewall.FindFirewall(findFirewall, d.auth)
	if err != nil {
		return response, err
	}
	response.Firewall = resp.Payload

	return response, nil
}
