package cmd

import (
	"bytes"
	"slices"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateCmdRejectsPositionalArgs(t *testing.T) {
	require.Error(t, ServiceCreateCmd.Args(ServiceCreateCmd, []string{"unexpected"}))
	require.NoError(t, ServiceCreateCmd.Args(ServiceCreateCmd, []string{}))
}

func TestServiceAliasResolvesToCreateCommand(t *testing.T) {
	plural, _, err := rootCmd.Find([]string{"services", "create"})
	require.NoError(t, err)
	require.Same(t, ServiceCreateCmd, plural)

	alias, _, err := rootCmd.Find([]string{"service", "create"})
	require.NoError(t, err)
	require.Same(t, ServiceCreateCmd, alias)
}

func TestServiceCreateNoArgsValidationPreventsExecution(t *testing.T) {
	called := false
	cmd := &cobra.Command{
		Use:  "create",
		Args: ServiceCreateCmd.Args,
		RunE: func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		},
	}
	cmd.SetArgs([]string{"unexpected"})

	err := cmd.Execute()
	require.Error(t, err)
	require.False(t, called)
}

// Command flags carry shell text that the service runs verbatim, so whitespace
// at either end can be significant: in `tail\ ` the escaped space belongs to
// the final argument. The quoted form is a control that never relied on
// trailing whitespace surviving.
var serviceCommandTextCases = []struct {
	name  string
	value string
}{
	{name: "escaped trailing space", value: `printf '%s' tail\ `},
	{name: "escaped trailing tab", value: "printf '%s' tail\\\t"},
	{name: "quoted trailing space", value: `printf '%s' 'tail '`},
}

type serviceCreateHarness struct {
	t      *testing.T
	server *renderapi.Server
	deps   *dependencies.Dependencies
}

func newServiceCreateHarness(t *testing.T) serviceCreateHarness {
	t.Helper()

	// Same fake API, workspace config, and non-TTY dependencies as update.
	h := newServiceUpdateHarness(t)
	return serviceCreateHarness{t: t, server: h.server, deps: h.deps}
}

// execute invokes `services create` on a freshly constructed command tree.
func (h serviceCreateHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()

	root := newRootCmd()
	services := cobraServicesCommand()
	services.AddCommand(newServiceCreateCmd())
	root.AddCommand(services)
	setupRootCmdPersistentRun(root, h.deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"services", "create"}, extraArgs...))

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

// createServiceDetails returns the serviceDetails object of the recorded POST body.
func (h serviceCreateHarness) createServiceDetails() map[string]any {
	h.t.Helper()

	rec, ok := h.server.LastRequest("POST", "/services")
	require.True(h.t, ok, "expected a POST /services request to be recorded")
	return testrequire.SubMap(h.t, testrequire.ParseJSONMap(h.t, string(rec.Body)), "serviceDetails")
}

var serviceCreateWebArgs = []string{
	"--name", "command-text", "--type", "web_service", "--repo", "https://github.com/org/repo",
	"--runtime", "node", "--output", "json",
}

func TestServiceCreate_CommandFlagsSendExactCommandText(t *testing.T) {
	flags := []struct {
		flag        string
		args        func(command string) []string
		wantDetails func(command string) map[string]any
	}{
		{
			flag: "build-command",
			args: func(command string) []string {
				return append(slices.Clone(serviceCreateWebArgs), "--build-command", command, "--start-command", "npm start")
			},
			wantDetails: func(command string) map[string]any {
				return map[string]any{
					"runtime":            "node",
					"envSpecificDetails": map[string]any{"buildCommand": command, "startCommand": "npm start"},
				}
			},
		},
		{
			flag: "start-command",
			args: func(command string) []string {
				return append(slices.Clone(serviceCreateWebArgs), "--build-command", "npm ci", "--start-command", command)
			},
			wantDetails: func(command string) map[string]any {
				return map[string]any{
					"runtime":            "node",
					"envSpecificDetails": map[string]any{"buildCommand": "npm ci", "startCommand": command},
				}
			},
		},
		{
			flag: "pre-deploy-command",
			args: func(command string) []string {
				return append(slices.Clone(serviceCreateWebArgs),
					"--build-command", "npm ci", "--start-command", "npm start", "--pre-deploy-command", command)
			},
			wantDetails: func(command string) map[string]any {
				return map[string]any{
					"runtime":            "node",
					"envSpecificDetails": map[string]any{"buildCommand": "npm ci", "startCommand": "npm start"},
					"preDeployCommand":   command,
				}
			},
		},
		{
			// Cron jobs carry the cron command as envSpecificDetails.startCommand.
			flag: "cron-command",
			args: func(command string) []string {
				return []string{
					"--name", "command-text", "--type", "cron_job", "--repo", "https://github.com/org/repo",
					"--runtime", "node", "--cron-schedule", "0 * * * *", "--build-command", "npm ci",
					"--cron-command", command, "--output", "json",
				}
			},
			wantDetails: func(command string) map[string]any {
				return map[string]any{
					"runtime":            "node",
					"schedule":           "0 * * * *",
					"envSpecificDetails": map[string]any{"buildCommand": "npm ci", "startCommand": command},
				}
			},
		},
	}

	for _, ff := range flags {
		for _, tc := range serviceCommandTextCases {
			t.Run(ff.flag+"/"+tc.name, func(t *testing.T) {
				harness := newServiceCreateHarness(t)

				_, err := harness.execute(ff.args(tc.value)...)
				require.NoError(t, err)

				assert.Equal(t, ff.wantDetails(tc.value), harness.createServiceDetails())
			})
		}
	}
}

func TestServiceCreate_BlankCommandFlagsAreTreatedAsUnset(t *testing.T) {
	t.Run("pre-deploy-command is omitted", func(t *testing.T) {
		harness := newServiceCreateHarness(t)

		_, err := harness.execute(append(slices.Clone(serviceCreateWebArgs),
			"--build-command", "npm ci", "--start-command", "npm start", "--pre-deploy-command", " \t ")...)
		require.NoError(t, err)

		assert.Equal(t, map[string]any{
			"runtime":            "node",
			"envSpecificDetails": map[string]any{"buildCommand": "npm ci", "startCommand": "npm start"},
		}, harness.createServiceDetails())
	})

	t.Run("static site build-command is omitted", func(t *testing.T) {
		harness := newServiceCreateHarness(t)

		_, err := harness.execute(
			"--name", "command-text", "--type", "static_site", "--repo", "https://github.com/org/repo",
			"--build-command", " \t ", "--output", "json",
		)
		require.NoError(t, err)

		assert.Equal(t, map[string]any{}, harness.createServiceDetails())
	})

	// A blank required command counts as missing rather than as an empty command.
	t.Run("required start-command is rejected before POST", func(t *testing.T) {
		harness := newServiceCreateHarness(t)

		_, err := harness.execute(append(slices.Clone(serviceCreateWebArgs),
			"--build-command", "npm ci", "--start-command", " \t ")...)
		require.ErrorContains(t, err, "--build-command and --start-command are required when runtime is native")
		assert.False(t, harness.server.HasRequest("POST", "/services"))
	})

	t.Run("required cron-command is rejected before POST", func(t *testing.T) {
		harness := newServiceCreateHarness(t)

		_, err := harness.execute(
			"--name", "command-text", "--type", "cron_job", "--repo", "https://github.com/org/repo",
			"--runtime", "node", "--cron-schedule", "0 * * * *", "--build-command", "npm ci",
			"--cron-command", " \t ", "--output", "json",
		)
		require.ErrorContains(t, err, "cron-command and cron-schedule are required for cron jobs")
		assert.False(t, harness.server.HasRequest("POST", "/services"))
	})
}

// createBody runs `services create` and returns the decoded POST /services body.
func (h serviceCreateHarness) createBody(args ...string) map[string]any {
	h.t.Helper()

	_, err := h.execute(append(args, "--output", "json")...)
	require.NoError(h.t, err)
	rec, ok := h.server.LastRequest("POST", "/services")
	require.True(h.t, ok, "expected a POST /services request to be recorded")
	return testrequire.ParseJSONMap(h.t, string(rec.Body))
}

// addCloneSource adds svc to the workspace as a clone source and returns its
// ID. Web services, private services and background workers get a native
// pre-deploy command.
func (h serviceCreateHarness) addCloneSource(svc *client.Service) string {
	h.t.Helper()

	svc.OwnerId = serviceTestWorkspaceID
	svc.Repo = pointers.From("https://github.com/org/repo")
	svc.Branch = pointers.From("main")
	var envSpecific client.EnvSpecificDetails
	require.NoError(h.t, envSpecific.FromNativeEnvironmentDetails(client.NativeEnvironmentDetails{
		BuildCommand: "npm ci", StartCommand: "npm start", PreDeployCommand: pointers.From("npm run migrate"),
	}))
	switch svc.Type {
	case client.WebService:
		details, err := svc.ServiceDetails.AsWebServiceDetails()
		require.NoError(h.t, err)
		details.EnvSpecificDetails = envSpecific
		require.NoError(h.t, svc.ServiceDetails.FromWebServiceDetails(details))
	case client.PrivateService:
		details, err := svc.ServiceDetails.AsPrivateServiceDetails()
		require.NoError(h.t, err)
		details.EnvSpecificDetails = envSpecific
		require.NoError(h.t, svc.ServiceDetails.FromPrivateServiceDetails(details))
	case client.BackgroundWorker:
		details, err := svc.ServiceDetails.AsBackgroundWorkerDetails()
		require.NoError(h.t, err)
		details.EnvSpecificDetails = envSpecific
		require.NoError(h.t, svc.ServiceDetails.FromBackgroundWorkerDetails(details))
	}
	return h.server.Services.Add(svc).Id
}

func (h serviceCreateHarness) addWebCloneSource() string {
	h.t.Helper()

	return h.addCloneSource(renderapi.NewWebService(renderapi.WebServiceAttrs{
		Service: renderapi.CommonServiceAttrs{Name: "clone-source", RootDir: "services/api"},
		Details: renderapi.WebServiceDetailsAttrs{HealthCheckPath: "/source-health"},
	}))
}

// The optional settings a clone clears when their flag is given but blank, and
// where each appears in the create request.
var clearableCloneFlags = []struct {
	flag        string
	sourceValue string
	override    string
	inBody      func(t *testing.T, body map[string]any) (any, bool)
}{
	{
		flag: "root-directory", sourceValue: "services/api", override: "services/web",
		inBody: func(_ *testing.T, body map[string]any) (any, bool) {
			value, ok := body["rootDir"]
			return value, ok
		},
	},
	{
		flag: "health-check-path", sourceValue: "/source-health", override: "/ready",
		inBody: func(t *testing.T, body map[string]any) (any, bool) {
			value, ok := testrequire.SubMap(t, body, "serviceDetails")["healthCheckPath"]
			return value, ok
		},
	},
	{
		flag: "pre-deploy-command", sourceValue: "npm run migrate", override: "bin/migrate",
		inBody: func(t *testing.T, body map[string]any) (any, bool) {
			value, ok := testrequire.SubMap(t, body, "serviceDetails")["preDeployCommand"]
			return value, ok
		},
	},
}

func TestServiceCreate_CloneBlankOptionalFlagClearsSourceValue(t *testing.T) {
	for _, ff := range clearableCloneFlags {
		t.Run(ff.flag, func(t *testing.T) {
			clone := func(h serviceCreateHarness, extraArgs ...string) map[string]any {
				return h.createBody(append([]string{"--from", h.addWebCloneSource(), "--name", "clone"}, extraArgs...)...)
			}

			t.Run("omitted inherits the source value", func(t *testing.T) {
				value, ok := ff.inBody(t, clone(newServiceCreateHarness(t)))
				require.True(t, ok)
				assert.Equal(t, ff.sourceValue, value)
			})

			t.Run("non-empty overrides the source value", func(t *testing.T) {
				value, ok := ff.inBody(t, clone(newServiceCreateHarness(t), "--"+ff.flag, ff.override))
				require.True(t, ok)
				assert.Equal(t, ff.override, value)
			})

			for _, blank := range []struct{ name, value string }{{"empty", ""}, {"whitespace-only", " \t "}} {
				t.Run(blank.name+" clears the source value", func(t *testing.T) {
					value, ok := ff.inBody(t, clone(newServiceCreateHarness(t), "--"+ff.flag, blank.value))
					assert.False(t, ok, "expected no %s in the request, got %v", ff.flag, value)
				})
			}

			t.Run("empty on a direct create is omitted", func(t *testing.T) {
				body := newServiceCreateHarness(t).createBody(
					"--name", "direct", "--type", "web_service", "--repo", "https://github.com/org/repo",
					"--runtime", "node", "--build-command", "npm ci", "--start-command", "npm start",
					"--"+ff.flag, "",
				)
				value, ok := ff.inBody(t, body)
				assert.False(t, ok, "expected no %s in the request, got %v", ff.flag, value)
			})
		})
	}
}

// Clearing applies to every service type that has the setting.
func TestServiceCreate_CloneBlankOptionalFlagClearsAcrossServiceTypes(t *testing.T) {
	t.Run("static site root directory", func(t *testing.T) {
		h := newServiceCreateHarness(t)
		source := h.addCloneSource(renderapi.NewStaticSite(renderapi.StaticSiteAttrs{
			Service: renderapi.CommonServiceAttrs{Name: "site", RootDir: "site"},
		}))

		body := h.createBody("--from", source, "--name", "clone", "--root-directory", "")

		assert.NotContains(t, body, "rootDir")
		assert.Equal(t, "dist", testrequire.SubMap(t, body, "serviceDetails")["publishPath"])
	})

	for _, source := range []*client.Service{
		renderapi.NewPrivateService(renderapi.PrivateServiceAttrs{}),
		renderapi.NewBackgroundWorker(renderapi.BackgroundWorkerAttrs{}),
	} {
		t.Run(string(source.Type)+" pre-deploy command", func(t *testing.T) {
			h := newServiceCreateHarness(t)
			id := h.addCloneSource(source)

			details := testrequire.SubMap(t, h.createBody("--from", id, "--name", "clone"), "serviceDetails")
			assert.Equal(t, "npm run migrate", details["preDeployCommand"])

			details = testrequire.SubMap(t, h.createBody("--from", id, "--name", "clone", "--pre-deploy-command", ""), "serviceDetails")
			assert.NotContains(t, details, "preDeployCommand")
		})
	}

	// A blank flag the type does not support is accepted and sends nothing.
	t.Run("cron job ignores blank web-only flags", func(t *testing.T) {
		h := newServiceCreateHarness(t)
		source := h.addCloneSource(renderapi.NewCronJob(renderapi.CronJobAttrs{}))

		details := testrequire.SubMap(t, h.createBody("--from", source, "--name", "clone",
			"--health-check-path", "", "--pre-deploy-command", ""), "serviceDetails")

		assert.NotContains(t, details, "healthCheckPath")
		assert.NotContains(t, details, "preDeployCommand")
	})
}

// Other blank flags still count as not given, so a clone keeps inheriting
// required, source and plan settings.
func TestServiceCreate_CloneBlankRequiredFlagsInheritSourceValue(t *testing.T) {
	h := newServiceCreateHarness(t)

	body := h.createBody("--from", h.addWebCloneSource(), "--name", "clone",
		"--branch", "", "--plan", "", "--build-command", "", "--start-command", "")

	assert.Equal(t, "main", body["branch"])
	details := testrequire.SubMap(t, body, "serviceDetails")
	assert.Equal(t, "starter", details["plan"])
	assert.Equal(t, map[string]any{"buildCommand": "npm ci", "startCommand": "npm start"}, details["envSpecificDetails"])
}
