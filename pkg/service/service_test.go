package service

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
)

func TestGetService(t *testing.T) {
	for _, tc := range []struct {
		name          string
		inEnvironment bool
	}{
		{name: "retrieves project and environment by ID", inEnvironment: true},
		{name: "skips project and environment lookups for ungrouped services"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, server := newTestService(t)
			seeded := server.CreateProject(
				renderapi.ProjectAttrs{Name: "My Project", OwnerId: "tea-workspace"},
				renderapi.EnvAttrs{Name: "production"},
				renderapi.EnvAttrs{Name: "staging"},
			)
			attrs := renderapi.CommonServiceAttrs{Name: "my-service", OwnerID: "tea-workspace"}
			if tc.inEnvironment {
				attrs.EnvironmentID = seeded.Env("production").Id
			}
			service := server.Services.Add(renderapi.NewWebService(renderapi.WebServiceAttrs{Service: attrs}))

			model, err := svc.GetService(t.Context(), service.Id)
			require.NoError(t, err)
			require.NotNil(t, model)
			assert.Equal(t, service.Id, model.ID())
			assert.Equal(t, "my-service", model.Name())

			wantRequests := []string{"GET /services/" + service.Id}
			if tc.inEnvironment {
				require.NotNil(t, model.Environment)
				require.NotNil(t, model.Project)
				assert.Equal(t, seeded.Env("production").Id, model.Environment.Id)
				assert.Equal(t, seeded.Project.Id, model.Project.Id)
				assert.Equal(t, "production", model.EnvironmentName())
				assert.Equal(t, "My Project", model.ProjectName())
				wantRequests = append(wantRequests,
					"GET /environments/"+seeded.Env("production").Id,
					"GET /projects/"+seeded.Project.Id,
				)
			} else {
				assert.Nil(t, model.Environment)
				assert.Nil(t, model.Project)
			}
			// Single-service lookups must not list projects or environments.
			var requests []string
			for _, request := range server.Requests {
				requests = append(requests, request.Method+" "+request.URI)
			}
			assert.ElementsMatch(t, wantRequests, requests)
		})
	}
}

func TestGetServiceErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setup     func(*renderapi.Server)
		wantError string
		wantCause error
	}{
		{
			name: "service retrieval fails",
			setup: func(server *renderapi.Server) {
				server.Services.RespondWith(http.StatusForbidden)
			},
			wantCause: client.ErrForbidden,
		},
		{
			name: "service is not found",
			setup: func(server *renderapi.Server) {
				server.Services.Instances = nil
			},
			wantCause: client.ErrNotFound,
		},
		{
			name: "environment retrieval is unauthorized",
			setup: func(server *renderapi.Server) {
				server.Environments.RespondWithErrorForGet(http.StatusUnauthorized)
			},
			wantCause: client.ErrUnauthorized,
		},
		{
			name: "project retrieval is unauthorized",
			setup: func(server *renderapi.Server) {
				server.Projects.RespondWithErrorForGet(http.StatusUnauthorized)
			},
			wantCause: client.ErrUnauthorized,
		},
		{
			name: "environment retrieval has a server error",
			setup: func(server *renderapi.Server) {
				server.Environments.RespondWithErrorForGet(http.StatusInternalServerError)
			},
			wantError: "received response code 500",
		},
		{
			name: "project retrieval has a server error",
			setup: func(server *renderapi.Server) {
				server.Projects.RespondWithErrorForGet(http.StatusInternalServerError)
			},
			wantError: "received response code 500",
		},
		{
			name: "service belongs to another workspace",
			setup: func(server *renderapi.Server) {
				server.Services.Instances[0].OwnerId = "tea-other"
			},
			wantError: "resource in workspace does not match",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, server := newTestService(t)
			seeded := server.CreateProject(
				renderapi.ProjectAttrs{Name: "My Project", OwnerId: "tea-workspace"},
				renderapi.EnvAttrs{Name: "production"},
			)
			service := server.Services.Add(renderapi.NewWebService(renderapi.WebServiceAttrs{
				Service: renderapi.CommonServiceAttrs{
					OwnerID:       "tea-workspace",
					EnvironmentID: seeded.Env("production").Id,
				},
			}))
			tc.setup(server)

			model, err := svc.GetService(t.Context(), service.Id)
			if tc.wantCause != nil {
				assert.ErrorIs(t, err, tc.wantCause)
			} else {
				assert.ErrorContains(t, err, tc.wantError)
			}
			assert.Nil(t, model)
		})
	}
}

func TestGetServiceWithoutMetadata(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resource string
		status   int
	}{
		{name: "environment forbidden", resource: "environment", status: http.StatusForbidden},
		{name: "project forbidden", resource: "project", status: http.StatusForbidden},
		{name: "environment not found", resource: "environment", status: http.StatusNotFound},
		{name: "project not found", resource: "project", status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, server := newTestService(t)
			seeded := server.CreateProject(
				renderapi.ProjectAttrs{Name: "My Project", OwnerId: "tea-workspace"},
				renderapi.EnvAttrs{Name: "production"},
			)
			service := server.Services.Add(renderapi.NewWebService(renderapi.WebServiceAttrs{
				Service: renderapi.CommonServiceAttrs{
					Name: "my-service", OwnerID: "tea-workspace", EnvironmentID: seeded.Env("production").Id,
				},
			}))
			if tc.resource == "environment" {
				server.Environments.RespondWithErrorForGet(tc.status)
			} else {
				server.Projects.RespondWithErrorForGet(tc.status)
			}

			model, err := svc.GetService(t.Context(), service.Id)
			require.NoError(t, err)
			require.NotNil(t, model)
			assert.Equal(t, service.Id, model.ID())
			assert.Equal(t, "my-service", model.Name())
			assert.Equal(t, service.EnvironmentId, model.Service.EnvironmentId)
			assert.Nil(t, model.Project)
			if tc.resource == "environment" {
				assert.Nil(t, model.Environment)
				assert.False(t, server.HasRequest(http.MethodGet, "/projects"))
			} else {
				require.NotNil(t, model.Environment)
				assert.Equal(t, "production", model.EnvironmentName())
			}
		})
	}
}

func newTestService(t *testing.T) (*Service, *renderapi.Server) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))
	t.Setenv("RENDER_WORKSPACE", "tea-workspace")
	cfg := config.Config{Workspace: "tea-workspace"}
	require.NoError(t, cfg.Persist())
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-workspace"}))
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return NewService(NewRepo(c), environment.NewRepo(c), project.NewRepo(c)), server
}
