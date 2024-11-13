package postgres

import (
	"github.com/renderinc/cli/pkg/client"
)

const PostgresType = "Postgres"

type Model struct {
	Postgres    *client.Postgres    `json:"postgres,omitempty"`
	Project     *client.Project     `json:"project,omitempty"`
	Environment *client.Environment `json:"environment,omitempty"`
}

func (m Model) ID() string {
	return m.Postgres.Id
}

func (m Model) Name() string {
	return m.Postgres.Name
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
	return PostgresType
}
