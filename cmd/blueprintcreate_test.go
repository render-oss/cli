package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

type executeBlueprintCommandOption func(*executeBlueprintCommandOptions)

type executeBlueprintCommandOptions struct {
	stdin string
}

// withStdin feeds stdin to the command.
func withStdin(stdin string) executeBlueprintCommandOption {
	return func(o *executeBlueprintCommandOptions) { o.stdin = stdin }
}

func executeBlueprintCommand(t *testing.T, server *renderapi.Server, args []string, opts ...executeBlueprintCommandOption) (command.ExecutionResult, CommandResult) {
	t.Helper()
	var o executeBlueprintCommandOptions
	for _, opt := range opts {
		opt(&o)
	}
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	root := newRootCmd()
	ea := newEarlyAccessCmd()
	root.AddCommand(ea)
	setupBlueprintCommands(ea, deps)
	setupRootCmdPersistentRun(root, deps)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(o.stdin))
	root.SetArgs(args)
	result := runExecution(root, time.Now())
	return result, CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

func TestBlueprintCommandsHidden(t *testing.T) {
	server := renderapi.NewServer(t)
	result, help := executeBlueprintCommand(t, server, []string{"ea", "--help"})
	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, help.Stdout, "blueprints")

	result, parentHelp := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "--help"})
	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, parentHelp.Stdout, "Create a Blueprint")
	require.NotContains(t, parentHelp.Stdout, "Update a Blueprint")
	require.NotContains(t, parentHelp.Stdout, "Plan a sync of a Blueprint")
	require.Empty(t, server.Requests)
}

func newBlueprintCreateServer(t *testing.T) *renderapi.Server {
	t.Helper()
	server := renderapi.NewServer(t)
	workspace := testids.WorkspaceID("blueprint-create")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: workspace}))
	t.Setenv("RENDER_WORKSPACE", workspace)
	return server
}

func TestBlueprintCreateText(t *testing.T) {
	server := newBlueprintCreateServer(t)
	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--repo", "https://github.com/acme/my-app", "--output", "text"})
	require.Equal(t, 0, result.ExitCode, output.Stderr)
	require.Contains(t, output.Stdout, "Blueprint my-app")
	require.Contains(t, output.Stdout, "Planned, waiting for approval")
	require.Contains(t, output.Stdout, "render ea blueprints approve")
	require.Empty(t, output.Stderr)
}

func TestBlueprintCreatePlanningOutcomes(t *testing.T) {
	t.Run("maps planning outcomes to exit codes", func(t *testing.T) {
		bp := *renderapi.NewBlueprint(bptypes.Blueprint{Name: "my-app"})
		errorNotices := []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "invalid service configuration"}}
		for _, tc := range []struct {
			name         string
			response     bptypes.CreateBlueprintResponse
			wantExitCode int
		}{
			{
				name: "planned",
				response: bptypes.CreateBlueprintResponse{
					Blueprint: bp, PlanningOutcome: bptypes.PlanningOutcomePlanned,
					Sync: &bptypes.SyncWithPlan{Id: testids.SyncID("planned")}, Notices: []bptypes.BlueprintNotice{},
				},
			},
			{
				name: "no changes",
				response: bptypes.CreateBlueprintResponse{
					Blueprint: bp, PlanningOutcome: bptypes.PlanningOutcomeNoChanges, Notices: []bptypes.BlueprintNotice{},
				},
			},
			{
				name: "unknown planning outcome does not fail creation",
				response: bptypes.CreateBlueprintResponse{
					Blueprint: bp, PlanningOutcome: bptypes.PlanningOutcome("unknown"), Notices: []bptypes.BlueprintNotice{},
				},
				wantExitCode: 0,
			},
			{
				name: "failed without a saved sync",
				response: bptypes.CreateBlueprintResponse{
					Blueprint: bp, PlanningOutcome: bptypes.PlanningOutcomeFailed, Notices: errorNotices,
				},
				wantExitCode: 1,
			},
			{
				name: "failed with a saved sync",
				response: bptypes.CreateBlueprintResponse{
					Blueprint: bp, PlanningOutcome: bptypes.PlanningOutcomeFailed, Notices: errorNotices,
					Sync: &bptypes.SyncWithPlan{Id: testids.SyncID("failed"), State: bptypes.SyncStateError},
				},
				wantExitCode: 1,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				server := newBlueprintCreateServer(t)
				server.Blueprints.RespondWithBodyForCreate(tc.response)

				result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--repo", "https://github.com/acme/my-app", "--output", "json"})
				require.Equal(t, tc.wantExitCode, result.ExitCode)
				require.Empty(t, output.Stderr)
				var out blueprint.CreateOut
				require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
				require.Equal(t, &tc.response, out.Data)
			})
		}
	})
}

func TestBlueprintCreateHTTPError(t *testing.T) {
	server := newBlueprintCreateServer(t)
	server.Blueprints.RespondWithErrorForCreate(http.StatusConflict, "repository is not connected")
	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--repo", "https://github.com/acme/my-app", "--output", "json"})
	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "repository is not connected")
}

func TestBlueprintCreateRequiresWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	t.Setenv("RENDER_WORKSPACE", "")
	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--repo", "https://github.com/acme/my-app", "--output", "json"})
	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "render workspace set")
	require.Empty(t, server.Requests)
}

func TestBlueprintCreateHelp(t *testing.T) {
	server := renderapi.NewServer(t)
	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--help"})
	require.Equal(t, 0, result.ExitCode)
	require.Empty(t, server.Requests)

	testassert.ContainsInOrder(t, output.Stdout,
		"--auto-sync",
		"--branch",
		"--existing-resources",
		"create_new",
		"--name",
		"--path",
		"relative to the repository root",
		"--repo",
		"--output",
		"defaults to render.yaml",
		"~/.render/state/manual-env-vars/<blueprintID>.json",
		"RENDER_CLI_CONFIG_DIR",
	)
	require.NotContains(t, output.Stdout, "--workspace")
}

func TestBlueprintCreateRequiresRepositoryOutsideAGitRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	server := newBlueprintCreateServer(t)
	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--output", "json"})
	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "cannot determine the repository from the local checkout; provide --repo")
	require.Empty(t, server.Requests)
}

func TestBlueprintCreateReportsGitConfigError(t *testing.T) {
	server := newBlueprintCreateServer(t)
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[invalid\n"), 0o644))
	t.Setenv("LC_ALL", "C")
	t.Chdir(dir)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--output", "json"})

	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "cannot determine the repository from the local checkout; provide --repo")
	require.Contains(t, output.Stderr, "bad config line 1")
	require.Empty(t, server.Requests)
}

func TestBlueprintCreateInputParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    blueprint.CreateInput
		wantErr string
	}{
		{
			name: "omitted optional flags remain nil despite Cobra defaults",
			args: []string{"--repo", "https://github.com/acme/app"},
			want: blueprint.CreateInput{Repo: new("https://github.com/acme/app")},
		},
		{
			name: "omitted flags remain nil for later inference",
			want: blueprint.CreateInput{},
		},
		{
			name: "explicit non-default values are present",
			args: []string{"--repo", "https://github.com/acme/app", "--branch", "release", "--path", "infra/render.yaml", "--name", "production", "--auto-sync=false", "--existing-resources", "adopt"},
			want: blueprint.CreateInput{
				Repo:              new("https://github.com/acme/app"),
				Branch:            new("release"),
				Path:              new("infra/render.yaml"),
				Name:              new("production"),
				AutoSync:          new(false),
				ExistingResources: new(bptypes.AdoptExisting),
			},
		},
		{
			name: "explicit default values remain present",
			args: []string{"--repo", "https://github.com/acme/app", "--path", "render.yaml", "--auto-sync=true", "--existing-resources", "create_new"},
			want: blueprint.CreateInput{
				Repo:              new("https://github.com/acme/app"),
				Path:              new("render.yaml"),
				AutoSync:          new(true),
				ExistingResources: new(bptypes.CreateNew),
			},
		},
		{
			name:    "invalid existing resources mode",
			args:    []string{"--repo", "https://github.com/acme/app", "--existing-resources", "delete"},
			wantErr: "--existing-resources must be create_new or adopt",
		},
		{
			name:    "hyphenated resource mode is rejected",
			args:    []string{"--repo", "https://github.com/acme/app", "--existing-resources", "create-new"},
			wantErr: "--existing-resources must be create_new or adopt",
		},
		{
			name:    "explicit empty resource mode",
			args:    []string{"--repo", "https://github.com/acme/app", "--existing-resources", ""},
			wantErr: "--existing-resources must be create_new or adopt",
		},
		{
			name:    "explicit empty repository",
			args:    []string{"--repo", ""},
			wantErr: "--repo must be a valid https:// or ssh:// URL",
		},
		{
			name:    "malformed repository URL",
			args:    []string{"--repo", "https://github.com/acme/%gh"},
			wantErr: "--repo must be a valid https:// or ssh:// URL",
		},
		{
			name:    "explicit empty path",
			args:    []string{"--repo", "https://github.com/acme/app", "--path", ""},
			wantErr: "--path must not be empty",
		},
		{
			name:    "explicit empty branch",
			args:    []string{"--repo", "https://github.com/acme/app", "--branch", ""},
			wantErr: "--branch must not be empty",
		},
		{
			name:    "explicit empty name",
			args:    []string{"--repo", "https://github.com/acme/app", "--name", ""},
			wantErr: "--name must not be empty",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newBlueprintCreateCmd(nil)
			cmd.SetContext(command.SetFormatInContext(context.Background(), new(command.TEXT)))
			require.NoError(t, cmd.ParseFlags(tc.args))
			var input blueprint.CreateInput
			err := command.ParseCommand(cmd, nil, &input)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, input)
		})
	}
}

func TestBlueprintCreateRejectsPositionalArguments(t *testing.T) {
	cmd := newBlueprintCreateCmd(nil)
	require.ErrorContains(t, cmd.Args(cmd, []string{"unexpected"}), "unknown command")
}

func TestBlueprintCreateGitDefaults(t *testing.T) {
	server := newBlueprintCreateServer(t)

	// Set up an SSH origin and a release branch, then run from a subdirectory.
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:acme/app.git"}})
	require.NoError(t, err)
	require.NoError(t, repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/release")))
	subdir := filepath.Join(dir, "infra")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	t.Chdir(subdir)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--path", "infra/render.yaml", "--output", "json"})

	require.Equal(t, 0, result.ExitCode, output.Stderr)

	created := server.Blueprints.Only(t)
	require.Equal(t, "app", created.Name)

	require.Equal(t, "https://github.com/acme/app.git", created.Repo)
	require.Equal(t, "release", created.Branch)
	require.Equal(t, "infra/render.yaml", created.Path)
}

func createResponseWithRequiredManualEnvVars() bptypes.CreateBlueprintResponse {
	return bptypes.CreateBlueprintResponse{
		Blueprint:       *renderapi.NewBlueprint(bptypes.Blueprint{Id: testids.BlueprintID("manual-env-file")}),
		PlanningOutcome: bptypes.PlanningOutcomePlanned,
		Notices:         []bptypes.BlueprintNotice{},
		Sync: &bptypes.SyncWithPlan{RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{
			{ServiceType: bptypes.ManualEnvVarTargetTypeWeb, ServiceName: "app", Key: "API_KEY"},
		}},
	}
}

func TestBlueprintCreateWritesManualEnvVarsFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	server := newBlueprintCreateServer(t)
	response := createResponseWithRequiredManualEnvVars()
	server.Blueprints.RespondWithBodyForCreate(response)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--repo", "https://github.com/acme/app", "--output", "json"})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	require.Empty(t, output.Stderr)

	filename := filepath.Join(configDir, "state", "manual-env-vars", response.Blueprint.Id+".json")
	require.FileExists(t, filename)

	var out blueprint.CreateOut
	require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
	require.Equal(t, &response, out.Data)
	testassert.ContainsInOrder(t, *out.Meta.Next,
		filename+" lists these environment variables with empty values.",
		"then run render ea blueprints approve "+response.Blueprint.Id,
	)
}

func TestBlueprintCreatePrintsBlueprintWhenPreparingManualEnvVarsFileFails(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	server := newBlueprintCreateServer(t)
	response := createResponseWithRequiredManualEnvVars()
	server.Blueprints.RespondWithBodyForCreate(response)
	// Make the expected state directory a regular file so creating
	// state/manual-env-vars fails after the Blueprint is created.
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "state"), nil, 0o600))

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "create", "--repo", "https://github.com/acme/app", "--output", "json"})

	require.Equal(t, 1, result.ExitCode)
	require.Contains(t, output.Stderr, "write manual environment variables file")

	var out blueprint.CreateOut
	require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
	require.Equal(t, &response, out.Data)
	require.Nil(t, out.Meta.Next)
}
