package metalgo

import (
	"github.com/metal-pod/metal-go/api/client/partition"
	"github.com/metal-pod/metal-go/api/models"
)

// PartitionListResponse is the response of a PartitionList action
type PartitionListResponse struct {
	Partition []*models.V1PartitionResponse
}

// PartitionGetResponse is the response of a PartitionGet action
type PartitionGetResponse struct {
	Partition *models.V1PartitionResponse
}

// PartitionCapacityResponse is the response of a PartitionGet action
type PartitionCapacityResponse struct {
	Capacity []*models.V1PartitionCapacity
}

// PartitionCreateRequest is the response of a ImageList action
type PartitionCreateRequest struct {
	ID                         string
	Name                       string
	Description                string
	Bootconfig                 BootConfig
	Mgmtserviceaddress         string
	Privatenetworkprefixlength int32
}

// BootConfig in the partition
type BootConfig struct {
	Commandline string
	Imageurl    string
	Kernelurl   string
}

func (b *BootConfig) convert() *models.V1PartitionBootConfiguration {
	return &models.V1PartitionBootConfiguration{
		Commandline: b.Commandline,
		Imageurl:    b.Imageurl,
		Kernelurl:   b.Kernelurl,
	}
}

// PartitionCreateResponse is the response of a ImageList action
type PartitionCreateResponse struct {
	Partition *models.V1PartitionResponse
}

// PartitionList return all partitions
func (d *Driver) PartitionList() (*PartitionListResponse, error) {
	response := &PartitionListResponse{}
	listPartitions := partition.NewListPartitionsParams()
	resp, err := d.partition.ListPartitions(listPartitions, d.auth)
	if err != nil {
		return response, err
	}
	response.Partition = resp.Payload
	return response, nil
}

// PartitionGet return a partition
func (d *Driver) PartitionGet(partitionID string) (*PartitionGetResponse, error) {
	response := &PartitionGetResponse{}
	getPartition := partition.NewFindPartitionParams()
	getPartition.ID = partitionID
	resp, err := d.partition.FindPartition(getPartition, d.auth)
	if err != nil {
		return response, err
	}
	response.Partition = resp.Payload
	return response, nil
}

// PartitionCapacity return a partition
func (d *Driver) PartitionCapacity() (*PartitionCapacityResponse, error) {
	response := &PartitionCapacityResponse{}
	partitionParams := partition.NewPartitionCapacityParams()
	resp, err := d.partition.PartitionCapacity(partitionParams, d.auth)
	if err != nil {
		return response, err
	}
	response.Capacity = resp.Payload
	return response, nil
}

// PartitionCreate create a partition
func (d *Driver) PartitionCreate(pcr PartitionCreateRequest) (*PartitionCreateResponse, error) {
	response := &PartitionCreateResponse{}

	createPartition := &models.V1PartitionCreateRequest{
		ID:                         &pcr.ID,
		Name:                       pcr.Name,
		Description:                pcr.Description,
		Mgmtserviceaddress:         pcr.Mgmtserviceaddress,
		Privatenetworkprefixlength: pcr.Privatenetworkprefixlength,
		Bootconfig:                 pcr.Bootconfig.convert(),
	}
	request := partition.NewCreatePartitionParams()
	request.SetBody(createPartition)
	resp, err := d.partition.CreatePartition(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Partition = resp.Payload
	return response, nil
}

// PartitionUpdate create a partition
func (d *Driver) PartitionUpdate(pcr PartitionCreateRequest) (*PartitionCreateResponse, error) {
	response := &PartitionCreateResponse{}

	updatePartition := &models.V1PartitionUpdateRequest{
		ID:                 &pcr.ID,
		Name:               pcr.Name,
		Description:        pcr.Description,
		Mgmtserviceaddress: pcr.Mgmtserviceaddress,
		Bootconfig:         pcr.Bootconfig.convert(),
	}
	request := partition.NewUpdatePartitionParams()
	request.SetBody(updatePartition)
	resp, err := d.partition.UpdatePartition(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Partition = resp.Payload
	return response, nil
}

// PartitionDelete return a partition
func (d *Driver) PartitionDelete(partitionID string) (*PartitionGetResponse, error) {
	response := &PartitionGetResponse{}
	request := partition.NewDeletePartitionParams()
	request.ID = partitionID
	resp, err := d.partition.DeletePartition(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Partition = resp.Payload
	return response, nil
}
