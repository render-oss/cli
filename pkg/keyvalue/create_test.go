package keyvalue_test

import (
	"testing"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/keyvalue"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanValues(t *testing.T) {
	values := keyvalue.PlanValues()
	assert.Contains(t, values, "free")
	assert.Contains(t, values, "256mb")
	assert.Contains(t, values, "1g")
	assert.Contains(t, values, "5g")
	assert.Contains(t, values, "10g")
	assert.Contains(t, values, "20g")
	assert.Contains(t, values, "40g")
	assert.NotContains(t, values, "custom", "although 'custom' is generated as an enum value in the OpenAPI spec, it is not actually a valid plan name, and instead an indicator that other plans exist")
	assert.Len(t, values, 7)
}

// TestPlanOptions is a completeness guard: every well-known plan returned by
// PlanValues() must appear in PlanOptions() with a non-empty display label, in
// the same order, so a newly added plan can't silently reach the interactive
// picker without a label.
func TestPlanOptions(t *testing.T) {
	options := keyvalue.PlanOptions()
	values := keyvalue.PlanValues()

	require.Len(t, options, len(values), "PlanOptions must cover every PlanValues entry")
	for i, opt := range options {
		assert.Equal(t, values[i], opt.Value, "PlanOptions must preserve PlanValues order")
		assert.NotEmpty(t, opt.Label, "plan %q is missing a display label", opt.Value)
	}
}

// advertisedPlans lists every KV plan the CLI advertises, with the display label
// the picker must show for it. Spelling the expectations out here rather than
// reusing the production list keeps this an independent check on which names
// reach users, so a plan added to the schema forces a conscious update.
var advertisedPlans = []struct {
	plan  client.KeyValuePlan
	label string
}{
	// free has no spec-based name, but it is a real offering users pick, so it
	// stays advertised alongside the spec-based names.
	{plan: client.KeyValuePlanFree, label: "Free"},

	{plan: "256mb", label: "256mb"},
	{plan: "1g", label: "1g"},
	{plan: "5g", label: "5g"},
	{plan: "10g", label: "10g"},
	{plan: "20g", label: "20g"},
	{plan: "40g", label: "40g"},
}

// unadvertisedPlans lists the older KV plan names that remain valid --plan input
// the CLI passes through to the API, but that no longer appear in help text or
// the interactive picker.
var unadvertisedPlans = []client.KeyValuePlan{
	client.KeyValuePlanStarter,
	client.KeyValuePlanStandard,
	client.KeyValuePlanPro,
	client.KeyValuePlanProPlus,
}

// TestAdvertisedPlans checks that PlanValues and PlanOptions offer exactly the
// advertised plans with their labels, and that BuildCreateRequest passes each
// through unchanged. That the rendered --plan help matches PlanValues is covered
// by TestPlanFlagHelpOffersTheAdvertisedPlans in the cmd package.
func TestAdvertisedPlans(t *testing.T) {
	values := keyvalue.PlanValues()
	labels := map[string]string{}
	for _, opt := range keyvalue.PlanOptions() {
		labels[opt.Value] = opt.Label
	}

	wantValues := make([]string, 0, len(advertisedPlans))
	for _, tc := range advertisedPlans {
		wantValues = append(wantValues, string(tc.plan))
	}
	assert.ElementsMatch(t, wantValues, values,
		"update advertisedPlans when the schema changes which plans are advertised")

	for _, tc := range advertisedPlans {
		t.Run(string(tc.plan), func(t *testing.T) {
			assert.True(t, tc.plan.Valid(), "%q must be a valid KeyValuePlan value", tc.plan)
			assert.Contains(t, values, string(tc.plan),
				"%q must be advertised", tc.plan)
			assert.Equal(t, tc.label, labels[string(tc.plan)])

			body, err := keyvalue.BuildCreateRequest(kvtypes.KeyValueCreateRequestInput{
				Name:    "my-kv",
				OwnerID: "tea-owner-abc",
				Plan:    string(tc.plan),
			})
			require.NoError(t, err)
			assert.Equal(t, tc.plan, body.Plan, "--plan must reach the API unchanged")
		})
	}
}

// TestUnadvertisedPlansAreStillAccepted covers the older names: each is a valid
// value that --plan passes through, it just is not advertised.
func TestUnadvertisedPlansAreStillAccepted(t *testing.T) {
	values := keyvalue.PlanValues()

	for _, plan := range unadvertisedPlans {
		t.Run(string(plan), func(t *testing.T) {
			assert.True(t, plan.Valid(), "%q must be a valid KeyValuePlan value", plan)
			assert.NotContains(t, values, string(plan),
				"%q must not be advertised", plan)

			body, err := keyvalue.BuildCreateRequest(kvtypes.KeyValueCreateRequestInput{
				Name:    "my-kv",
				OwnerID: "tea-owner-abc",
				Plan:    string(plan),
			})
			require.NoError(t, err)
			assert.Equal(t, plan, body.Plan, "--plan must reach the API unchanged")
		})
	}
}

func TestBuildCreateRequest_AllowsArbitraryPlanNames(t *testing.T) {
	input := kvtypes.KeyValueCreateRequestInput{
		Name:    "my-kv",
		OwnerID: "tea-owner-abc",
		Plan:    "Pro Plus",
	}
	body, err := keyvalue.BuildCreateRequest(input)
	require.NoError(t, err)
	assert.Equal(t, client.KeyValuePlan("Pro Plus"), body.Plan)
}

func TestBuildCreateRequest_RequiredFields(t *testing.T) {
	input := kvtypes.KeyValueCreateRequestInput{
		Name:    "my-kv",
		OwnerID: "tea-owner-abc",
		Plan:    kvtypes.PlanFree,
	}
	body, err := keyvalue.BuildCreateRequest(input)
	require.NoError(t, err)
	assert.Equal(t, "my-kv", body.Name)
	assert.Equal(t, "tea-owner-abc", body.OwnerId)
	assert.Equal(t, client.KeyValuePlan(kvtypes.PlanFree), body.Plan)
	assert.Nil(t, body.Region)
	assert.Nil(t, body.MaxmemoryPolicy)
	assert.Nil(t, body.PersistenceMode)
	assert.Nil(t, body.EnvironmentId)
	assert.Nil(t, body.IpAllowList)
}

func TestBuildCreateRequest_MissingRequiredFields(t *testing.T) {
	base := kvtypes.KeyValueCreateRequestInput{
		Name:    "my-kv",
		OwnerID: "tea-owner-abc",
		Plan:    kvtypes.PlanFree,
	}

	cases := []struct {
		name        string
		input       kvtypes.KeyValueCreateRequestInput
		errContains string
	}{
		{
			name: "name",
			input: kvtypes.KeyValueCreateRequestInput{
				OwnerID: base.OwnerID,
				Plan:    base.Plan,
			},
			errContains: "name is required",
		},
		{
			name: "owner ID",
			input: kvtypes.KeyValueCreateRequestInput{
				Name: base.Name,
				Plan: base.Plan,
			},
			errContains: "owner ID is required",
		},
		{
			name: "plan",
			input: kvtypes.KeyValueCreateRequestInput{
				Name:    base.Name,
				OwnerID: base.OwnerID,
			},
			errContains: "plan is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := keyvalue.BuildCreateRequest(tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestBuildCreateRequest_OptionalFields(t *testing.T) {
	region := "virginia"
	policy := kvtypes.MaxmemoryPolicyAllkeysLru
	persistence := client.PersistenceModeSnapshot
	envID := testids.EnvironmentID("optional")
	input := kvtypes.KeyValueCreateRequestInput{
		Name:            "my-kv",
		OwnerID:         "tea-owner-abc",
		Plan:            kvtypes.PlanPro,
		Region:          &region,
		MaxmemoryPolicy: &policy,
		PersistenceMode: &persistence,
		EnvironmentID:   &envID,
	}
	body, err := keyvalue.BuildCreateRequest(input)
	require.NoError(t, err)
	assert.Equal(t, client.KeyValuePlan(kvtypes.PlanPro), body.Plan)
	require.NotNil(t, body.Region)
	assert.Equal(t, client.Region("virginia"), *body.Region)
	require.NotNil(t, body.MaxmemoryPolicy)
	assert.Equal(t, client.AllkeysLru, *body.MaxmemoryPolicy)
	require.NotNil(t, body.PersistenceMode)
	assert.Equal(t, client.PersistenceModeSnapshot, *body.PersistenceMode)
	require.NotNil(t, body.EnvironmentId)
	assert.Equal(t, envID, *body.EnvironmentId)
}

func TestBuildCreateRequest_CommonPlanValues(t *testing.T) {
	cases := []struct {
		plan     string
		expected client.KeyValuePlan
	}{
		{kvtypes.PlanFree, client.KeyValuePlanFree},
		{kvtypes.PlanStarter, client.KeyValuePlanStarter},
		{kvtypes.PlanStandard, client.KeyValuePlanStandard},
		{kvtypes.PlanPro, client.KeyValuePlanPro},
		{kvtypes.PlanProPlus, client.KeyValuePlanProPlus},
	}
	for _, tc := range cases {
		t.Run(tc.plan, func(t *testing.T) {
			input := kvtypes.KeyValueCreateRequestInput{Name: "x", OwnerID: "tea-owner", Plan: tc.plan}
			body, err := keyvalue.BuildCreateRequest(input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, body.Plan)
		})
	}
}

func TestBuildCreateRequest_IpAllowList(t *testing.T) {
	input := kvtypes.KeyValueCreateRequestInput{
		Name:        "my-kv",
		OwnerID:     "tea-owner",
		Plan:        kvtypes.PlanFree,
		IPAllowList: []string{"cidr=1.2.3.4/32,description=office", "cidr=10.0.0.0/8,description=internal"},
	}
	body, err := keyvalue.BuildCreateRequest(input)
	require.NoError(t, err)
	require.NotNil(t, body.IpAllowList)
	assert.Len(t, *body.IpAllowList, 2)
}

func TestBuildCreateRequest_EmptyIpAllowList(t *testing.T) {
	input := kvtypes.KeyValueCreateRequestInput{
		Name:    "my-kv",
		OwnerID: "tea-owner",
		Plan:    kvtypes.PlanFree,
	}
	body, err := keyvalue.BuildCreateRequest(input)
	require.NoError(t, err)
	assert.Nil(t, body.IpAllowList)
}

func TestBuildCreateRequest_AllMaxmemoryPolicyValues(t *testing.T) {
	cases := []struct {
		policy   kvtypes.MaxmemoryPolicy
		expected client.MaxmemoryPolicy
	}{
		{kvtypes.MaxmemoryPolicyNoeviction, client.Noeviction},
		{kvtypes.MaxmemoryPolicyAllkeysLfu, client.AllkeysLfu},
		{kvtypes.MaxmemoryPolicyAllkeysLru, client.AllkeysLru},
		{kvtypes.MaxmemoryPolicyAllkeysRandom, client.AllkeysRandom},
		{kvtypes.MaxmemoryPolicyVolatileLfu, client.VolatileLfu},
		{kvtypes.MaxmemoryPolicyVolatileLru, client.VolatileLru},
		{kvtypes.MaxmemoryPolicyVolatileRandom, client.VolatileRandom},
		{kvtypes.MaxmemoryPolicyVolatileTtl, client.VolatileTtl},
	}
	for _, tc := range cases {
		t.Run(string(tc.policy), func(t *testing.T) {
			p := tc.policy
			input := kvtypes.KeyValueCreateRequestInput{
				Name:            "x",
				OwnerID:         "tea-owner",
				Plan:            kvtypes.PlanFree,
				MaxmemoryPolicy: &p,
			}
			body, err := keyvalue.BuildCreateRequest(input)
			require.NoError(t, err)
			require.NotNil(t, body.MaxmemoryPolicy)
			assert.Equal(t, tc.expected, *body.MaxmemoryPolicy)
		})
	}
}
