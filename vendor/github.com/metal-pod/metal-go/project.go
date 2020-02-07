package metalgo

import (
	"github.com/metal-pod/metal-go/api/client/project"
	"github.com/metal-pod/metal-go/api/models"
)

// ProjectListResponse is the response of a ProjectList action
type ProjectListResponse struct {
	Project []*models.V1ProjectResponse
}

// ProjectGetResponse is the response of a ProjectGet action
type ProjectGetResponse struct {
	Project *models.V1ProjectResponse
}

// ProjectCreateRequest is the response of a ImageList action
type ProjectCreateRequest struct {
	Name        string
	Description string
	Tenant      string
}

// ProjectUpdateRequest is the response of a ImageList action
type ProjectUpdateRequest struct {
	ProjectCreateRequest
	ID string
}

// ProjectCreateResponse is the response of a ImageList action
type ProjectCreateResponse struct {
	Project *models.V1ProjectResponse
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

// ProjectCreate create a Project
func (d *Driver) ProjectCreate(pcr ProjectCreateRequest) (*ProjectCreateResponse, error) {
	response := &ProjectCreateResponse{}

	createProject := &models.V1ProjectCreateRequest{
		Name:        pcr.Name,
		Description: pcr.Description,
		Tenant:      &pcr.Tenant,
	}
	request := project.NewCreateProjectParams()
	request.SetBody(createProject)
	resp, err := d.project.CreateProject(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Project = resp.Payload
	return response, nil
}

// ProjectUpdate create a Project
func (d *Driver) ProjectUpdate(pcr ProjectUpdateRequest) (*ProjectCreateResponse, error) {
	response := &ProjectCreateResponse{}

	updateProject := &models.V1ProjectUpdateRequest{
		ID:          &pcr.ID,
		Name:        pcr.Name,
		Description: pcr.Description,
	}
	request := project.NewUpdateProjectParams()
	request.SetBody(updateProject)
	resp, err := d.project.UpdateProject(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Project = resp.Payload
	return response, nil
}

// ProjectDelete return a Project
func (d *Driver) ProjectDelete(projectID string) (*ProjectGetResponse, error) {
	response := &ProjectGetResponse{}
	request := project.NewDeleteProjectParams()
	request.ID = projectID
	resp, err := d.project.DeleteProject(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Project = resp.Payload
	return response, nil
}
