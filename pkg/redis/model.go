package redis

import (
	"github.com/render-oss/cli/pkg/client"
)

const RedisType = "Redis"

type Model struct {
	Redis       *client.Redis       `json:"redis,omitempty"`
	Project     *client.Project     `json:"project,omitempty"`
	Environment *client.Environment `json:"environment,omitempty"`
}

func (m Model) ID() string {
	return m.Redis.Id
}

func (m Model) Name() string {
	return m.Redis.Name
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
	return RedisType
}
