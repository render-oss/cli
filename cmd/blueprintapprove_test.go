package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
)

var testBlueprintID = testids.BlueprintID("approve")

func TestBlueprintApproveIsHidden(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "--help"})

	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, output.Stdout, "approve")
	require.Empty(t, server.Requests)
}

// approveSecret stands in for a manually-supplied value that must never be
// printed.
const approveSecret = "s3cr3t-approve-value"

const approveManualEnvVarsJSON = `[{"serviceType":"web","serviceName":"app","key":"API_KEY","value":"` + approveSecret + `"}]`

func newBlueprintApproveFixture(t *testing.T) (server *renderapi.Server, defaultFile string) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	server = renderapi.NewServer(t)
	server.Blueprints.Add(renderapi.NewBlueprint(bptypes.Blueprint{Id: testBlueprintID}))
	return server, filepath.Join(configDir, "state", "manual-env-vars", testBlueprintID+".json")
}

func requireNoSecret(t *testing.T, output CommandResult) {
	t.Helper()
	require.NotContains(t, output.Stdout, approveSecret)
	require.NotContains(t, output.Stderr, approveSecret)
}

func TestBlueprintApproveTextOutput(t *testing.T) {
	server, _ := newBlueprintApproveFixture(t)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", testBlueprintID})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	testassert.ContainsInOrder(t, output.Stdout, "Approved", "Next:")
	require.Empty(t, output.Stderr)

	require.Len(t, server.Requests, 1)
	require.Equal(t, "/blueprints/"+testBlueprintID+"/approve", server.Requests[0].URI)
}

func TestBlueprintApproveJSONOutput(t *testing.T) {
	server, _ := newBlueprintApproveFixture(t)
	sync := bptypes.ApprovedSync{
		Id:           testids.SyncID("running"),
		State:        bptypes.SyncStateRunning,
		Commit:       bptypes.CommitRef{Id: "abc123"},
		DashboardUrl: "https://dashboard.render.com/sync",
	}
	server.Blueprints.RespondWithBodyForApprove(sync)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", testBlueprintID, "--output", "json"})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	var out blueprint.ApproveOut
	require.NoError(t, json.Unmarshal([]byte(output.Stdout), &out))
	require.Equal(t, &sync, out.Data)

	require.NotNil(t, out.Meta.Next)
	require.Contains(t, *out.Meta.Next, "render services")
}

// TestBlueprintApproveManualEnvVarsAreSentInRequestCorrectly covers each
// place --manual-env-vars-file can read values from: the default file when the
// flag is omitted, an explicit file, or stdin. Only the default file is deleted
// after approval.
func TestBlueprintApproveManualEnvVarsAreSentInRequestCorrectly(t *testing.T) {
	for _, tc := range []struct {
		name string
		// defaultFileContents, when set, is written to the default manual env var
		// file.
		defaultFileContents string
		// explicitFileContents, when set, is written to a file passed with
		// --manual-env-vars-file.
		explicitFileContents string
		// stdin, when set, is piped in with --manual-env-vars-file -.
		stdin string
		// wantRequestBody is the body the CLI sends to the approve endpoint.
		wantRequestBody        string
		wantDefaultFilePresent bool
	}{
		{
			name:            "no file sends no values",
			wantRequestBody: `{}`,
		},
		{
			name:                "default file is sent, then deleted",
			defaultFileContents: approveManualEnvVarsJSON,
			wantRequestBody:     `{"manualEnvVars":` + approveManualEnvVarsJSON + `}`,
		},
		{
			name:                   "explicit file is sent and kept, ignoring the default file",
			defaultFileContents:    `[]`,
			explicitFileContents:   approveManualEnvVarsJSON,
			wantRequestBody:        `{"manualEnvVars":` + approveManualEnvVarsJSON + `}`,
			wantDefaultFilePresent: true,
		},
		{
			name:            "stdin is sent",
			stdin:           approveManualEnvVarsJSON,
			wantRequestBody: `{"manualEnvVars":` + approveManualEnvVarsJSON + `}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, defaultFile := newBlueprintApproveFixture(t)
			args := []string{"ea", "blueprints", "approve", testBlueprintID}
			if tc.defaultFileContents != "" {
				require.NoError(t, files.Write(defaultFile, []byte(tc.defaultFileContents)))
			}
			explicitFile := filepath.Join(t.TempDir(), "values.json")
			if tc.explicitFileContents != "" {
				require.NoError(t, files.Write(explicitFile, []byte(tc.explicitFileContents)))
				args = append(args, "--manual-env-vars-file", explicitFile)
			}
			if tc.stdin != "" {
				args = append(args, "--manual-env-vars-file", "-")
			}

			result, output := executeBlueprintCommand(t, server, args, withStdin(tc.stdin))

			require.Equal(t, 0, result.ExitCode, output.Stderr)
			require.Empty(t, output.Stderr)
			requireNoSecret(t, output)

			require.Len(t, server.Requests, 1)
			require.JSONEq(t, tc.wantRequestBody, string(server.Requests[0].Body))

			_, err := os.Stat(defaultFile)
			require.Equal(t, tc.wantDefaultFilePresent, err == nil, "default file present")
			if tc.explicitFileContents != "" {
				require.FileExists(t, explicitFile)
			}
		})
	}
}

func TestBlueprintApproveExpandsTildeInManualEnvVarsFile(t *testing.T) {
	server, _ := newBlueprintApproveFixture(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	require.NoError(t, files.Write(filepath.Join(home, "values.json"), []byte(approveManualEnvVarsJSON)))

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", testBlueprintID, "--manual-env-vars-file", "~/values.json"})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	require.Len(t, server.Requests, 1)
	require.JSONEq(t, `{"manualEnvVars":`+approveManualEnvVarsJSON+`}`, string(server.Requests[0].Body))
}

func TestBlueprintApproveDoesNotDeleteManualEnvVarFileAfterHTTPError(t *testing.T) {
	server, defaultFile := newBlueprintApproveFixture(t)
	require.NoError(t, files.Write(defaultFile, []byte(approveManualEnvVarsJSON)))
	message := "Missing values for 1 sync: false environment variables: worker/app-worker STRIPE_SECRET. Nothing was approved."
	server.Blueprints.RespondWithErrorForApprove(http.StatusBadRequest, message)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", testBlueprintID, "--output", "json"})

	require.Equal(t, 1, result.ExitCode, output.Stderr)
	require.Empty(t, output.Stdout)
	require.Contains(t, output.Stderr, message)
	requireNoSecret(t, output)
	require.FileExists(t, defaultFile)
}

func TestBlueprintApproveRejectsMalformedManualEnvVars(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "unquoted value",
			input:   `[{"key":"API_KEY","value":` + approveSecret + `}]`,
			wantErr: "line 1, column 27: invalid JSON",
		},
		{
			name:    "unquoted value on a later line",
			input:   "[\n  {\n    \"key\": \"API_KEY\",\n    \"value\": " + approveSecret + "\n  }\n]\n",
			wantErr: "line 4, column 14: invalid JSON",
		},
		{
			name:    "value is not a string",
			input:   "[\n  {\n    \"key\": \"API_KEY\",\n    \"value\": 12345\n  }\n]\n",
			wantErr: "line 4: [0].value must be a string",
		},
		{
			name:    ".env format",
			input:   "API_KEY=" + approveSecret + "\n",
			wantErr: "line 1, column 1: invalid JSON",
		},
		{
			name:    "array of numbers",
			input:   `[1]`,
			wantErr: "expected a JSON array of objects",
		},
		{
			name:    "null instead of array",
			input:   `null`,
			wantErr: "expected a JSON array of objects",
		},
		{
			name:    "object instead of array",
			input:   `{"key":"API_KEY","value":"` + approveSecret + `"}`,
			wantErr: "expected a JSON array of objects",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newBlueprintApproveFixture(t)

			result, output := executeBlueprintCommand(t, server,
				[]string{"ea", "blueprints", "approve", testBlueprintID, "--manual-env-vars-file", "-"}, withStdin(tc.input))

			require.Equal(t, 1, result.ExitCode, output.Stderr)
			require.Contains(t, output.Stderr, tc.wantErr)
			requireNoSecret(t, output)
			require.NotContains(t, output.Stderr, "invalid character", "raw encoding/json errors quote input bytes, such as a secret's first character")
			require.Empty(t, server.Requests)
		})
	}
}

func TestBlueprintApproveReportsMissingExplicitManualEnvVarFile(t *testing.T) {
	server, _ := newBlueprintApproveFixture(t)
	missing := filepath.Join(t.TempDir(), "missing.json")

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", testBlueprintID, "--manual-env-vars-file", missing})

	require.Equal(t, 1, result.ExitCode, output.Stderr)
	require.Contains(t, output.Stderr, missing)
	require.Empty(t, server.Requests)
}

func TestBlueprintApproveReportsMalformedExplicitManualEnvVarFile(t *testing.T) {
	server, _ := newBlueprintApproveFixture(t)
	explicit := filepath.Join(t.TempDir(), "values.json")
	require.NoError(t, files.Write(explicit, []byte("API_KEY="+approveSecret+"\n")))

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", testBlueprintID, "--manual-env-vars-file", explicit})

	require.Equal(t, 1, result.ExitCode, output.Stderr)
	testassert.ContainsInOrder(t, output.Stderr, explicit, "line 1, column 1: invalid JSON")
	requireNoSecret(t, output)
	require.Empty(t, server.Requests)
	require.FileExists(t, explicit)
}

func TestBlueprintApproveWarnsWhenDefaultFileCannotBeDeleted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not prevent deletion on Windows")
	}
	server, defaultFile := newBlueprintApproveFixture(t)
	require.NoError(t, files.Write(defaultFile, []byte(approveManualEnvVarsJSON)))
	dir := filepath.Dir(defaultFile)
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", testBlueprintID})

	require.Equal(t, 0, result.ExitCode, output.Stderr)
	require.Contains(t, output.Stdout, "Approved")
	testassert.ContainsInOrder(t, output.Stderr, "Warning:", defaultFile, "Delete it yourself")
	requireNoSecret(t, output)
	require.FileExists(t, defaultFile)
}

func TestBlueprintApproveHelp(t *testing.T) {
	server := renderapi.NewServer(t)

	result, output := executeBlueprintCommand(t, server, []string{"ea", "blueprints", "approve", "--help"})

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
