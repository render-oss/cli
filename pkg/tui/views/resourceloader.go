package views

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/resource"
)

type ListResourceInput struct {
	Project         *client.Project
	EnvironmentIDs  []string `cli:"environment-ids"`
	IncludePreviews bool     `cli:"include-previews"`
}

func (l ListResourceInput) ToParams() resource.ResourceParams {
	return resource.ResourceParams{
		EnvironmentIDs:  l.EnvironmentIDs,
		IncludePreviews: l.IncludePreviews,
	}
}

type ResourceLoader struct {
	resourceService *resource.Service
}

func NewResourceLoader(resourceService *resource.Service) *ResourceLoader {
	return &ResourceLoader{resourceService: resourceService}
}

func (l *ResourceLoader) LoadResourceData(ctx context.Context, in ListResourceInput) ([]resource.Resource, error) {
	return l.resourceService.ListResources(ctx, in.ToParams())
}
