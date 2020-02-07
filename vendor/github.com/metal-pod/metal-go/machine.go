package metalgo

import (
	"net/http"
	"time"

	"github.com/metal-pod/metal-go/api/client/machine"
	"github.com/metal-pod/metal-go/api/models"
)

// MachineCreateRequest contains data for a machine creation
type MachineCreateRequest struct {
	Description   string
	Hostname      string
	Name          string
	UserData      string
	Size          string
	Project       string
	Partition     string
	Image         string
	Tags          []string
	SSHPublicKeys []string
	UUID          string
	Networks      []MachineAllocationNetwork
	IPs           []string
}

// MachineFindRequest contains criteria for a machine listing
type MachineFindRequest struct {
	ID          *string
	Name        *string
	PartitionID *string
	SizeID      *string
	RackID      *string
	Liveliness  *string
	Tags        []string

	// allocation
	AllocationName      *string
	AllocationProject   *string
	AllocationImageID   *string
	AllocationHostname  *string
	AllocationSucceeded *bool

	// network
	NetworkIDs                 []string
	NetworkPrefixes            []string
	NetworkIPs                 []string
	NetworkDestinationPrefixes []string
	NetworkVrfs                []int64
	NetworkPrivate             *bool
	NetworkASNs                []int64
	NetworkNat                 *bool
	NetworkUnderlay            *bool

	// hardware
	HardwareMemory   *int64
	HardwareCPUCores *int64

	// nics
	NicsMacAddresses         []string
	NicsNames                []string
	NicsVrfs                 []string
	NicsNeighborMacAddresses []string
	NicsNeighborNames        []string
	NicsNeighborVrfs         []string

	// disks
	DiskNames []string
	DiskSizes []int64

	// state
	StateValue *string

	// ipmi
	IpmiAddress    *string
	IpmiMacAddress *string
	IpmiUser       *string
	IpmiInterface  *string

	// fru
	FruChassisPartNumber   *string
	FruChassisPartSerial   *string
	FruBoardMfg            *string
	FruBoardMfgSerial      *string
	FruBoardPartNumber     *string
	FruProductManufacturer *string
	FruProductPartNumber   *string
	FruProductSerial       *string
}

// MachineAllocationNetwork contains configuration for machine networks
type MachineAllocationNetwork struct {
	Autoacquire bool
	NetworkID   string
}

func (n *MachineCreateRequest) translateNetworks() []*models.V1MachineAllocationNetwork {
	var nets []*models.V1MachineAllocationNetwork
	for i := range n.Networks {
		net := models.V1MachineAllocationNetwork{
			Networkid:   &n.Networks[i].NetworkID,
			Autoacquire: &n.Networks[i].Autoacquire,
		}
		nets = append(nets, &net)
	}
	return nets
}

// MachineCreateResponse is returned when a machine was created
type MachineCreateResponse struct {
	Machine *models.V1MachineResponse
}

// MachineListResponse contains the machine list result
type MachineListResponse struct {
	Machines []*models.V1MachineResponse
}

// MachineGetResponse contains the machine get result
type MachineGetResponse struct {
	Machine *models.V1MachineResponse
}

// MachineIpmiResponse contains the machine get result
type MachineIpmiResponse struct {
	IPMI *models.V1MachineIPMI
}

// MachineDeleteResponse contains the machine delete result
type MachineDeleteResponse struct {
	Machine *models.V1MachineResponse
}

// MachinePowerResponse contains the machine power result
type MachinePowerResponse struct {
	Machine *models.V1MachineResponse
}

// ChassisIdentifyLEDPowerResponse contains the machine LED power result
type ChassisIdentifyLEDPowerResponse struct {
	Machine *models.V1MachineResponse
}

// MachineBiosResponse contains the machine bios result
type MachineBiosResponse struct {
	Machine *models.V1MachineResponse
}

// MachineStateResponse contains the machine bios result
type MachineStateResponse struct {
	Machine *models.V1MachineResponse
}

// MachineCreate creates a single metal machine
func (d *Driver) MachineCreate(mcr *MachineCreateRequest) (*MachineCreateResponse, error) {
	response := &MachineCreateResponse{}

	allocateRequest := &models.V1MachineAllocateRequest{
		Description: mcr.Description,
		Partitionid: &mcr.Partition,
		Hostname:    mcr.Hostname,
		Imageid:     &mcr.Image,
		Name:        mcr.Name,
		UUID:        mcr.UUID,
		Projectid:   &mcr.Project,
		Sizeid:      &mcr.Size,
		SSHPubKeys:  mcr.SSHPublicKeys,
		UserData:    mcr.UserData,
		Tags:        mcr.Tags,
		Networks:    mcr.translateNetworks(),
		Ips:         mcr.IPs,
	}
	allocMachine := machine.NewAllocateMachineParams()
	allocMachine.SetBody(allocateRequest)

	retryOnlyOnStatusNotFound := func(sc int) bool {
		return sc == http.StatusNotFound
	}
	allocMachine.WithContext(newRetryContext(3, 5*time.Second, retryOnlyOnStatusNotFound))

	resp, err := d.machine.AllocateMachine(allocMachine, d.auth)
	if err != nil {
		return response, err
	}

	response.Machine = resp.Payload

	return response, nil
}

// MachineDelete deletes a single metal machine
func (d *Driver) MachineDelete(machineID string) (*MachineDeleteResponse, error) {
	freeMachine := machine.NewFreeMachineParams()
	freeMachine.ID = machineID

	response := &MachineDeleteResponse{}
	resp, err := d.machine.FreeMachine(freeMachine, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}

// MachineGet returns the machine with the given ID
func (d *Driver) MachineGet(id string) (*MachineGetResponse, error) {
	findMachine := machine.NewFindMachineParams()
	findMachine.ID = id

	response := &MachineGetResponse{}
	resp, err := d.machine.FindMachine(findMachine, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload

	return response, nil
}

// MachineList lists all machines
func (d *Driver) MachineList() (*MachineListResponse, error) {
	listMachines := machine.NewListMachinesParams()
	response := &MachineListResponse{}
	resp, err := d.machine.ListMachines(listMachines, d.auth)
	if err != nil {
		return response, err
	}
	response.Machines = resp.Payload

	return response, nil
}

// MachineFind lists all machines that match the given properties
func (d *Driver) MachineFind(mfr *MachineFindRequest) (*MachineListResponse, error) {
	if mfr == nil {
		return d.MachineList()
	}

	response := &MachineListResponse{}
	var err error
	var resp *machine.FindMachinesOK

	findMachines := machine.NewFindMachinesParams()
	req := &models.V1MachineFindRequest{
		ID:                         mfr.ID,
		Name:                       mfr.Name,
		PartitionID:                mfr.PartitionID,
		Sizeid:                     mfr.SizeID,
		Rackid:                     mfr.RackID,
		Liveliness:                 mfr.Liveliness,
		Tags:                       mfr.Tags,
		AllocationName:             mfr.AllocationName,
		AllocationProject:          mfr.AllocationProject,
		AllocationImageID:          mfr.AllocationImageID,
		AllocationHostname:         mfr.AllocationHostname,
		AllocationSucceeded:        mfr.AllocationSucceeded,
		NetworkIds:                 mfr.NetworkIDs,
		NetworkPrefixes:            mfr.NetworkPrefixes,
		NetworkIps:                 mfr.NetworkIPs,
		NetworkDestinationPrefixes: mfr.NetworkDestinationPrefixes,
		NetworkVrfs:                mfr.NetworkVrfs,
		NetworkPrivate:             mfr.NetworkPrivate,
		NetworkAsns:                mfr.NetworkASNs,
		NetworkNat:                 mfr.NetworkNat,
		NetworkUnderlay:            mfr.NetworkUnderlay,
		HardwareMemory:             mfr.HardwareMemory,
		HardwareCPUCores:           mfr.HardwareCPUCores,
		NicsMacAddresses:           mfr.NicsMacAddresses,
		NicsNames:                  mfr.NicsNames,
		NicsVrfs:                   mfr.NicsVrfs,
		NicsNeighborMacAddresses:   mfr.NicsNeighborMacAddresses,
		NicsNeighborNames:          mfr.NicsNeighborNames,
		NicsNeighborVrfs:           mfr.NicsNeighborVrfs,
		DiskNames:                  mfr.DiskNames,
		DiskSizes:                  mfr.DiskSizes,
		StateValue:                 mfr.StateValue,
		IPMIAddress:                mfr.IpmiAddress,
		IPMIMacAddress:             mfr.IpmiMacAddress,
		IPMIUser:                   mfr.IpmiUser,
		IPMIInterface:              mfr.IpmiInterface,
		FruChassisPartNumber:       mfr.FruChassisPartNumber,
		FruChassisPartSerial:       mfr.FruChassisPartSerial,
		FruBoardMfg:                mfr.FruBoardMfg,
		FruBoardMfgSerial:          mfr.FruBoardMfgSerial,
		FruBoardPartNumber:         mfr.FruChassisPartNumber,
		FruProductManufacturer:     mfr.FruProductManufacturer,
		FruProductPartNumber:       mfr.FruProductPartNumber,
		FruProductSerial:           mfr.FruProductSerial,
	}
	findMachines.SetBody(req)

	resp, err = d.machine.FindMachines(findMachines, d.auth)
	if err != nil {
		return response, err
	}
	response.Machines = resp.Payload

	return response, nil
}

// MachineIpmi returns the IPMI data of the given machine
func (d *Driver) MachineIpmi(machineID string) (*MachineIpmiResponse, error) {
	ipmiMachine := machine.NewIPMIDataParams()
	ipmiMachine.ID = machineID

	response := &MachineIpmiResponse{}
	resp, err := d.machine.IPMIData(ipmiMachine, d.auth)
	if err != nil {
		return response, err
	}
	response.IPMI = resp.Payload

	return response, nil
}

// MachinePowerOn powers on the given machine
func (d *Driver) MachinePowerOn(machineID string) (*MachinePowerResponse, error) {
	machineOn := machine.NewMachineOnParams()
	machineOn.ID = machineID
	machineOn.Body = []string{}

	response := &MachinePowerResponse{}
	resp, err := d.machine.MachineOn(machineOn, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}

// MachinePowerOff powers off the given machine
func (d *Driver) MachinePowerOff(machineID string) (*MachinePowerResponse, error) {
	machineOff := machine.NewMachineOffParams()
	machineOff.ID = machineID
	machineOff.Body = []string{}

	response := &MachinePowerResponse{}
	resp, err := d.machine.MachineOff(machineOff, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}

// MachinePowerReset power-resets the given machine
func (d *Driver) MachinePowerReset(machineID string) (*MachinePowerResponse, error) {
	machineReset := machine.NewMachineResetParams()
	machineReset.ID = machineID
	machineReset.Body = []string{}

	response := &MachinePowerResponse{}
	resp, err := d.machine.MachineReset(machineReset, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}

// MachineBootBios boots given machine into BIOS
func (d *Driver) MachineBootBios(machineID string) (*MachineBiosResponse, error) {
	machineBios := machine.NewMachineBiosParams()
	machineBios.ID = machineID
	machineBios.Body = []string{}

	response := &MachineBiosResponse{}
	resp, err := d.machine.MachineBios(machineBios, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}

// MachineLock locks a machine to prevent it from being destroyed
func (d *Driver) MachineLock(machineID, description string) (*MachineStateResponse, error) {
	return d.machineState(machineID, "LOCKED", description)
}

// MachineUnLock unlocks a machine
func (d *Driver) MachineUnLock(machineID string) (*MachineStateResponse, error) {
	return d.machineState(machineID, "", "")
}

// MachineReserve reserves a machine for single allocation
func (d *Driver) MachineReserve(machineID, description string) (*MachineStateResponse, error) {
	return d.machineState(machineID, "RESERVED", description)
}

// MachineUnReserve unreserves a machine
func (d *Driver) MachineUnReserve(machineID string) (*MachineStateResponse, error) {
	return d.machineState(machineID, "", "")
}

func (d *Driver) machineState(machineID, state, description string) (*MachineStateResponse, error) {
	machineState := machine.NewSetMachineStateParams()
	machineState.ID = machineID
	machineState.Body = &models.V1MachineState{
		Value:       &state,
		Description: &description,
	}

	response := &MachineStateResponse{}
	resp, err := d.machine.SetMachineState(machineState, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}

// ChassisIdentifyLEDPowerOn powers on the given machine
func (d *Driver) ChassisIdentifyLEDPowerOn(machineID, description string) (*ChassisIdentifyLEDPowerResponse, error) {
	machineLedOn := machine.NewChassisIdentifyLEDOnParams()
	machineLedOn.ID = machineID
	machineLedOn.Description = description
	machineLedOn.Body = []string{}

	response := &ChassisIdentifyLEDPowerResponse{}
	resp, err := d.machine.ChassisIdentifyLEDOn(machineLedOn, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}

// ChassisIdentifyLEDPowerOff powers off the given machine
func (d *Driver) ChassisIdentifyLEDPowerOff(machineID, description string) (*ChassisIdentifyLEDPowerResponse, error) {
	machineLedOff := machine.NewChassisIdentifyLEDOffParams()
	machineLedOff.ID = machineID
	machineLedOff.Description = description
	machineLedOff.Body = []string{}

	response := &ChassisIdentifyLEDPowerResponse{}
	resp, err := d.machine.ChassisIdentifyLEDOff(machineLedOff, d.auth)
	if err != nil {
		return response, err
	}
	response.Machine = resp.Payload
	return response, nil
}
