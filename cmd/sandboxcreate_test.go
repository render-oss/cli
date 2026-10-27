package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

func TestSandboxCreate_PlanPassesThroughToAPI(t *testing.T) {
	tests := []struct {
		name string
		plan string
	}{
		{name: "current plan", plan: "1c-2g"},
		{name: "deprecated plan", plan: "starter"},
		{name: "plan unknown to this CLI", plan: "32c-64g"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)

			args := []string{"sandboxes", "create", "--plan", tc.plan, "--output", "json"}
			_, err := executeSandboxCommand(t, server, args...)
			require.NoError(t, err)

			req, ok := server.LastRequest("POST", "/sandboxes")
			require.True(t, ok)
			body := testrequire.ParseJSONMap(t, string(req.Body))
			assert.Equal(t, tc.plan, body["plan"])
		})
	}
}

func TestSandboxPlanNamesOmitDeprecatedPlans(t *testing.T) {
	assert.Equal(t, []string{"1c-2g", "2c-4g", "4c-8g", "8c-16g"}, sandboxPlanNames())
}

func TestSandboxCreateInputValidateNetworkPolicy(t *testing.T) {
	cases := []struct {
		name             string
		networkPolicy    string
		allowEgressHTTPS []string
		errContains      string
		listsPolicies    bool
	}{
		{name: "empty network policy is allowed (server picks default)", networkPolicy: ""},
		{name: "allow-all", networkPolicy: "allow-all"},
		{name: "deny-all", networkPolicy: "deny-all"},
		{name: "allow-list with domains", networkPolicy: "allow-list", allowEgressHTTPS: []string{"api.example.com"}},
		{name: "domains alone imply allow-list", allowEgressHTTPS: []string{"api.example.com", "*.example.com"}},
		{name: "unknown policy", networkPolicy: "somewhat-open", errContains: `invalid network policy "somewhat-open"`, listsPolicies: true},
		{name: "constant-style name rejected", networkPolicy: "AllowAll", errContains: `invalid network policy "AllowAll"`, listsPolicies: true},
		{name: "allow-list without domains", networkPolicy: "allow-list", errContains: "--network-policy=allow-list requires at least one --allow-egress-https"},
		{name: "domains with allow-all", networkPolicy: "allow-all", allowEgressHTTPS: []string{"api.example.com"}, errContains: "--allow-egress-https requires --network-policy=allow-list, got allow-all"},
		{name: "domains with deny-all", networkPolicy: "deny-all", allowEgressHTTPS: []string{"api.example.com"}, errContains: "--allow-egress-https requires --network-policy=allow-list, got deny-all"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := SandboxCreateInput{NetworkPolicy: tc.networkPolicy, AllowEgressHTTPS: tc.allowEgressHTTPS}
			err := input.Validate(false)
			if tc.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
			if tc.listsPolicies {
				for _, name := range sandboxNetworkPolicyNames() {
					assert.Contains(t, err.Error(), name)
				}
			}
		})
	}
}

func TestSandboxCreateInputValidateTimeout(t *testing.T) {
	cases := []struct {
		name        string
		timeout     int
		errContains string
	}{
		{name: "zero means the server default", timeout: 0},
		{name: "one second", timeout: 1},
		{name: "the documented maximum", timeout: 86400},
		{name: "negative is rejected rather than silently defaulted", timeout: -1, errContains: "invalid timeout -1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := SandboxCreateInput{Timeout: tc.timeout}
			err := input.Validate(false)
			if tc.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestSandboxCreateInputValidateIdleTimeout(t *testing.T) {
	cases := []struct {
		name        string
		timeout     int
		errContains string
	}{
		{name: "zero disables idle suspension", timeout: 0},
		{name: "the enabled minimum", timeout: 300},
		{name: "the maximum", timeout: 86400},
		{name: "below the enabled minimum", timeout: 299, errContains: "invalid idle timeout 299"},
		{name: "above the maximum", timeout: 86401, errContains: "invalid idle timeout 86401"},
		{name: "negative", timeout: -1, errContains: "invalid idle timeout -1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := (&SandboxCreateInput{IdleTimeout: tc.timeout}).Validate(false)
			if tc.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestSandboxCreateNetworkPolicyNamesMatchSchema(t *testing.T) {
	names := sandboxNetworkPolicyNames()
	require.NotEmpty(t, names)
	for _, name := range names {
		assert.True(t, sandboxclient.SandboxNetworkPolicyType(name).Valid(), "policy %q should be valid per schema", name)
		input := SandboxCreateInput{NetworkPolicy: name}
		if name == string(sandboxclient.AllowList) {
			input.AllowEgressHTTPS = []string{"api.example.com"}
		}
		assert.NoError(t, input.Validate(false), "policy %q should be accepted", name)
	}
}

func TestSandboxCreateInputValidateEnv(t *testing.T) {
	cases := []struct {
		name        string
		env         []string
		errContains string
	}{
		{name: "no env is allowed", env: nil},
		{name: "single pair", env: []string{"FOO=bar"}},
		{name: "multiple pairs", env: []string{"FOO=bar", "BAZ=qux"}},
		{name: "empty value allowed", env: []string{"FOO="}},
		{name: "value may contain equals", env: []string{"DSN=postgres://u:p@h/db?sslmode=disable"}},
		{name: "missing equals", env: []string{"FOO"}, errContains: `invalid --env-var "FOO"`},
		{name: "whitespace is trimmed", env: []string{" FOO = bar "}},
		{name: "empty key", env: []string{"=bar"}, errContains: `invalid --env-var "=bar"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := SandboxCreateInput{EnvVars: tc.env}
			err := input.Validate(false)
			if tc.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
			assert.Contains(t, err.Error(), "KEY=VALUE")
		})
	}
}

func TestResolveSandboxSource(t *testing.T) {
	tests := []struct {
		name             string
		from             string
		fromSet          bool
		explicitID       string
		wantSnapshotID   string
		wantSnapshotName *string
	}{
		{name: "explicit snapshot id", explicitID: "snp-explicit", wantSnapshotID: "snp-explicit"},
		{name: "exact snapshot id", from: "snp-0123456789abcdefghij", fromSet: true, wantSnapshotID: "snp-0123456789abcdefghij"},
		{name: "name", from: "gold", fromSet: true, wantSnapshotName: commandStringPointer("gold")},
		{name: "empty value is a name", fromSet: true, wantSnapshotName: commandStringPointer("")},
		{name: "short snp prefix is a name", from: "snp-abc123", fromSet: true, wantSnapshotName: commandStringPointer("snp-abc123")},
		{name: "uppercase character is a name", from: "snp-0123456789abcdefghiJ", fromSet: true, wantSnapshotName: commandStringPointer("snp-0123456789abcdefghiJ")},
		{name: "w is outside id alphabet", from: "snp-0123456789abcdefghiw", fromSet: true, wantSnapshotName: commandStringPointer("snp-0123456789abcdefghiw")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshotID, snapshotName := resolveSandboxSource(tc.from, tc.fromSet, tc.explicitID)
			assert.Equal(t, tc.wantSnapshotID, snapshotID)
			assert.Equal(t, tc.wantSnapshotName, snapshotName)
		})
	}
}

func commandStringPointer(value string) *string {
	return &value
}

func TestResolveSandboxEnv(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.env")
	override := filepath.Join(dir, "override.env")
	require.NoError(t, os.WriteFile(base, []byte("FOO=from-base\nBASE_ONLY=yes\n"), 0o600))
	require.NoError(t, os.WriteFile(override, []byte("FOO=from-override\n"), 0o600))

	cases := []struct {
		name        string
		files       []string
		pairs       []string
		want        map[string]string
		errContains string
	}{
		{name: "nothing set returns nil", want: nil},
		{name: "inline only", pairs: []string{"FOO=bar"}, want: map[string]string{"FOO": "bar"}},
		{name: "file only", files: []string{base}, want: map[string]string{"FOO": "from-base", "BASE_ONLY": "yes"}},
		{name: "later file overrides earlier", files: []string{base, override}, want: map[string]string{"FOO": "from-override", "BASE_ONLY": "yes"}},
		{name: "inline overrides file", files: []string{base}, pairs: []string{"FOO=inline"}, want: map[string]string{"FOO": "inline", "BASE_ONLY": "yes"}},
		{name: "missing file errors", files: []string{filepath.Join(dir, "nope.env")}, errContains: "nope.env"},
		{name: "invalid inline pair errors", pairs: []string{"FOO"}, errContains: `invalid --env-var "FOO"`},
		{name: "whitespace trimmed like workflows", pairs: []string{" FOO = bar "}, want: map[string]string{"FOO": "bar"}},
		{name: "value may contain equals", pairs: []string{"DSN=a=b=c"}, want: map[string]string{"DSN": "a=b=c"}},
		{name: "empty value allowed", pairs: []string{"EMPTY="}, want: map[string]string{"EMPTY": ""}},
		{name: "later inline duplicate wins", pairs: []string{"FOO=first", "FOO=second"}, want: map[string]string{"FOO": "second"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSandboxEnv(tc.files, tc.pairs)
			if tc.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

var sandboxCreateActiveWorkspaceID = testids.WorkspaceID("create")

func newSandboxCreateServer(t *testing.T) *renderapi.Server {
	t.Helper()
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxCreateActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", sandboxCreateActiveWorkspaceID)
	server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxclient.SandboxGroup{OwnerId: sandboxCreateActiveWorkspaceID}))
	return server
}

func sandboxCreateDefaultGroup(t *testing.T, server *renderapi.Server) *sandboxclient.SandboxGroup {
	t.Helper()
	for _, g := range server.SandboxGroups.Instances {
		if g.OwnerId == sandboxCreateActiveWorkspaceID {
			return g
		}
	}
	t.Fatal("no sandbox group seeded for the active workspace")
	return nil
}

func TestSandboxCreate_SnapshotIDSent(t *testing.T) {
	server := newSandboxCreateServer(t)
	group := sandboxCreateDefaultGroup(t, server)
	snap := server.SandboxSnapshots.Add(renderapi.NewSandboxSnapshot(sandboxclient.SandboxSnapshot{SandboxGroupId: group.Id}))

	_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--snapshot-id", snap.Id, "--output", "json")
	require.NoError(t, err)

	req, ok := server.LastRequest("POST", "/sandboxes")
	require.True(t, ok, "expected a create request")
	body := testrequire.ParseJSONMap(t, string(req.Body))
	assert.Equal(t, snap.Id, body["snapshotId"])
}

func TestSandboxCreate_FromDispatchesSnapshotSource(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		snapshot  func(group *sandboxclient.SandboxGroup) sandboxclient.SandboxSnapshot
		wantField string
		wantValue string
		wantError string
	}{
		{
			name: "snapshot id",
			from: "snp-0123456789abcdefghij",
			snapshot: func(group *sandboxclient.SandboxGroup) sandboxclient.SandboxSnapshot {
				return sandboxclient.SandboxSnapshot{Id: "snp-0123456789abcdefghij", SandboxGroupId: group.Id}
			},
			wantField: "snapshotId",
			wantValue: "snp-0123456789abcdefghij",
		},
		{
			name: "snapshot name",
			from: "gold",
			snapshot: func(group *sandboxclient.SandboxGroup) sandboxclient.SandboxSnapshot {
				name := "gold"
				return sandboxclient.SandboxSnapshot{SandboxGroupId: group.Id, Name: &name}
			},
			wantField: "snapshotName",
			wantValue: "gold",
		},
		{
			name: "empty snapshot name is sent for server validation",
			from: "",
			snapshot: func(group *sandboxclient.SandboxGroup) sandboxclient.SandboxSnapshot {
				name := ""
				return sandboxclient.SandboxSnapshot{SandboxGroupId: group.Id, Name: &name}
			},
			wantField: "snapshotName",
			wantValue: "",
			wantError: "400 (invalid_snapshot_name)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			group := sandboxCreateDefaultGroup(t, server)
			server.SandboxSnapshots.Add(renderapi.NewSandboxSnapshot(tc.snapshot(group)))

			_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--from", tc.from, "--output", "json")
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}

			req, ok := server.LastRequest("POST", "/sandboxes")
			require.True(t, ok, "expected a create request")
			body := testrequire.ParseJSONMap(t, string(req.Body))
			assert.Equal(t, tc.wantValue, body[tc.wantField])
			if tc.wantField == "snapshotId" {
				assert.NotContains(t, body, "snapshotName")
			} else {
				assert.NotContains(t, body, "snapshotId")
			}
		})
	}
}

func TestSandboxCreate_FromAndSnapshotIDAreMutuallyExclusive(t *testing.T) {
	server := newSandboxCreateServer(t)

	_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--from", "gold", "--snapshot-id", "snp-explicit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "if any flags in the group")
	assert.False(t, server.HasRequest("POST", "/sandboxes"))
}

func TestSandboxCreate_WithoutSnapshotIDOmitsIt(t *testing.T) {
	server := newSandboxCreateServer(t)

	_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--output", "json")
	require.NoError(t, err)

	req, ok := server.LastRequest("POST", "/sandboxes")
	require.True(t, ok, "expected a create request")
	body := testrequire.ParseJSONMap(t, string(req.Body))
	assert.NotContains(t, body, "snapshotId")
	assert.NotContains(t, body, "snapshotName")
}

func TestSandboxCreate_IdleTimeout(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantPresent bool
		want        float64
	}{
		{name: "omitted uses the server default"},
		{name: "custom timeout", args: []string{"--idle-timeout", "1800"}, wantPresent: true, want: 1800},
		{name: "zero disables", args: []string{"--idle-timeout", "0"}, wantPresent: true, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			args := append([]string{"sandboxes", "create", "--output", "json"}, tc.args...)
			_, err := executeSandboxCommand(t, server, args...)
			require.NoError(t, err)

			req, ok := server.LastRequest("POST", "/sandboxes")
			require.True(t, ok)
			body := testrequire.ParseJSONMap(t, string(req.Body))
			got, present := body["idleTimeoutSeconds"]
			assert.Equal(t, tc.wantPresent, present)
			if tc.wantPresent {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSandboxCreate_SnapshotIDErrors(t *testing.T) {
	tests := []struct {
		name        string
		seed        func(server *renderapi.Server, group *sandboxclient.SandboxGroup) string
		extraArgs   []string
		errContains []string
	}{
		{
			name: "unknown snapshot is not found",
			seed: func(*renderapi.Server, *sandboxclient.SandboxGroup) string {
				return testids.SandboxSnapshotID("missing")
			},
			errContains: []string{"404 (snapshot_not_found)", "snapshot not found"},
		},
		{
			name: "snapshot still being created surfaces the error code",
			seed: func(server *renderapi.Server, group *sandboxclient.SandboxGroup) string {
				return server.SandboxSnapshots.Add(renderapi.NewSandboxSnapshot(sandboxclient.SandboxSnapshot{SandboxGroupId: group.Id, Status: sandboxclient.SandboxSnapshotStatusCreating})).Id
			},
			errContains: []string{"409 (snapshot_not_available)"},
		},
		{
			name: "runtime snapshot with a different plan surfaces the error code",
			seed: func(server *renderapi.Server, group *sandboxclient.SandboxGroup) string {
				return server.SandboxSnapshots.Add(renderapi.NewSandboxSnapshot(sandboxclient.SandboxSnapshot{SandboxGroupId: group.Id, Kind: sandboxclient.Runtime, Plan: sandboxclient.Starter})).Id
			},
			extraArgs:   []string{"--plan", "pro"},
			errContains: []string{"409 (snapshot_plan_mismatch)"},
		},
		{
			name: "snapshot owned by another workspace is not found",
			seed: func(server *renderapi.Server, _ *sandboxclient.SandboxGroup) string {
				other := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxclient.SandboxGroup{OwnerId: testids.WorkspaceID("other")}))
				return server.SandboxSnapshots.Add(renderapi.NewSandboxSnapshot(sandboxclient.SandboxSnapshot{SandboxGroupId: other.Id})).Id
			},
			errContains: []string{"404 (snapshot_not_found)"},
		},
		{
			name: "snapshot in another group of the workspace is not available",
			seed: func(server *renderapi.Server, _ *sandboxclient.SandboxGroup) string {
				other := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxclient.SandboxGroup{OwnerId: sandboxCreateActiveWorkspaceID, Name: "Secondary"}))
				return server.SandboxSnapshots.Add(renderapi.NewSandboxSnapshot(sandboxclient.SandboxSnapshot{SandboxGroupId: other.Id})).Id
			},
			errContains: []string{"409 (snapshot_not_available)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			snapshotID := tc.seed(server, sandboxCreateDefaultGroup(t, server))

			_, err := executeSandboxCommand(t, server, append([]string{"sandboxes", "create", "--snapshot-id", snapshotID, "--output", "json"}, tc.extraArgs...)...)
			require.Error(t, err)
			for _, want := range tc.errContains {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, server.Sandboxes.Instances)
		})
	}
}

func TestSandboxCreate_NetworkPolicyRequest(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPolicy map[string]any
	}{
		{
			name:       "deny-all is sent as type",
			args:       []string{"--network-policy", "deny-all"},
			wantPolicy: map[string]any{"type": "deny-all"},
		},
		{
			name: "allow-egress-https implies allow-list and sends https rules",
			args: []string{"--allow-egress-https", "api.example.com", "--allow-egress-https", "*.example.com"},
			wantPolicy: map[string]any{
				"type": "allow-list",
				"rules": []any{
					map[string]any{"domain": "api.example.com", "protocol": "https"},
					map[string]any{"domain": "*.example.com", "protocol": "https"},
				},
			},
		},
		{
			name: "explicit allow-list with a domain",
			args: []string{"--network-policy", "allow-list", "--allow-egress-https", "api.example.com"},
			wantPolicy: map[string]any{
				"type":  "allow-list",
				"rules": []any{map[string]any{"domain": "api.example.com", "protocol": "https"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)

			args := append([]string{"sandboxes", "create", "--output", "json"}, tc.args...)
			out, err := executeSandboxCommand(t, server, args...)
			require.NoError(t, err)

			req, ok := server.LastRequest("POST", "/sandboxes")
			require.True(t, ok, "expected a create request")
			body := testrequire.ParseJSONMap(t, string(req.Body))
			assert.Equal(t, tc.wantPolicy, body["networkPolicy"])

			created := testrequire.ParseJSONMap(t, out.Stdout)
			policy, ok := created["networkPolicy"].(map[string]any)
			require.True(t, ok, "expected networkPolicy in output, got %v", created["networkPolicy"])
			assert.Equal(t, tc.wantPolicy["type"], policy["type"])
			assert.Equal(t, tc.wantPolicy["rules"], policy["rules"])
		})
	}
}

func TestSandboxCreate_AllowEgressHTTPSRejectedWithOtherPolicy(t *testing.T) {
	server := newSandboxCreateServer(t)

	_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--network-policy", "deny-all", "--allow-egress-https", "api.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--allow-egress-https requires --network-policy=allow-list")
	assert.False(t, server.HasRequest("POST", "/sandboxes"))
}

func TestSandboxCreate_BuildDispatchesSource(t *testing.T) {
	tests := []struct {
		name      string
		build     string
		wantField string
		wantValue string
		wantError string
	}{
		{name: "build id", build: "sbb-0123456789abcdefghij", wantField: "buildId", wantValue: "sbb-0123456789abcdefghij"},
		{name: "build name", build: "python", wantField: "buildName", wantValue: "python"},
		{name: "empty build name is sent for server validation", build: "", wantField: "buildName", wantValue: "", wantError: "400: invalid buildName"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			group := sandboxCreateDefaultGroup(t, server)
			server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{Id: "sbb-0123456789abcdefghij", SandboxGroupId: group.Id}))
			server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{SandboxGroupId: group.Id, Name: "python"}))

			_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--build", tc.build, "--output", "json")
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}

			req, ok := server.LastRequest("POST", "/sandboxes")
			require.True(t, ok, "expected a create request")
			body := testrequire.ParseJSONMap(t, string(req.Body))
			assert.Equal(t, tc.wantValue, body[tc.wantField])
			if tc.wantField == "buildId" {
				assert.NotContains(t, body, "buildName")
			} else {
				assert.NotContains(t, body, "buildId")
			}
			assert.NotContains(t, body, "snapshotId")
			assert.NotContains(t, body, "snapshotName")
		})
	}
}

func TestSandboxCreate_BuildNamePrefersNewestSucceededBuild(t *testing.T) {
	server := newSandboxCreateServer(t)
	group := sandboxCreateDefaultGroup(t, server)
	server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{SandboxGroupId: group.Id, Name: "python", CreatedAt: time.Now().Add(-time.Hour)}))
	server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{SandboxGroupId: group.Id, Name: "python", Status: sandboxclient.SandboxBuildStatusRunning}))

	_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--build", "python", "--output", "json")
	require.NoError(t, err)
	assert.Len(t, server.Sandboxes.Instances, 1)
}

func TestSandboxCreate_BuildIsMutuallyExclusiveWithSnapshotFlags(t *testing.T) {
	for _, extra := range [][]string{{"--from", "gold"}, {"--snapshot-id", "snp-explicit"}} {
		t.Run(extra[0], func(t *testing.T) {
			server := newSandboxCreateServer(t)

			_, err := executeSandboxCommand(t, server, append([]string{"sandboxes", "create", "--build", "python"}, extra...)...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "none of the others can be")
			assert.False(t, server.HasRequest("POST", "/sandboxes"))
		})
	}
}

func TestSandboxCreateInputValidateBuild(t *testing.T) {
	assert.NoError(t, (&SandboxCreateInput{Build: "python"}).Validate(false))
	assert.ErrorContains(t, (&SandboxCreateInput{Build: "python", From: "gold"}).Validate(false), "--build cannot be used with --from or --snapshot-id")
	assert.ErrorContains(t, (&SandboxCreateInput{Build: "python", SnapshotID: "snp-abc"}).Validate(false), "--build cannot be used with --from or --snapshot-id")
}

func TestSandboxCreate_BuildErrors(t *testing.T) {
	tests := []struct {
		name        string
		seed        func(server *renderapi.Server, group *sandboxclient.SandboxGroup) string
		errContains []string
	}{
		{
			name: "unknown build is not found",
			seed: func(*renderapi.Server, *sandboxclient.SandboxGroup) string {
				return testids.SandboxBuildID("missing")
			},
			errContains: []string{"404", "not found: build"},
		},
		{
			name: "build still running has not succeeded",
			seed: func(server *renderapi.Server, group *sandboxclient.SandboxGroup) string {
				return server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{SandboxGroupId: group.Id, Status: sandboxclient.SandboxBuildStatusRunning})).Id
			},
			errContains: []string{"409", "build has not succeeded"},
		},
		{
			name: "failed build has not succeeded",
			seed: func(server *renderapi.Server, group *sandboxclient.SandboxGroup) string {
				return server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{SandboxGroupId: group.Id, Status: sandboxclient.SandboxBuildStatusFailed})).Id
			},
			errContains: []string{"409", "build has not succeeded"},
		},
		{
			name: "build owned by another workspace is not found",
			seed: func(server *renderapi.Server, _ *sandboxclient.SandboxGroup) string {
				other := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxclient.SandboxGroup{OwnerId: testids.WorkspaceID("other")}))
				return server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{SandboxGroupId: other.Id})).Id
			},
			errContains: []string{"404", "not found: build"},
		},
		{
			name: "unknown build name is not found",
			seed: func(*renderapi.Server, *sandboxclient.SandboxGroup) string {
				return "missing"
			},
			errContains: []string{"404", "not found: build"},
		},
		{
			name: "name with no succeeded build has not succeeded",
			seed: func(server *renderapi.Server, group *sandboxclient.SandboxGroup) string {
				server.SandboxBuilds.Add(renderapi.NewSandboxBuild(sandboxclient.SandboxBuild{SandboxGroupId: group.Id, Name: "broken", Status: sandboxclient.SandboxBuildStatusFailed}))
				return "broken"
			},
			errContains: []string{"409", "build has not succeeded"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			build := tc.seed(server, sandboxCreateDefaultGroup(t, server))

			_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--build", build, "--output", "json")
			require.Error(t, err)
			for _, want := range tc.errContains {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, server.Sandboxes.Instances)
		})
	}
}

func TestSandboxCreate_Wait(t *testing.T) {
	server := newSandboxCreateServer(t)

	result, err := executeSandboxCommand(t, server, "sandboxes", "create", "--wait", "--output", "json")
	require.NoError(t, err)

	created := server.Sandboxes.Only(t)
	assert.Equal(t, "running", testrequire.ParseJSONMap(t, result.Stdout)["status"])
	assert.True(t, server.HasRequest("GET", "/sandboxes/"+created.Id), "--wait polls the new sandbox")
}

func TestSandboxCreate_NoWaitDoesNotPoll(t *testing.T) {
	server := newSandboxCreateServer(t)

	_, err := executeSandboxCommand(t, server, "sandboxes", "create", "--output", "json")
	require.NoError(t, err)

	created := server.Sandboxes.Only(t)
	assert.False(t, server.HasRequest("GET", "/sandboxes/"+created.Id))
}

func TestSandboxCreate_WaitFailurePrintsSandbox(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		t.Run(format, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			server.Sandboxes.RespondToGetWith(http.StatusInternalServerError)

			result, err := executeSandboxCommand(t, server, "sandboxes", "create", "--wait", "--output", format)
			require.Error(t, err)

			created := server.Sandboxes.Only(t)
			assert.Contains(t, result.Stdout, created.Id, "a failed wait still prints the sandbox ID")
		})
	}
}

func TestSandboxCreate_GroupSelection(t *testing.T) {
	for _, groupID := range []string{"", "sbg-cph1rs3idesc73a2b2mg"} {
		t.Run(groupID, func(t *testing.T) {
			server := newSandboxCreateServer(t)
			if groupID != "" {
				server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxclient.SandboxGroup{Id: groupID, OwnerId: sandboxCreateActiveWorkspaceID}))
			}
			args := []string{"sandboxes", "create", "--output", "json"}
			if groupID != "" {
				args = append(args, "--group", groupID)
			}
			_, err := executeSandboxCommand(t, server, args...)
			require.NoError(t, err)
			req, ok := server.LastRequest("POST", "/sandboxes")
			require.True(t, ok)
			body := testrequire.ParseJSONMap(t, string(req.Body))
			if groupID == "" {
				assert.NotContains(t, body, "sandboxGroupId")
			} else {
				assert.Equal(t, groupID, body["sandboxGroupId"])
			}
		})
	}
}
