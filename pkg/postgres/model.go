package postgres

import (
	"github.com/renderinc/render-cli/pkg/client"
)

const ResourceIDPrefix = "dpg-"
const PostgresType = "Postgres"

type Model struct {
	postgres    *client.Postgres
	project     *client.Project
	environment *client.Environment
}

func (m Model) ID() string {
	return m.postgres.Id
}

func (m Model) Name() string {
	return m.postgres.Name
}

func (m Model) Environment() *client.Environment {
	return m.environment
}

func (m Model) EnvironmentName() string {
	if m.environment != nil {
		return m.environment.Name
	}
	return ""
}

func (m Model) Project() *client.Project {
	return m.project
}

func (m Model) ProjectName() string {
	if m.project != nil {
		return m.project.Name
	}
	return ""
}

func (m Model) Type() string {
	return PostgresType
}
