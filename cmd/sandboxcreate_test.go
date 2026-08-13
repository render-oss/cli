package cmd

import (
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
