package keyvalue

import (
	"github.com/render-oss/cli/pkg/client"
)

const KeyValueType = "Key Value"

type Model struct {
	KeyValue    *client.KeyValue    `json:"keyValue,omitempty"`
	Project     *client.Project     `json:"project,omitempty"`
	Environment *client.Environment `json:"environment,omitempty"`
}

func (m Model) ID() string {
	return m.KeyValue.Id
}

func (m Model) Name() string {
	return m.KeyValue.Name
}

func (m Model) EnvironmentName() string {
	if m.Environment != nil {
		return m.Environment.Name
	}
	return ""
}

func (m Model) ProjectName() string {
	if m.Project != nil {
		return m.Project.Name
	}
	return ""
}

func (m Model) Type() string {
	return KeyValueType
}
