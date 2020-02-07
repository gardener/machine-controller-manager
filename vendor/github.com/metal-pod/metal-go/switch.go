package metalgo

import (
	sw "github.com/metal-pod/metal-go/api/client/switch_operations"
	"github.com/metal-pod/metal-go/api/models"
)

// SwitchListResponse is the response of a SwitchList action
type SwitchListResponse struct {
	Switch []*models.V1SwitchResponse
}

// SwitchList return all switches
func (d *Driver) SwitchList() (*SwitchListResponse, error) {
	response := &SwitchListResponse{}
	listSwitchs := sw.NewListSwitchesParams()
	resp, err := d.sw.ListSwitches(listSwitchs, d.auth)
	if err != nil {
		return response, err
	}
	response.Switch = resp.Payload
	return response, nil
}
