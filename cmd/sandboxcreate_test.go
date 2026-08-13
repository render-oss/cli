package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

func TestSandboxCreateInputValidate(t *testing.T) {
	cases := []struct {
		name        string
		plan        string
		errContains string
	}{
		{name: "empty plan is allowed (server picks default)", plan: ""},
		{name: "starter", plan: "starter"},
		{name: "standard", plan: "standard"},
		{name: "pro", plan: "pro"},
		{name: "unknown plan", plan: "mega", errContains: `invalid plan "mega"`},
		{name: "legacy-style name rejected", plan: "pro_plus", errContains: `invalid plan "pro_plus"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := SandboxCreateInput{Plan: tc.plan}
			err := input.Validate(false)
			if tc.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
			// The error must enumerate the valid plans so the user knows what to use.
			for _, name := range sandboxPlanNames() {
				assert.Contains(t, err.Error(), name)
			}
		})
	}
}

// TestSandboxCreateValidateAcceptsEverySchemaPlan guards that every plan the
// schema considers valid is also accepted by Validate, so the two never drift.
func TestSandboxCreateValidateAcceptsEverySchemaPlan(t *testing.T) {
	for _, p := range sandboxclient.SandboxPlanValues() {
		input := SandboxCreateInput{Plan: string(p)}
		assert.NoError(t, input.Validate(false), "plan %q should be accepted", p)
	}
}

func TestSandboxCreateInputValidateNetworkPolicy(t *testing.T) {
	cases := []struct {
		name          string
		networkPolicy string
		errContains   string
	}{
		{name: "empty network policy is allowed (server picks default)", networkPolicy: ""},
		{name: "allow-all", networkPolicy: "allow-all"},
		{name: "deny-all", networkPolicy: "deny-all"},
		{name: "unknown policy", networkPolicy: "somewhat-open", errContains: `invalid network policy "somewhat-open"`},
		{name: "constant-style name rejected", networkPolicy: "AllowAll", errContains: `invalid network policy "AllowAll"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := SandboxCreateInput{NetworkPolicy: tc.networkPolicy}
			err := input.Validate(false)
			if tc.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
			for _, name := range sandboxNetworkPolicyNames() {
				assert.Contains(t, err.Error(), name)
			}
		})
	}
}

func TestSandboxCreateNetworkPolicyNamesMatchSchema(t *testing.T) {
	names := sandboxNetworkPolicyNames()
	require.NotEmpty(t, names)
	for _, name := range names {
		assert.True(t, sandboxclient.SandboxNetworkPolicyDefault(name).Valid(), "policy %q should be valid per schema", name)
		input := SandboxCreateInput{NetworkPolicy: name}
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
