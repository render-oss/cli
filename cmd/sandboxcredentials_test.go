package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/sandboxcredential"
)

const sandboxCredentialTestID = "inference-key"

type sandboxCredentialsHarness struct {
	t      *testing.T
	server *renderapi.Server
	group  string
}

func newSandboxCredentialsHarness(t *testing.T) *sandboxCredentialsHarness {
	t.Helper()
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: sandboxGroupsActiveWorkspaceID}))
	t.Setenv("RENDER_WORKSPACE", sandboxGroupsActiveWorkspaceID)
	return &sandboxCredentialsHarness{t: t, server: server, group: seedSandboxGroup(server, "Default").Id}
}

func (h *sandboxCredentialsHarness) execute(args ...string) (CommandResult, error) {
	h.t.Helper()
	return h.executeWithStdin("", args...)
}

func (h *sandboxCredentialsHarness) executeWithStdin(stdin string, args ...string) (CommandResult, error) {
	h.t.Helper()
	fullArgs := append([]string{"ea", "sandbox-groups", "credentials"}, args...)
	return executeSandboxGroupsCommandWithStdin(h.t, h.server, stdin, fullArgs...)
}

func TestSandboxCredentials_OnlyUnderEarlyAccess(t *testing.T) {
	// arrange
	root := newRootCmd()
	ea := newEarlyAccessCmd()
	root.AddCommand(ea)
	deps := dependencies.New(nil)
	setupSandboxGroupsCommands(root, deps)
	setupEarlyAccessSandboxGroupsCommands(ea, deps)

	// act
	gaCmd, _, gaErr := root.Find([]string{"sandbox-groups", "credentials", "list"})
	eaCmd, _, eaErr := root.Find([]string{"ea", "sandbox-groups", "credentials", "list"})

	// assert
	require.NoError(t, gaErr)
	require.NoError(t, eaErr)
	assert.Equal(t, []string{"render sandbox-groups", "render ea sandbox-groups credentials list"},
		[]string{gaCmd.CommandPath(), eaCmd.CommandPath()})
}

func (h *sandboxCredentialsHarness) credentialPath(id string) string {
	return "/sandbox-groups/" + h.group + "/credentials/" + id
}

func sandboxCredentialFixture() *renderapi.SandboxCredentialState {
	return &renderapi.SandboxCredentialState{
		Value: "fixture-secret",
		Credential: sandboxesclient.SandboxCredential{
			Id:      sandboxCredentialTestID,
			Domain:  "api.example.com",
			Enabled: true,
			Methods: []string{},
			Paths:   []string{},
			Injection: sandboxesclient.SandboxCredentialInjection{
				Type:     sandboxesclient.Header,
				Name:     pointers.From("Authorization"),
				Template: pointers.From("Bearer ${SECRET}"),
			},
			Version: &sandboxesclient.SandboxCredentialVersion{
				Id:          1,
				Fingerprint: strings.Repeat("a", 64),
				UpdatedAt:   time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC),
				UpdatedBy:   "usr-credential-test",
			},
		},
	}
}

// Seed two retained configurations without running create or update commands.
func sandboxCredentialHistoryFixture() *renderapi.SandboxCredentialState {
	fixture := sandboxCredentialFixture()
	original := renderapi.SandboxCredentialRevision{Credential: fixture.Credential, Value: fixture.Value}
	fixture.Value = "rotated-secret"
	fixture.Credential.Enabled = false
	fixture.Credential.Version = &sandboxesclient.SandboxCredentialVersion{
		Id:          2,
		Fingerprint: strings.Repeat("b", 64),
		UpdatedAt:   original.Credential.Version.UpdatedAt.Add(time.Minute),
		UpdatedBy:   "usr-credential-test",
	}
	fixture.History = []renderapi.SandboxCredentialRevision{
		original,
		{Credential: fixture.Credential, Value: fixture.Value},
	}
	return fixture
}

func (h *sandboxCredentialsHarness) seedCredential(fixture *renderapi.SandboxCredentialState) {
	h.t.Helper()
	fixture.GroupID = h.group
	if len(fixture.History) == 0 {
		fixture.History = []renderapi.SandboxCredentialRevision{{Credential: fixture.Credential, Value: fixture.Value}}
	}
	h.server.SandboxCredentials.Add(fixture)
}

func sandboxCredentialCreateArgs(id string) []string {
	return []string{"create", id, "--domain", "api.example.com", "--injection-type", "header", "--injection-name", "Authorization", "--output", "json"}
}

func TestSandboxCredentialsCreate_SecretSources(t *testing.T) {
	const secret = "credential-secret-canary-é-\uFFFD\n"
	path := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(path, []byte(secret), 0600))
	tests := []struct {
		name  string
		args  []string
		stdin string
	}{
		{name: "flag", args: []string{"--value", secret}},
		{name: "file", args: []string{"--value-file", path}},
		{name: "stdin", args: []string{"--value-file", "-"}, stdin: secret},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxCredentialsHarness(t)

			result, err := h.executeWithStdin(tc.stdin, append(sandboxCredentialCreateArgs(sandboxCredentialTestID), tc.args...)...)
			require.NoError(t, err)

			req, ok := h.server.LastRequest("POST", "/sandbox-groups/"+h.group+"/credentials")
			require.True(t, ok)
			body := testrequire.ParseJSONMap(t, string(req.Body))
			assert.Equal(t, secret, body["value"], "send the complete value, including its trailing newline")
			assert.NotContains(t, result.Stdout+result.Stderr, strings.TrimSuffix(secret, "\n"))
			assert.NotContains(t, testrequire.ParseJSONMap(t, result.Stdout), "value")
		})
	}
}

func TestSandboxCredentials_RejectsInvalidUTF8(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, source := range []string{"flag", "file", "stdin"} {
			for _, invalid := range []struct {
				name  string
				value string
			}{
				{name: "invalid byte", value: "secret-canary-\xff"},
				{name: "truncated sequence", value: "secret-canary-\xe2\x82"},
			} {
				t.Run(operation+"/"+source+"/"+invalid.name, func(t *testing.T) {
					h := newSandboxCredentialsHarness(t)
					args := sandboxCredentialCreateArgs(sandboxCredentialTestID)
					if operation == "update" {
						fixture := sandboxCredentialFixture()
						h.seedCredential(fixture)
						args = []string{"update", fixture.Credential.Id, "--output", "json"}
					}
					var stdin string
					switch source {
					case "flag":
						args = append(args, "--value", invalid.value)
					case "file":
						path := filepath.Join(t.TempDir(), "secret")
						require.NoError(t, os.WriteFile(path, []byte(invalid.value), 0600))
						args = append(args, "--value-file", path)
					case "stdin":
						args = append(args, "--value-file", "-")
						stdin = invalid.value
					}

					result, err := h.executeWithStdin(stdin, args...)

					require.ErrorContains(t, err, "credential value must be valid UTF-8")
					assert.Empty(t, h.server.Requests, "reject before constructing an API request")
					assert.NotContains(t, result.Stdout+result.Stderr, "secret-canary")
				})
			}
		}
	}
}

func TestSandboxCredentialsCreate_Options(t *testing.T) {
	h := newSandboxCredentialsHarness(t)
	args := append(sandboxCredentialCreateArgs(sandboxCredentialTestID),
		"--value", "test-secret", "--injection-template", "Bearer ${SECRET}",
		"--methods", "POST,GET", "--paths", "/v1/messages,/v1/models", "--enabled=false")

	_, err := h.execute(args...)
	require.NoError(t, err)

	req, ok := h.server.LastRequest("POST", "/sandbox-groups/"+h.group+"/credentials")
	require.True(t, ok)
	assert.JSONEq(t, `{
  "id":"inference-key", "domain":"api.example.com", "value":"test-secret", "enabled":false,
  "methods":["POST","GET"], "paths":["/v1/messages","/v1/models"],
  "injection":{"type":"header","name":"Authorization","template":"Bearer ${SECRET}"}
 }`, string(req.Body))
}

func TestSandboxCredentialsUpdate_PreservesOmittedFieldsAndClearsFilters(t *testing.T) {
	h := newSandboxCredentialsHarness(t)
	fixture := sandboxCredentialFixture()
	fixture.Credential.Methods = []string{"POST"}
	fixture.Credential.Paths = []string{"/v1"}
	h.seedCredential(fixture)

	_, err := h.execute("update", fixture.Credential.Id, "--enabled=false", "--methods=", "--paths=", "--output", "json")
	require.NoError(t, err)

	req, ok := h.server.LastRequest("PATCH", h.credentialPath(fixture.Credential.Id))
	require.True(t, ok)
	// Omitted value, domain, and injection fields must stay out of the PATCH.
	assert.JSONEq(t, `{"enabled":false,"methods":[],"paths":[]}`, string(req.Body))
}

func TestSandboxCredentialsUpdate_RotationAndInjection(t *testing.T) {
	h := newSandboxCredentialsHarness(t)
	fixture := sandboxCredentialFixture()
	h.seedCredential(fixture)

	_, err := h.executeWithStdin("user:new-value", "update", fixture.Credential.Id,
		"--value-file", "-", "--domain", "other.example.com", "--injection-type", "basic", "--output", "json")
	require.NoError(t, err)

	req, ok := h.server.LastRequest("PATCH", h.credentialPath(fixture.Credential.Id))
	require.True(t, ok)
	assert.JSONEq(t, `{"value":"user:new-value","domain":"other.example.com","injection":{"type":"basic"}}`, string(req.Body))
}

func TestSandboxCredentials_ListGetAndVersionsFormats(t *testing.T) {
	for _, format := range []string{"text", "json", "yaml"} {
		for _, operation := range []string{"list", "get", "versions"} {
			t.Run(operation+"/"+format, func(t *testing.T) {
				h := newSandboxCredentialsHarness(t)
				fixture := sandboxCredentialHistoryFixture()
				h.seedCredential(fixture)
				args := []string{operation, "--output", format}
				want := fixture.Credential.Id
				if operation != "list" {
					args = append(args, fixture.Credential.Id)
				}
				if operation == "versions" {
					want = fixture.Credential.Version.Fingerprint
				}

				result, err := h.execute(args...)
				require.NoError(t, err)

				assert.Contains(t, result.Stdout, want)
				assert.NotContains(t, result.Stdout+result.Stderr, fixture.Value)
				assert.NotContains(t, result.Stdout+result.Stderr, fixture.History[0].Value)
			})
		}
	}
}

func TestSandboxCredentials_Restore(t *testing.T) {
	h := newSandboxCredentialsHarness(t)
	fixture := sandboxCredentialHistoryFixture()
	h.seedCredential(fixture)

	_, err := h.execute("restore", fixture.Credential.Id, "--version", "1", "--output", "json")
	require.NoError(t, err)

	req, ok := h.server.LastRequest("POST", h.credentialPath(fixture.Credential.Id)+"/restore")
	require.True(t, ok)
	assert.JSONEq(t, `{"versionId":1}`, string(req.Body))
}

func TestSandboxCredentialsDelete_PreviewAndIdempotentConfirmation(t *testing.T) {
	t.Run("preview preserves the credential", func(t *testing.T) {
		h := newSandboxCredentialsHarness(t)
		fixture := sandboxCredentialFixture()
		h.seedCredential(fixture)

		result, err := h.execute("delete", fixture.Credential.Id, "--output", "text")
		require.NoError(t, err)

		assert.Contains(t, result.Stdout, "--confirm")
		assert.False(t, h.server.HasDeleteRequest())
	})
	tests := []struct {
		name    string
		fixture *renderapi.SandboxCredentialState
	}{
		{name: "existing credential", fixture: sandboxCredentialFixture()},
		{name: "already deleted credential"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxCredentialsHarness(t)
			if tc.fixture != nil {
				h.seedCredential(tc.fixture)
			}

			result, err := h.execute("delete", sandboxCredentialTestID, "--confirm", "--output", "json")
			require.NoError(t, err)

			assert.True(t, h.server.HasRequest("DELETE", h.credentialPath(sandboxCredentialTestID)))
			var out sandboxcredential.DeleteOut
			require.NoError(t, json.Unmarshal([]byte(result.Stdout), &out))
			assert.True(t, out.Meta.Deleted)
		})
	}
}

func TestSandboxCredentials_GroupResolution(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*sandboxCredentialsHarness)
		explicit  bool
		wantError string
	}{
		{name: "default group"},
		{name: "explicit group skips lookup", explicit: true, setup: func(h *sandboxCredentialsHarness) {
			h.server.SandboxGroups.RespondWith(http.StatusForbidden)
		}},
		{name: "no default group", wantError: "no default sandbox group", setup: func(h *sandboxCredentialsHarness) {
			h.server.SandboxGroups.Only(h.t).IsDefault = false
		}},
		{name: "lookup failure", wantError: "not allowed", setup: func(h *sandboxCredentialsHarness) {
			h.server.SandboxGroups.RespondWith(http.StatusForbidden)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxCredentialsHarness(t)
			fixture := sandboxCredentialFixture()
			h.seedCredential(fixture)
			if tc.setup != nil {
				tc.setup(h)
			}
			args := []string{"list", "--output", "json"}
			if tc.explicit {
				args = append(args, "--group", h.group)
			}

			result, err := h.execute(args...)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				assert.False(t, h.server.HasRequest("GET", "/credentials"))
				return
			}
			require.NoError(t, err)
			assert.Contains(t, result.Stdout, fixture.Credential.Id)
		})
	}
}

func TestSandboxCredentials_GroupIsolation(t *testing.T) {
	h := newSandboxCredentialsHarness(t)
	fixture := sandboxCredentialFixture()
	h.seedCredential(fixture)
	other := seedSandboxGroup(h.server, "Other")

	result, err := h.execute("list", "--group", other.Id, "--output", "json")
	require.NoError(t, err)
	assert.JSONEq(t, "[]", result.Stdout)

	_, err = h.execute("get", fixture.Credential.Id, "--group", other.Id, "--output", "json")
	require.ErrorContains(t, err, "404")
}

func TestSandboxCredentials_ValidationBeforeRequests(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing args", []string{"get"}, "accepts 1 arg"},
		{"empty ID", []string{"get", ""}, "credential ID must not"},
		{"current path", []string{"get", "."}, "credential ID must not"},
		{"parent path", []string{"get", ".."}, "credential ID must not"},
		{"missing required fields", []string{"create", "key"}, "create requires"},
		{"no update", []string{"update", "key"}, "at least one"},
		{"mutually exclusive secret", []string{"update", "key", "--value", "canary", "--value-file", "-"}, "none of the others"},
		{"missing file", []string{"update", "key", "--value-file", "/nonexistent/credential"}, "read --value-file"},
		{"missing version", []string{"restore", "key"}, `required flag(s) "version" not set`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxCredentialsHarness(t)
			_, err := h.execute(tc.args...)
			require.ErrorContains(t, err, tc.want)
			assert.Empty(t, h.server.Requests)
		})
	}
}

func TestSandboxCredentials_ServerValidation(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantBody string
	}{
		{"invalid ID", []string{"get", "../key"}, ""},
		{"reserved ID", []string{"get", "rnd-key"}, ""},
		{"ID characters", []string{"get", "key with spaces"}, ""},
		{"create ID", append(sandboxCredentialCreateArgs("."), "--value", "test-secret"), `{"id":".","domain":"api.example.com","value":"test-secret","methods":[],"paths":[],"injection":{"type":"header","name":"Authorization"}}`},
		{"empty domain", []string{"update", "key", "--domain="}, `{"domain":""}`},
		{"invalid injection", []string{"update", "key", "--injection-type", "unknown"}, `{"injection":{"type":"unknown"}}`},
		{"missing name", []string{"update", "key", "--injection-type", "header"}, `{"injection":{"type":"header"}}`},
		{"basic name", []string{"update", "key", "--injection-type", "basic", "--injection-name", "foo"}, `{"injection":{"type":"basic","name":"foo"}}`},
		{"name without type", []string{"update", "key", "--injection-name", "foo"}, `{"injection":{"type":"","name":"foo"}}`},
		{"template without type", []string{"update", "key", "--injection-template", "foo"}, `{"injection":{"type":"","template":"foo"}}`},
		{"missing placeholder", []string{"update", "key", "--injection-type", "basic", "--injection-template", "foo"}, `{"injection":{"type":"basic","template":"foo"}}`},
		{"too many placeholders", []string{"update", "key", "--injection-type", "basic", "--injection-template", strings.Repeat("${SECRET}", 6)}, `{"injection":{"type":"basic","template":"${SECRET}${SECRET}${SECRET}${SECRET}${SECRET}${SECRET}"}}`},
		{"header characters", []string{"update", "key", "--injection-type", "header", "--injection-name", "bad\nname"}, `{"injection":{"type":"header","name":"bad\nname"}}`},
		{"header value characters", []string{"update", "key", "--value", "test\nsecret"}, `{"value":"test\nsecret"}`},
		{"invalid version", []string{"restore", "key", "--version", "0"}, `{"versionId":0}`},
		{"negative version", []string{"restore", "key", "--version", "-1"}, `{"versionId":-1}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxCredentialsHarness(t)
			method := http.MethodPatch
			path := h.credentialPath(url.PathEscape(tc.args[1]))
			switch tc.args[0] {
			case "create":
				method = http.MethodPost
				path = "/sandbox-groups/" + h.group + "/credentials"
				h.server.SandboxCredentials.RespondWithErrorForCreate(http.StatusBadRequest)
			case "get":
				method = http.MethodGet
				h.server.SandboxCredentials.RespondWithErrorForGet(http.StatusBadRequest)
			case "update":
				h.server.SandboxCredentials.RespondWithErrorForUpdate(http.StatusBadRequest)
			case "restore":
				method = http.MethodPost
				path += "/restore"
				h.server.SandboxCredentials.RespondWithErrorForRestore(http.StatusBadRequest)
			}

			_, err := h.execute(append(tc.args, "--group", h.group, "--output", "json")...)

			require.ErrorContains(t, err, "received response code 400 (credential_error): credential operation failed")
			req, ok := h.server.LastRequest(method, path)
			require.True(t, ok, "validation must reach the server")
			if tc.wantBody != "" {
				assert.JSONEq(t, tc.wantBody, string(req.Body))
			}
		})
	}
}

func TestSandboxCredentials_APIError(t *testing.T) {
	operations := []struct {
		name    string
		args    []string
		respond func(*renderapi.SandboxCredentialResource, int)
	}{
		{"list", []string{"list"}, (*renderapi.SandboxCredentialResource).RespondWithErrorForList},
		{"get", []string{"get", sandboxCredentialTestID}, (*renderapi.SandboxCredentialResource).RespondWithErrorForGet},
		{"create", append(sandboxCredentialCreateArgs("new-key"), "--value", "test-secret"), (*renderapi.SandboxCredentialResource).RespondWithErrorForCreate},
		{"update", []string{"update", sandboxCredentialTestID, "--enabled=false"}, (*renderapi.SandboxCredentialResource).RespondWithErrorForUpdate},
		{"delete", []string{"delete", sandboxCredentialTestID, "--confirm"}, (*renderapi.SandboxCredentialResource).RespondWithErrorForDelete},
		{"versions", []string{"versions", sandboxCredentialTestID}, (*renderapi.SandboxCredentialResource).RespondWithErrorForVersions},
		{"restore", []string{"restore", sandboxCredentialTestID, "--version", "1"}, (*renderapi.SandboxCredentialResource).RespondWithErrorForRestore},
	}
	statuses := []struct {
		code      int
		wantError string
	}{
		{http.StatusBadRequest, "received response code 400"},
		{http.StatusUnauthorized, command.ErrTokenExpired.Error()},
		{http.StatusForbidden, command.ErrActionNotAllowed.Error()},
		{http.StatusNotFound, "received response code 404"},
		{http.StatusConflict, "received response code 409"},
		{http.StatusTooManyRequests, client.ErrTooManyRequests.Error()},
		{http.StatusInternalServerError, "received response code 500"},
		{http.StatusServiceUnavailable, "received response code 503"},
	}
	for _, operation := range operations {
		for _, status := range statuses {
			t.Run(fmt.Sprintf("%s/%d", operation.name, status.code), func(t *testing.T) {
				h := newSandboxCredentialsHarness(t)
				h.seedCredential(sandboxCredentialHistoryFixture())
				operation.respond(h.server.SandboxCredentials, status.code)

				_, err := h.execute(append(operation.args, "--group", h.group, "--output", "json")...)

				require.ErrorContains(t, err, status.wantError)
			})
		}
	}
}

func TestSandboxCredentials_Help(t *testing.T) {
	h := newSandboxCredentialsHarness(t)

	result, err := h.execute("--help")
	require.NoError(t, err)

	for _, operation := range []string{"create", "get", "list", "update", "delete", "versions", "restore"} {
		assert.Contains(t, result.Stdout, operation)
	}
}

func TestSandboxCredentials_SecretSizeLimit(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		wantError string
	}{
		{name: "empty explicit value", args: []string{"--value", ""}},
		{name: "exact byte limit", args: []string{"--value-file", "-"}, stdin: strings.Repeat("x", 5120)},
		{name: "oversized flag", args: []string{"--value", strings.Repeat("x", 5121)}, wantError: "received response code 400"},
		{name: "oversized stdin", args: []string{"--value-file", "-"}, stdin: strings.Repeat("x", 5121), wantError: "received response code 400"},
		{name: "limit counts UTF-8 bytes", args: []string{"--value-file", "-"}, stdin: strings.Repeat("é", 2561), wantError: "received response code 400"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newSandboxCredentialsHarness(t)
			if tc.wantError != "" {
				h.server.SandboxCredentials.RespondWithErrorForCreate(http.StatusBadRequest)
			}

			_, err := h.executeWithStdin(tc.stdin, append(sandboxCredentialCreateArgs(sandboxCredentialTestID), tc.args...)...)

			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
			} else {
				require.NoError(t, err)
			}
			req, ok := h.server.LastRequest("POST", "/sandbox-groups/"+h.group+"/credentials")
			require.True(t, ok)
			body := testrequire.ParseJSONMap(t, string(req.Body))
			wantValue := tc.stdin
			if tc.args[0] == "--value" {
				wantValue = tc.args[1]
			}
			assert.Equal(t, wantValue, body["value"], "the server receives the complete value")
		})
	}
}
