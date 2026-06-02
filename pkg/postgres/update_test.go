package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/postgres"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

func TestBuildUpdateRequest_OnlyNameSet(t *testing.T) {
	input := pgtypes.UpdatePostgresInput{
		IDOrName: "my-db",
		Name:     pointers.From("new-name"),
	}
	body, err := postgres.BuildUpdateRequest(input)
	require.NoError(t, err)

	assert.Equal(t, pointers.From("new-name"), body.Name)
	assert.Nil(t, body.Plan)
	assert.Nil(t, body.DiskSizeGB)
	assert.Nil(t, body.EnableDiskAutoscaling)
	assert.Nil(t, body.EnableHighAvailability)
	assert.Nil(t, body.DatadogAPIKey)
	assert.Nil(t, body.DatadogSite)
	assert.Nil(t, body.ParameterOverrides, "nil (not empty map) is load-bearing: a non-nil empty map would clear all overrides and potentially restart the database")
	assert.Nil(t, body.IpAllowList, "omitting both IP flags must leave IpAllowList nil so the API leaves it unchanged")
}

func TestBuildUpdateRequest_IPAllowListReplace(t *testing.T) {
	input := pgtypes.UpdatePostgresInput{
		IDOrName: "my-db",
		IPAllowList: []string{
			"cidr=10.0.0.0/8,description=internal",
			"cidr=203.0.113.5/32,description=office",
		},
	}
	body, err := postgres.BuildUpdateRequest(input)
	require.NoError(t, err)

	require.NotNil(t, body.IpAllowList)
	require.Len(t, *body.IpAllowList, 2)
	assert.Equal(t, "10.0.0.0/8", (*body.IpAllowList)[0].CidrBlock)
	assert.Equal(t, "internal", (*body.IpAllowList)[0].Description)
	assert.Equal(t, "203.0.113.5/32", (*body.IpAllowList)[1].CidrBlock)
	assert.Equal(t, "office", (*body.IpAllowList)[1].Description)
}

func TestBuildUpdateRequest_ClearIPAllowList(t *testing.T) {
	input := pgtypes.UpdatePostgresInput{
		IDOrName:         "my-db",
		ClearIPAllowList: true,
	}
	body, err := postgres.BuildUpdateRequest(input)
	require.NoError(t, err)

	// Must be a non-nil pointer to an empty slice (not nil — that means "leave alone")
	require.NotNil(t, body.IpAllowList)
	assert.Empty(t, *body.IpAllowList)
}

func TestBuildUpdateRequest_ParameterOverrides(t *testing.T) {
	input := pgtypes.UpdatePostgresInput{
		IDOrName:           "my-db",
		ParameterOverrides: []string{"max_connections=100", "shared_buffers=256MB"},
	}
	body, err := postgres.BuildUpdateRequest(input)
	require.NoError(t, err)

	require.NotNil(t, body.ParameterOverrides)
	assert.Equal(t, "100", (*body.ParameterOverrides)["max_connections"])
	assert.Equal(t, "256MB", (*body.ParameterOverrides)["shared_buffers"])
}

func TestBuildUpdateRequest_MalformedParameterOverride_ReturnsError(t *testing.T) {
	input := pgtypes.UpdatePostgresInput{
		IDOrName:           "my-db",
		ParameterOverrides: []string{"noequals"},
	}
	_, err := postgres.BuildUpdateRequest(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KEY=VALUE")
}

func TestBuildUpdateRequest_MalformedIPAllowList_ReturnsError(t *testing.T) {
	input := pgtypes.UpdatePostgresInput{
		IDOrName:    "my-db",
		IPAllowList: []string{"not-valid"},
	}
	_, err := postgres.BuildUpdateRequest(input)
	require.Error(t, err)
	// Confirm it's the IP allow-list parse error that surfaced (not some other
	// failure), so the message stays user-actionable.
	assert.Contains(t, err.Error(), "--ip-allow-list")
}

func TestBuildUpdateRequest_AllScalarFields(t *testing.T) {
	input := pgtypes.UpdatePostgresInput{
		IDOrName:         "my-db",
		Name:             pointers.From("renamed"),
		Plan:             pointers.From("standard"),
		DiskSizeGB:       pointers.From(100),
		DiskAutoscaling:  pointers.From(true),
		HighAvailability: pointers.From(true),
		DatadogAPIKey:    pointers.From("dd-key"),
		DatadogSite:      pointers.From("US3"),
	}
	body, err := postgres.BuildUpdateRequest(input)
	require.NoError(t, err)

	assert.Equal(t, pointers.From("renamed"), body.Name)
	require.NotNil(t, body.Plan)
	assert.Equal(t, pgclient.PostgresPlans("standard"), *body.Plan)
	assert.Equal(t, pointers.From(100), body.DiskSizeGB)
	assert.Equal(t, pointers.From(true), body.EnableDiskAutoscaling)
	assert.Equal(t, pointers.From(true), body.EnableHighAvailability)
	assert.Equal(t, pointers.From("dd-key"), body.DatadogAPIKey)
	assert.Equal(t, pointers.From("US3"), body.DatadogSite)
}
