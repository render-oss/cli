package cmd

import (
	"bytes"
	"net/http"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/pointers"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
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

// num-instances stays registered so the command can return a helpful
// "use the dashboard" error rather than Cobra's generic "unknown flag", but it
// must be hidden from help output since it is never a valid update.
func TestServiceUpdateNumInstancesFlagIsHidden(t *testing.T) {
	cmd := newServiceUpdateTestCmd()

	flag := cmd.Flags().Lookup("num-instances")
	require.NotNil(t, flag, "num-instances flag should still be registered")
	assert.True(t, flag.Hidden, "num-instances flag should be hidden from help")
}

// End-to-end check: render the actual --help output and confirm num-instances
// does not leak into it. --max-shutdown-delay is a sibling visible flag that
// proves the help text rendered, so the absence assertion can't pass vacuously.
func TestServiceUpdateHelpOutputOmitsNumInstances(t *testing.T) {
	cmd := newServiceUpdateTestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})

	require.NoError(t, cmd.Execute())

	help := out.String()
	assert.Contains(t, help, "--max-shutdown-delay", "help output should have rendered visible flags")
	assert.NotContains(t, help, "--num-instances", "num-instances should be hidden from help output")
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

func TestServiceUpdate_JSONOutput_UpdatesTopLevelFieldsAndReturnsEnvelope(t *testing.T) {
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

	// Per-flag PATCH-body wiring is covered by TestServiceUpdate_FlagWiresIntoPatchBody.
	// This test owns the JSON output envelope shape, so it sets a single flag.
	result, err := harness.execute(svc.Id,
		"--name", "json-service-renamed",
		"--output", "json",
	)
	require.NoError(t, err)

	assert.True(t, harness.server.HasRequest("PATCH", "/services/"+svc.Id))
	assert.Equal(t, "json-service-renamed", svc.Name)

	body := testrequire.ParseJSONMap(t, result.Stdout)
	testassert.MapContains(t, body, map[string]any{
		"data": map[string]any{
			"id":            svc.Id,
			"name":          "json-service-renamed",
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

// addWebService seeds a web service under a new project and returns it.
func (h serviceUpdateHarness) addWebService(name string) *client.Service {
	h.t.Helper()

	project := h.server.CreateProject(
		renderapi.ProjectAttrs{Name: "Website", OwnerId: serviceTestWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	return h.server.Services.Add(renderapi.NewWebService(renderapi.WebServiceAttrs{
		Service: renderapi.CommonServiceAttrs{
			Name:          name,
			OwnerID:       serviceTestWorkspaceID,
			EnvironmentID: project.Env("production").Id,
		},
	}))
}

// TestServiceUpdate_ParseCommand_HappyPath verifies that every flag the update command
// registers binds to the expected ServiceUpdateInput field with the expected
// type. It drives the real command's flag set through ParseCommand in memory —
// but does not invoke the command.
// It exhaustively covers the CLI flag input <-> `cli:` tag seam that the e2e tests would only exercise indirectly.
// The input->PATCH-body transform is covered separately in pkg/service/update_test.go.
func TestServiceUpdate_ParseCommand_HappyPath(t *testing.T) {
	cmd := newServiceUpdateTestCmd()
	require.NoError(t, cmd.ParseFlags([]string{
		"--name", "svc-name",
		"--repo", "https://github.com/example/repo",
		"--branch", "main",
		"--image", "docker.io/library/nginx:alpine",
		"--plan", "starter",
		"--runtime", string(servicetypes.ServiceRuntimeGo),
		"--root-directory", "app",
		"--build-command", "make build",
		"--start-command", "./run",
		"--pre-deploy-command", "bin/migrate",
		"--health-check-path", "/ready",
		"--publish-directory", "public",
		"--cron-command", "echo hi",
		"--cron-schedule", "0 12 * * *",
		"--registry-credential", "rc-123",
		"--auto-deploy=false",
		"--build-filter-path", "src/**",
		"--build-filter-path", "go.mod",
		"--build-filter-ignored-path", "docs/**",
		"--num-instances", "3",
		"--max-shutdown-delay", "42",
		"--previews", string(servicetypes.PreviewsGenerationManual),
		"--maintenance-mode=true",
		"--maintenance-mode-uri", "https://status.example.com",
		"--ip-allow-list", "cidr=203.0.113.5/32,description=office",
	}))

	var in servicetypes.ServiceUpdateInput
	require.NoError(t, command.ParseCommand(cmd, []string{"srv-abc123"}, &in))

	previews := servicetypes.PreviewsGenerationManual
	runtime := servicetypes.ServiceRuntimeGo
	assert.Equal(t, servicetypes.ServiceUpdateInput{
		Name:                    "svc-name",
		Repo:                    pointers.From("https://github.com/example/repo"),
		Branch:                  pointers.From("main"),
		Image:                   pointers.From("docker.io/library/nginx:alpine"),
		Plan:                    pointers.From("starter"),
		Runtime:                 &runtime,
		RootDirectory:           pointers.From("app"),
		BuildCommand:            pointers.From("make build"),
		StartCommand:            pointers.From("./run"),
		PreDeployCommand:        pointers.From("bin/migrate"),
		HealthCheckPath:         pointers.From("/ready"),
		PublishDirectory:        pointers.From("public"),
		CronCommand:             pointers.From("echo hi"),
		CronSchedule:            pointers.From("0 12 * * *"),
		RegistryCredential:      pointers.From("rc-123"),
		AutoDeploy:              pointers.From(false),
		BuildFilterPaths:        []string{"src/**", "go.mod"},
		BuildFilterIgnoredPaths: []string{"docs/**"},
		NumInstances:            pointers.From(3),
		MaxShutdownDelay:        pointers.From(42),
		Previews:                &previews,
		MaintenanceMode:         pointers.From(true),
		MaintenanceModeURI:      pointers.From("https://status.example.com"),
		IPAllowList:             []string{"cidr=203.0.113.5/32,description=office"},
		ServiceIDOrName:         "srv-abc123",
	}, in)
}

// TestServiceUpdate_FlagWiresIntoPatchBody verifies end-to-end that flags reach
// the serialized PATCH body and that nothing unspecified comes along for the
// ride. Each case asserts the *entire* decoded body by equality, so an extra or
// misplaced field fails the test — stronger than checking individual keys.
// Cases cover a single top-level field, a single serviceDetails field, a mix of
// top-level and serviceDetails fields, and multiple serviceDetails fields.
func TestServiceUpdate_FlagWiresIntoPatchBody(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantBody map[string]any
	}{
		{
			name:     "name maps to top-level name only",
			args:     []string{"--name", "renamed-service"},
			wantBody: map[string]any{"name": "renamed-service"},
		},
		{
			name: "plan maps into serviceDetails only",
			args: []string{"--plan", "starter"},
			wantBody: map[string]any{
				"serviceDetails": map[string]any{"plan": "starter"},
			},
		},
		{
			name: "top-level and detail fields coexist without leaking",
			args: []string{"--name", "renamed-service", "--health-check-path", "/ready"},
			wantBody: map[string]any{
				"name":           "renamed-service",
				"serviceDetails": map[string]any{"healthCheckPath": "/ready"},
			},
		},
		{
			name: "multiple serviceDetails fields land together",
			args: []string{"--health-check-path", "/ready", "--plan", "starter"},
			wantBody: map[string]any{
				"serviceDetails": map[string]any{
					"healthCheckPath": "/ready",
					"plan":            "starter",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := newServiceUpdateHarness(t)
			svc := harness.addWebService("wire-service")

			args := append([]string{svc.Id}, tt.args...)
			args = append(args, "--output", "json")
			_, err := harness.execute(args...)
			require.NoError(t, err)

			rec, ok := harness.server.LastRequest("PATCH", "/services/"+svc.Id)
			require.True(t, ok, "expected a PATCH request to be recorded")
			body := testrequire.ParseJSONMap(t, string(rec.Body))
			assert.Equal(t, tt.wantBody, body)
		})
	}
}

func TestServiceUpdate_TypeIncompatibleFlag_FailsBeforePatch(t *testing.T) {
	harness := newServiceUpdateHarness(t)
	svc := harness.addWebService("web-service")

	// --cron-schedule is rejected only by ValidateForServiceType, which runs
	// against the fetched service type. The command must fail before writing.
	_, err := harness.execute(svc.Id, "--cron-schedule", "0 12 * * *", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--cron-schedule is not supported for web_service")
	assert.False(t, harness.server.HasRequest("PATCH", "/services/"+svc.Id))
}

func TestServiceUpdate_DefaultTextOutput(t *testing.T) {
	harness := newServiceUpdateHarness(t)
	svc := harness.addWebService("text-service")

	result, err := harness.execute(svc.Id, "--name", "text-service-renamed")
	require.NoError(t, err)

	assert.True(t, harness.server.HasRequest("PATCH", "/services/"+svc.Id))
	assert.Contains(t, result.Stdout, "Updated this service")
	assert.Contains(t, result.Stdout, "text-service-renamed")
}

func TestServiceUpdate_ServiceDetailsPatchReachesAPI(t *testing.T) {
	harness := newServiceUpdateHarness(t)
	svc := harness.addWebService("plan-service")

	// --plan flows into the ServiceDetails union; this exercises that the union
	// serializes and is accepted end-to-end (the fake decodes the PATCH body).
	_, err := harness.execute(svc.Id, "--plan", "starter", "--output", "json")
	require.NoError(t, err)

	assert.True(t, harness.server.HasRequest("PATCH", "/services/"+svc.Id))
}

func TestServiceUpdate_UnknownService_FailsToResolveBeforePatch(t *testing.T) {
	harness := newServiceUpdateHarness(t)

	// No service is seeded, so resolving the name finds nothing. The command
	// must surface a resolve error and never attempt a PATCH.
	_, err := harness.execute("does-not-exist", "--name", "renamed", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `failed to resolve service "does-not-exist"`)
	assert.False(t, harness.server.HasRequest("PATCH", "/services/"))
}

func TestServiceUpdate_APIError_SurfacesWithoutWriting(t *testing.T) {
	harness := newServiceUpdateHarness(t)
	svc := harness.addWebService("api-error-service")

	// The fake drains its error queue FIFO across all service requests, so a
	// single queued error lands on the first GET the command issues to resolve
	// and load the service — before any PATCH. This asserts the command
	// propagates an unexpected API error instead of swallowing it, and does not
	// write when a pre-PATCH step fails.
	harness.server.Services.RespondWith(http.StatusInternalServerError)

	_, err := harness.execute(svc.Id, "--name", "renamed", "--output", "json")
	require.Error(t, err)
	assert.False(t, harness.server.HasRequest("PATCH", "/services/"+svc.Id))
}
