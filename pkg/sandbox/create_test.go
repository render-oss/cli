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
		name        string
		input       CreateInput
		wantDefault string // expected networkPolicy.default in the request body
		wantAbsent  bool   // expect no networkPolicy key at all
	}{
		{
			name:       "unset omits networkPolicy so the server default applies",
			input:      CreateInput{},
			wantAbsent: true,
		},
		{
			name:        "deny-all is sent as networkPolicy.default",
			input:       CreateInput{NetworkPolicy: "deny-all"},
			wantDefault: "deny-all",
		},
		{
			name:        "allow-all is sent as networkPolicy.default",
			input:       CreateInput{NetworkPolicy: "allow-all"},
			wantDefault: "allow-all",
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
			policyObj, ok := policy.(map[string]any)
			require.True(t, ok, "networkPolicy should be an object, got %T", policy)
			assert.Equal(t, tc.wantDefault, policyObj["default"])
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
