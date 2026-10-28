package cmd

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func executeBlueprintCreate(t *testing.T, server *renderapi.Server, args ...string) (command.ExecutionResult, CommandResult) {
	t.Helper()
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
	root.SetArgs(args)
	result := runExecution(root, time.Now())
	return result, CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

func TestBlueprintCommandsHidden(t *testing.T) {
	server := renderapi.NewServer(t)
	result, help := executeBlueprintCreate(t, server, "ea", "--help")
	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, help.Stdout, "blueprints")

	result, parentHelp := executeBlueprintCreate(t, server, "ea", "blueprints", "--help")
	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, parentHelp.Stdout, "Create a Blueprint")
	require.Empty(t, server.Requests)
}

func TestBlueprintCreateNotImplemented(t *testing.T) {
	server := renderapi.NewServer(t)
	result, output := executeBlueprintCreate(t, server, "ea", "blueprints", "create", "--output", "json")
	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "blueprint creation is not implemented yet")
	require.Empty(t, server.Requests)
}

func TestBlueprintCreateHelp(t *testing.T) {
	server := renderapi.NewServer(t)
	result, output := executeBlueprintCreate(t, server, "ea", "blueprints", "create", "--help")
	require.Equal(t, 0, result.ExitCode)
	for _, flag := range []string{"--repo", "--branch", "--path", "--name", "--auto-sync", "--existing-resources", "--output"} {
		require.Contains(t, output.Stdout, flag)
	}
	require.NotContains(t, output.Stdout, "--workspace")
	require.Contains(t, output.Stdout, "relative to the repository root")
	require.Contains(t, output.Stdout, "create_new")
	require.Empty(t, server.Requests)
}

func TestBlueprintCreateInputParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    BlueprintCreateInput
		wantErr string
	}{
		{
			name: "omitted flags remain nil despite Cobra defaults",
			want: BlueprintCreateInput{},
		},
		{
			name: "explicit non-default values are present",
			args: []string{"--repo", "https://github.com/acme/app", "--branch", "release", "--path", "infra/render.yaml", "--name", "production", "--auto-sync=false", "--existing-resources", "adopt"},
			want: BlueprintCreateInput{
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
			args: []string{"--path", "render.yaml", "--auto-sync=true", "--existing-resources", "create_new"},
			want: BlueprintCreateInput{
				Path:              new("render.yaml"),
				AutoSync:          new(true),
				ExistingResources: new(bptypes.CreateNew),
			},
		},
		{
			name:    "invalid existing resources mode",
			args:    []string{"--existing-resources", "delete"},
			wantErr: "--existing-resources must be create_new or adopt",
		},
		{
			name:    "hyphenated resource mode is rejected",
			args:    []string{"--existing-resources", "create-new"},
			wantErr: "--existing-resources must be create_new or adopt",
		},
		{
			name:    "explicit empty resource mode",
			args:    []string{"--existing-resources", ""},
			wantErr: "--existing-resources must be create_new or adopt",
		},
		{
			name:    "explicit empty repository",
			args:    []string{"--repo", ""},
			wantErr: "--repo must not be empty",
		},
		{
			name:    "explicit empty path",
			args:    []string{"--path", ""},
			wantErr: "--path must not be empty",
		},
		{
			name:    "explicit empty branch",
			args:    []string{"--branch", ""},
			wantErr: "--branch must not be empty",
		},
		{
			name:    "explicit empty name",
			args:    []string{"--name", ""},
			wantErr: "--name must not be empty",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newBlueprintCreateCmd(nil)
			cmd.SetContext(command.SetFormatInContext(context.Background(), new(command.TEXT)))
			require.NoError(t, cmd.ParseFlags(tc.args))
			var input BlueprintCreateInput
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
