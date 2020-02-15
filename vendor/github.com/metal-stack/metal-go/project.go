package metalgo

import (
	"github.com/metal-stack/metal-go/api/client/project"
	"github.com/metal-stack/metal-go/api/models"
)

// ProjectListResponse is the response of a ProjectList action
type ProjectListResponse struct {
	Project []*models.V1ProjectResponse
}

// ProjectGetResponse is the response of a ProjectGet action
type ProjectGetResponse struct {
	Project *models.V1ProjectResponse
}

// ProjectFindRequest is the find request struct
type ProjectFindRequest struct {
	ID     string
	Name   string
	Tenant string
}

// ProjectList return all projects
func (d *Driver) ProjectList() (*ProjectListResponse, error) {
	response := &ProjectListResponse{}
	listProjects := project.NewListProjectsParams()
	resp, err := d.project.ListProjects(listProjects, d.auth)
	if err != nil {
		return response, err
	}
	response.Project = resp.Payload
	return response, nil
}

// ProjectFind return projects by given findRequest
func (d *Driver) ProjectFind(pfr ProjectFindRequest) (*ProjectListResponse, error) {
	response := &ProjectListResponse{}
	findProjects := project.NewFindProjectsParams()
	findProjects.Body = &models.V1ProjectFindRequest{
		ID:       &models.WrappersStringValue{Value: pfr.ID},
		Name:     &models.WrappersStringValue{Value: pfr.Name},
		TenantID: &models.WrappersStringValue{Value: pfr.Tenant},
	}
	resp, err := d.project.FindProjects(findProjects, d.auth)
	if err != nil {
		return response, err
	}
	response.Project = resp.Payload
	return response, nil
}

// ProjectGet return a Project
func (d *Driver) ProjectGet(projectID string) (*ProjectGetResponse, error) {
	response := &ProjectGetResponse{}
	getProject := project.NewFindProjectParams()
	getProject.ID = projectID
	resp, err := d.project.FindProject(getProject, d.auth)
	if err != nil {
		return response, err
	}
	response.Project = resp.Payload
	return response, nil
}
