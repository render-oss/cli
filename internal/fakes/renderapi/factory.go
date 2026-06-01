package renderapi

import (
	"fmt"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
)

// EnvAttrs defines the fields a caller can specify for a nested fake environment
// created via Server.CreateProject. Zero-value fields are filled by NewEnvironment
// defaults; ProjectId is wired automatically.
type EnvAttrs struct {
	// ID is optional; when empty it is generated deterministically from the
	// owning project and environment names.
	ID   string
	Name string
}

// SeededProject bundles a persisted project with its environments, keyed by name,
// for assertions and resource placement.
type SeededProject struct {
	Project      *client.Project
	environments map[string]*client.Environment
}

// Env returns the seeded environment with the given name. It panics if no such
// environment was seeded, surfacing test wiring mistakes immediately.
func (p *SeededProject) Env(name string) *client.Environment {
	env, ok := p.environments[name]
	if !ok {
		panic(fmt.Sprintf("renderapi: SeededProject has no environment named %q", name))
	}
	return env
}

// CreateProject persists a project along with its child environments, wiring
// project.EnvironmentIds and environment.ProjectId in both directions. IDs are
// generated (deterministically from the names) when not supplied, so tests never
// need to declare them — read them back via SeededProject.Project / Env(name).
//
// Callers control any field through ProjectAttrs / EnvAttrs; NewProject and
// NewEnvironment fill the remaining zero values. The returned pointers are the
// same instances held by the server, so mutating them is observable through the API.
func (s *Server) CreateProject(attrs ProjectAttrs, envs ...EnvAttrs) *SeededProject {
	if attrs.Id == "" && attrs.Name != "" {
		attrs.Id = testids.ProjectID(attrs.Name)
	}
	project := NewProject(attrs)

	seeded := &SeededProject{
		Project:      project,
		environments: make(map[string]*client.Environment, len(envs)),
	}

	for _, e := range envs {
		if _, dup := seeded.environments[e.Name]; dup {
			panic(fmt.Sprintf("renderapi: CreateProject given duplicate environment name %q", e.Name))
		}
		envInput := client.Environment{Id: e.ID, Name: e.Name, ProjectId: project.Id}
		if envInput.Id == "" && e.Name != "" {
			// Namespace by project so the same env name in two projects doesn't collide.
			envInput.Id = testids.EnvironmentID(attrs.Name + "-" + e.Name)
		}
		env := NewEnvironment(envInput)
		project.EnvironmentIds = append(project.EnvironmentIds, env.Id)
		s.Environments.Add(env)
		seeded.environments[e.Name] = env
	}

	s.Projects.Add(project)
	return seeded
}
