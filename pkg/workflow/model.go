package workflow

import (
	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
)

const WorkflowType = "Workflow"

type Model struct {
	Workflow    *wfclient.Workflow  `json:"workflow"`
	Project     *client.Project     `json:"project,omitempty"`
	Environment *client.Environment `json:"environment,omitempty"`
}

func (m Model) ID() string {
	return m.Workflow.Id
}

func (m Model) Name() string {
	return m.Workflow.Name
}

func (m Model) ProjectName() string {
	if m.Project != nil {
		return m.Project.Name
	}
	return ""
}

func (m Model) EnvironmentName() string {
	if m.Environment != nil {
		return m.Environment.Name
	}
	return ""
}

func (m Model) Type() string {
	return WorkflowType
}
