package metalgo

import (
	"github.com/metal-pod/metal-go/api/client/image"
	"github.com/metal-pod/metal-go/api/models"
)

// ImageListResponse is the response of a ImageList action
type ImageListResponse struct {
	Image []*models.V1ImageResponse
}

// ImageGetResponse is the response of a ImageList action
type ImageGetResponse struct {
	Image *models.V1ImageResponse
}

// ImageCreateRequest is the response of a ImageList action
type ImageCreateRequest struct {
	ID          string
	Name        string
	Description string
	URL         string
	Features    []string
}

// ImageCreateResponse is the response of a ImageList action
type ImageCreateResponse struct {
	Image *models.V1ImageResponse
}

// ImageList return all machine images
func (d *Driver) ImageList() (*ImageListResponse, error) {
	response := &ImageListResponse{}
	listImages := image.NewListImagesParams()
	resp, err := d.image.ListImages(listImages, d.auth)
	if err != nil {
		return response, err
	}
	response.Image = resp.Payload
	return response, nil
}

// ImageGet return a image
func (d *Driver) ImageGet(imageID string) (*ImageGetResponse, error) {
	response := &ImageGetResponse{}
	request := image.NewFindImageParams()
	request.ID = imageID
	resp, err := d.image.FindImage(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Image = resp.Payload
	return response, nil
}

// ImageCreate create a image
func (d *Driver) ImageCreate(icr ImageCreateRequest) (*ImageCreateResponse, error) {
	response := &ImageCreateResponse{}

	createImage := &models.V1ImageCreateRequest{
		Description: icr.Description,
		Features:    icr.Features,
		ID:          &icr.ID,
		Name:        icr.Name,
		URL:         &icr.URL,
	}
	request := image.NewCreateImageParams()
	request.SetBody(createImage)
	resp, err := d.image.CreateImage(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Image = resp.Payload
	return response, nil
}

// ImageUpdate create a image
func (d *Driver) ImageUpdate(icr ImageCreateRequest) (*ImageCreateResponse, error) {
	response := &ImageCreateResponse{}

	updateImage := &models.V1ImageUpdateRequest{
		Description: icr.Description,
		Features:    icr.Features,
		ID:          &icr.ID,
		Name:        icr.Name,
		URL:         icr.URL,
	}
	request := image.NewUpdateImageParams()
	request.SetBody(updateImage)
	resp, err := d.image.UpdateImage(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Image = resp.Payload
	return response, nil
}

// ImageDelete return a image
func (d *Driver) ImageDelete(imageID string) (*ImageGetResponse, error) {
	response := &ImageGetResponse{}
	request := image.NewDeleteImageParams()
	request.ID = imageID
	resp, err := d.image.DeleteImage(request, d.auth)
	if err != nil {
		return response, err
	}
	response.Image = resp.Payload
	return response, nil
}
