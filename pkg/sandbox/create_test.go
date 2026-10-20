package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

func TestServiceCreate_NetworkPolicyBody(t *testing.T) {
	tests := []struct {
		name       string
		input      CreateInput
		wantPolicy map[string]any // expected networkPolicy in the request body
		wantAbsent bool           // expect no networkPolicy key at all
	}{
		{
			name:       "unset omits networkPolicy so the server default applies",
			input:      CreateInput{},
			wantAbsent: true,
		},
		{
			name:       "deny-all is sent as networkPolicy.type",
			input:      CreateInput{NetworkPolicy: "deny-all"},
			wantPolicy: map[string]any{"type": "deny-all"},
		},
		{
			name:       "allow-all is sent as networkPolicy.type",
			input:      CreateInput{NetworkPolicy: "allow-all"},
			wantPolicy: map[string]any{"type": "allow-all"},
		},
		{
			name: "allow-list sends its rules",
			input: CreateInput{NetworkPolicy: "allow-list", EgressRules: []sandboxclient.SandboxEgressRule{
				{Domain: "api.example.com", Protocol: new(sandboxclient.Https)},
				{Domain: "*.example.com", Protocol: new(sandboxclient.Https)},
			}},
			wantPolicy: map[string]any{
				"type": "allow-list",
				"rules": []any{
					map[string]any{"domain": "api.example.com", "protocol": "https"},
					map[string]any{"domain": "*.example.com", "protocol": "https"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/sandboxes", r.URL.Path)
				require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(sandboxclient.Sandbox{Id: "sbx-1", Status: sandboxclient.SandboxStatusCreating})
			})

			sb, err := svc.Create(context.Background(), tc.input, nil)
			require.NoError(t, err)
			require.NotNil(t, sb)
			assert.Equal(t, "sbx-1", string(sb.Id))

			policy, present := gotBody["networkPolicy"]
			if tc.wantAbsent {
				assert.False(t, present, "networkPolicy should be omitted, got %v", policy)
				return
			}
			require.True(t, present, "networkPolicy should be present in request body")
			assert.Equal(t, tc.wantPolicy, policy)
		})
	}
}

func TestServiceCreate_IdleTimeoutBody(t *testing.T) {
	tests := []struct {
		name        string
		input       CreateInput
		wantPresent bool
		want        float64
	}{
		{name: "unset omits idle timeout", input: CreateInput{}},
		{name: "custom timeout", input: CreateInput{IdleTimeout: new(1800)}, wantPresent: true, want: 1800},
		{name: "zero disables", input: CreateInput{IdleTimeout: new(0)}, wantPresent: true, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(sandboxclient.Sandbox{Id: "sbx-1", Status: sandboxclient.SandboxStatusCreating})
			})

			_, err := svc.Create(context.Background(), tc.input, nil)
			require.NoError(t, err)

			got, present := gotBody["idleTimeoutSeconds"]
			assert.Equal(t, tc.wantPresent, present)
			if tc.wantPresent {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestServiceCreate_EnvBody(t *testing.T) {
	tests := []struct {
		name       string
		input      CreateInput
		wantEnv    map[string]any
		wantAbsent bool
	}{
		{
			name:       "no env omits the field",
			input:      CreateInput{},
			wantAbsent: true,
		},
		{
			name:    "env pairs are sent verbatim",
			input:   CreateInput{Env: map[string]string{"FOO": "bar", "EMPTY": ""}},
			wantEnv: map[string]any{"FOO": "bar", "EMPTY": ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(sandboxclient.Sandbox{Id: "sbx-1", Status: sandboxclient.SandboxStatusCreating})
			})

			_, err := svc.Create(context.Background(), tc.input, nil)
			require.NoError(t, err)

			env, present := gotBody["env"]
			if tc.wantAbsent {
				assert.False(t, present, "env should be omitted, got %v", env)
				return
			}
			require.True(t, present)
			assert.Equal(t, tc.wantEnv, env)
		})
	}
}

func TestServiceCreate_SnapshotSourceBody(t *testing.T) {
	tests := []struct {
		name             string
		input            CreateInput
		wantSnapshotID   string
		wantSnapshotName string
	}{
		{name: "unset omits snapshot source", input: CreateInput{}},
		{name: "snapshot id is sent", input: CreateInput{SnapshotID: "snp-abc123"}, wantSnapshotID: "snp-abc123"},
		{name: "snapshot name is sent", input: CreateInput{SnapshotName: stringPointer("gold")}, wantSnapshotName: "gold"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(sandboxclient.Sandbox{Id: "sbx-1", Status: sandboxclient.SandboxStatusCreating})
			})

			_, err := svc.Create(context.Background(), tc.input, nil)
			require.NoError(t, err)

			if tc.wantSnapshotID == "" {
				assert.NotContains(t, gotBody, "snapshotId")
			} else {
				assert.Equal(t, tc.wantSnapshotID, gotBody["snapshotId"])
			}
			if tc.wantSnapshotName == "" {
				assert.NotContains(t, gotBody, "snapshotName")
			} else {
				assert.Equal(t, tc.wantSnapshotName, gotBody["snapshotName"])
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
