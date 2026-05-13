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
	assert.Contains(t, values, "starter")
	assert.Contains(t, values, "standard")
	assert.Contains(t, values, "pro")
	assert.Contains(t, values, "pro_plus")
	assert.NotContains(t, values, "custom", "although 'custom' is generated as an enum value in the OpenAPI spec, it is not actually a valid plan name, and instead an indicator that other plans exist")
	assert.Len(t, values, 5)
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
	envID := testids.EnvironmentID("optional")
	input := kvtypes.KeyValueCreateRequestInput{
		Name:            "my-kv",
		OwnerID:         "tea-owner-abc",
		Plan:            kvtypes.PlanPro,
		Region:          &region,
		MaxmemoryPolicy: &policy,
		EnvironmentID:   &envID,
	}
	body, err := keyvalue.BuildCreateRequest(input)
	require.NoError(t, err)
	assert.Equal(t, client.KeyValuePlan(kvtypes.PlanPro), body.Plan)
	require.NotNil(t, body.Region)
	assert.Equal(t, "virginia", *body.Region)
	require.NotNil(t, body.MaxmemoryPolicy)
	assert.Equal(t, client.AllkeysLru, *body.MaxmemoryPolicy)
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
