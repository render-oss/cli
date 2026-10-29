package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
	"github.com/render-oss/cli/pkg/command"
)

var testUpdateBlueprintID = testids.BlueprintID("update")

func TestBlueprintUpdateHelp(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, "ea", "blueprints", "update", "--help")

	require.Equal(t, 0, result.ExitCode)
	require.Contains(t, output.Stdout, "update <blueprintId>")
	for _, flag := range []string{"--name <NAME>", "--branch <BRANCH>", "--path <PATH>", "--auto-sync"} {
		require.Contains(t, output.Stdout, flag)
	}
	require.NotContains(t, output.Stdout, "--repo")
	require.NotContains(t, output.Stdout, "--workspace")
	require.Empty(t, server.Requests)
}

func TestBlueprintUpdateNotImplemented(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, "ea", "blueprints", "update", testUpdateBlueprintID, "--name", "app", "--output", "json")

	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "blueprint update is not implemented yet")
	require.Empty(t, server.Requests)
}

func TestBlueprintUpdateInputParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    blueprint.UpdateInput
		wantErr string
	}{
		{
			name: "only supplied settings are present",
			args: []string{testUpdateBlueprintID, "--branch", "release"},
			want: blueprint.UpdateInput{BlueprintID: testUpdateBlueprintID, Branch: new("release")},
		},
		{
			name: "all settings",
			args: []string{testUpdateBlueprintID, "--name", "app", "--branch", "release", "--path", "infra/render.yaml", "--auto-sync=true"},
			want: blueprint.UpdateInput{
				BlueprintID: testUpdateBlueprintID,
				Name:        new("app"),
				Branch:      new("release"),
				Path:        new("infra/render.yaml"),
				AutoSync:    new(true),
			},
		},
		{
			name: "disabling auto-sync is present",
			args: []string{testUpdateBlueprintID, "--auto-sync=false"},
			want: blueprint.UpdateInput{BlueprintID: testUpdateBlueprintID, AutoSync: new(false)},
		},
		{
			name:    "input is validated",
			args:    []string{testUpdateBlueprintID},
			wantErr: "provide at least one of --name, --branch, --path, or --auto-sync",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newBlueprintUpdateCmd(nil)
			cmd.SetContext(command.SetFormatInContext(context.Background(), new(command.TEXT)))

			require.NoError(t, cmd.ParseFlags(tc.args))

			var input blueprint.UpdateInput
			err := command.ParseCommand(cmd, cmd.Flags().Args(), &input)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, input)
		})
	}
}

func TestBlueprintUpdateRequiresOneBlueprintID(t *testing.T) {
	cmd := newBlueprintUpdateCmd(nil)
	require.Error(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{testUpdateBlueprintID, testUpdateBlueprintID}))
}
