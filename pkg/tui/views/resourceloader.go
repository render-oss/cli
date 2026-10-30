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

// LoadResourceData lists resources for the input's scope. A selected Project
// with no environments has an empty scope, so it lists nothing rather than
// falling back to the unfiltered workspace listing. Without a selected
// Project, empty EnvironmentIDs still mean all resources.
func (l *ResourceLoader) LoadResourceData(ctx context.Context, in ListResourceInput) ([]resource.Resource, error) {
	if in.Project != nil && len(in.EnvironmentIDs) == 0 {
		return []resource.Resource{}, nil
	}
	return l.resourceService.ListResources(ctx, in.ToParams())
}
