package cmd

import (
	"bytes"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServiceUpdateTestCmd() *cobra.Command {
	return newServiceUpdateCmd(dependencies.New(nil))
}

func TestServiceUpdateCmdArgsValidation(t *testing.T) {
	cmd := newServiceUpdateTestCmd()

	t.Run("rejects zero positional args", func(t *testing.T) {
		require.Error(t, cmd.Args(cmd, []string{}))
	})

	t.Run("accepts one positional arg", func(t *testing.T) {
		require.NoError(t, cmd.Args(cmd, []string{"my-service"}))
	})

	t.Run("rejects more than one positional arg", func(t *testing.T) {
		require.Error(t, cmd.Args(cmd, []string{"arg1", "arg2"}))
	})
}

func TestServiceUpdateAliasResolvesToUpdateCommand(t *testing.T) {
	root := newRootCmd()
	services := cobraServicesCommand()
	update := newServiceUpdateTestCmd()
	services.AddCommand(update)
	root.AddCommand(services)

	plural, _, err := root.Find([]string{"services", "update"})
	require.NoError(t, err)
	require.Same(t, update, plural)

	alias, _, err := root.Find([]string{"service", "update"})
	require.NoError(t, err)
	require.Same(t, update, alias)
}

func TestServiceUpdateNoArgsValidationPreventsExecution(t *testing.T) {
	update := newServiceUpdateTestCmd()
	called := false
	cmd := &cobra.Command{
		Use:  "update",
		Args: update.Args,
		RunE: func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		},
	}
	cmd.SetArgs([]string{"arg1", "arg2"})

	err := cmd.Execute()
	require.Error(t, err)
	require.False(t, called)
}

func TestServiceUpdateFlagsRegistration(t *testing.T) {
	cmd := newServiceUpdateTestCmd()

	tests := []struct {
		flagName string
	}{
		{"name"},
		{"repo"},
		{"branch"},
		{"image"},
		{"plan"},
		{"runtime"},
		{"root-directory"},
		{"build-command"},
		{"start-command"},
		{"pre-deploy-command"},
		{"health-check-path"},
		{"publish-directory"},
		{"cron-command"},
		{"cron-schedule"},
		{"registry-credential"},
		{"auto-deploy"},
		{"build-filter-path"},
		{"build-filter-ignored-path"},
		{"num-instances"},
		{"max-shutdown-delay"},
		{"previews"},
		{"maintenance-mode"},
		{"maintenance-mode-uri"},
		{"ip-allow-list"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag, "flag %s should be registered", tt.flagName)
		})
	}
}

func TestServiceUpdateCommandStructure(t *testing.T) {
	cmd := newServiceUpdateTestCmd()

	t.Run("command use string is update <service>", func(t *testing.T) {
		require.Equal(t, "update <service>", cmd.Use)
	})

	t.Run("command requires exactly 1 positional arg", func(t *testing.T) {
		require.Error(t, cmd.Args(cmd, []string{}))
		require.NoError(t, cmd.Args(cmd, []string{"service"}))
		require.Error(t, cmd.Args(cmd, []string{"arg1", "arg2"}))
	})

	t.Run("command has RunE defined", func(t *testing.T) {
		require.NotNil(t, cmd.RunE)
	})
}

type serviceUpdateHarness struct {
	t      *testing.T
	server *renderapi.Server
	deps   *dependencies.Dependencies
}

func newServiceUpdateHarness(t *testing.T) serviceUpdateHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: serviceTestWorkspaceID, Name: serviceTestWorkspaceName}))
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_WORKSPACE", "")
	require.NoError(t, (&config.Config{
		Workspace:     serviceTestWorkspaceID,
		WorkspaceName: serviceTestWorkspaceName,
	}).Persist())

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{
			StdinTTY:  false,
			StdoutTTY: false,
			StderrTTY: false,
		}, nil
	}

	return serviceUpdateHarness{
		t:      t,
		server: server,
		deps:   deps,
	}
}

// execute invokes `services update` on the cobra application
func (h serviceUpdateHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()

	root, stdout, stderr := h.setupCmd()
	root.SetArgs(append([]string{"services", "update"}, extraArgs...))

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

// setupCmd builds the Cobra application under test
func (h serviceUpdateHarness) setupCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	h.t.Helper()

	root := newRootCmd()
	services := cobraServicesCommand()
	services.AddCommand(newServiceUpdateCmd(h.deps))
	root.AddCommand(services)
	setupRootCmdPersistentRun(root, h.deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	return root, &stdout, &stderr
}

func TestServiceUpdate_JSONOutput_ReturnsServiceOutEnvelope(t *testing.T) {
	harness := newServiceUpdateHarness(t)
	project := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "Website", OwnerId: serviceTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	envID := project.Env("production").Id
	svc := harness.server.Services.Add(renderapi.NewWebService(renderapi.WebServiceAttrs{
		Service: renderapi.CommonServiceAttrs{
			Name:          "json-service",
			OwnerID:       serviceTestWorkspaceID,
			EnvironmentID: envID,
		},
	}))

	result, err := harness.execute(svc.Id,
		"--name", "json-service-renamed",
		"--output", "json",
	)
	require.NoError(t, err)

	assert.True(t, harness.server.HasRequest("PATCH", "/services/"+svc.Id))

	body := testrequire.ParseJSONMap(t, result.Stdout)
	testassert.MapContains(t, body, map[string]any{
		"data": map[string]any{
			"id":            svc.Id,
			"name":          "json-service",
			"type":          string(client.WebService),
			"projectId":     project.Project.Id,
			"environmentId": envID,
			"serviceDetails": map[string]any{
				"runtime":            string(client.ServiceRuntimeNode),
				"envSpecificDetails": map[string]any{},
			},
		},
	})
	assert.NotContains(t, body, "id")
}
