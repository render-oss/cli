package service

import "github.com/render-oss/cli/pkg/client"

type ServiceOut struct {
	client.Service
	ProjectID       *string `json:"projectId"`
	ProjectName     string  `json:"-"`
	EnvironmentName string  `json:"-"`
}

type DeleteOut struct {
	Data ServiceOut    `json:"data"`
	Meta DeleteOutMeta `json:"meta"`
}

type DeleteOutMeta struct {
	Deleted bool   `json:"deleted"`
	Message string `json:"message,omitempty"`
}

func newServiceOutFromModel(model *Model) ServiceOut {
	if model == nil {
		return ServiceOut{}
	}

	out := ServiceOut{}
	if model.Service != nil {
		out.Service = *model.Service
	}
	if model.Project != nil {
		out.ProjectID = &model.Project.Id
		out.ProjectName = model.Project.Name
	}
	if model.Environment != nil {
		out.EnvironmentName = model.Environment.Name
	}
	return out
}

// NewDeleteOutFromModel constructs a [DeleteOut] from a service [Model].
// Callers should mutate [DeleteOut.Meta] as needed.
func NewDeleteOutFromModel(model *Model) DeleteOut {
	return DeleteOut{
		Data: newServiceOutFromModel(model),
	}
}
