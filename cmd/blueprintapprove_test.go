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

var testBlueprintID = testids.BlueprintID("approve")

func TestBlueprintApproveIsHidden(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, "ea", "blueprints", "--help")

	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, output.Stdout, "approve")
	require.Empty(t, server.Requests)
}

func TestBlueprintApproveNotImplemented(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, "ea", "blueprints", "approve", testBlueprintID, "--output", "json")

	require.Equal(t, 1, result.ExitCode)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, "blueprint approval is not implemented yet")
	require.Empty(t, server.Requests)
}

func TestBlueprintApproveHelp(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, "ea", "blueprints", "approve", "--help")

	require.Equal(t, 0, result.ExitCode)

	require.Contains(t, output.Stdout, "approve <blueprintId>")
	require.Contains(t, output.Stdout, "--manual-env-vars-file <PATH>")
	require.Contains(t, output.Stdout, "~/.render/state/manual-env-vars/<blueprintID>.json")
	require.NotContains(t, output.Stdout, "--workspace")

	require.Empty(t, server.Requests)
}

func TestBlueprintApproveInputParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    blueprint.ApproveInput
		wantErr string
	}{
		{
			name: "omitted manual env vars file remains nil",
			args: []string{testBlueprintID},
			want: blueprint.ApproveInput{BlueprintID: testBlueprintID},
		},
		{
			name: "manual env vars file path",
			args: []string{testBlueprintID, "--manual-env-vars-file", "manual-env-vars.json"},
			want: blueprint.ApproveInput{BlueprintID: testBlueprintID, ManualEnvVarsFile: new("manual-env-vars.json")},
		},
		{
			name: "manual env vars from stdin",
			args: []string{testBlueprintID, "--manual-env-vars-file", "-"},
			want: blueprint.ApproveInput{BlueprintID: testBlueprintID, ManualEnvVarsFile: new("-")},
		},
		{
			name:    "sync ID instead of Blueprint ID",
			args:    []string{"exe-abcdefghij0123456789"},
			wantErr: `invalid Blueprint ID "exe-`,
		},
		{
			name:    "explicit empty manual env vars file",
			args:    []string{testBlueprintID, "--manual-env-vars-file", ""},
			wantErr: "--manual-env-vars-file must not be empty",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newBlueprintApproveCmd(nil)
			cmd.SetContext(command.SetFormatInContext(context.Background(), new(command.TEXT)))

			require.NoError(t, cmd.ParseFlags(tc.args))

			var input blueprint.ApproveInput
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

func TestBlueprintApproveRequiresOneBlueprintID(t *testing.T) {
	cmd := newBlueprintApproveCmd(nil)
	require.Error(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{testBlueprintID, testBlueprintID}))
}
