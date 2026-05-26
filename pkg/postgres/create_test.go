package postgres_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/postgres"
)

// Populate a CreateRequestInput with all required fields
func minimalInput() postgres.CreateRequestInput {
	return postgres.CreateRequestInput{
		Name: "my-pg", OwnerID: "tea-abc123xyz456", Plan: "free", Version: 18,
	}
}

func TestBuildCreateRequest_RequiredFields(t *testing.T) {
	t.Run("requires ownerID", func(t *testing.T) {
		in := minimalInput()
		in.OwnerID = ""
		_, err := postgres.BuildCreateRequest(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workspace")
	})

	t.Run("requires name", func(t *testing.T) {
		in := minimalInput()
		in.Name = ""
		_, err := postgres.BuildCreateRequest(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("requires plan", func(t *testing.T) {
		in := minimalInput()
		in.Plan = ""
		_, err := postgres.BuildCreateRequest(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plan")
	})

	t.Run("requires version", func(t *testing.T) {
		in := minimalInput()
		in.Version = 0
		_, err := postgres.BuildCreateRequest(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "version")
	})
}

func TestBuildCreateRequest_RejectsInvalidDiskSize(t *testing.T) {
	in := minimalInput()
	in.DiskSizeGB = pointers.From(7)
	_, err := postgres.BuildCreateRequest(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk-size-gb")
}

func TestBuildCreateRequest_AllFieldsSpecified(t *testing.T) {
	envID := "evm-123"
	body, err := postgres.BuildCreateRequest(postgres.CreateRequestInput{
		Name:             "my-pg",
		OwnerID:          "tea-owner",
		Plan:             "pro_4gb",
		Version:          17,
		Region:           pointers.From("ohio"),
		EnvironmentID:    &envID,
		DatabaseName:     pointers.From("appdb"),
		DatabaseUser:     pointers.From("appuser"),
		HighAvailability: pointers.From(true),
		DiskSizeGB:       pointers.From(100),
		DiskAutoscaling:  pointers.From(true),
		DatadogAPIKey:    pointers.From("dd-key"),
		DatadogSite:      pointers.From("US3"),
		IPAllowList: []string{
			"cidr=10.0.0.0/8,description=internal",
			"cidr=203.0.113.5/32,description=office",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "my-pg", body.Name)
	assert.Equal(t, "tea-owner", body.OwnerId)
	assert.Equal(t, pgclient.PostgresPlans("pro_4gb"), body.Plan)
	assert.Equal(t, client.PostgresVersion(strconv.Itoa(17)), body.Version)
	require.NotNil(t, body.Region)
	assert.Equal(t, client.Region("ohio"), *body.Region)
	assert.Equal(t, pointers.From("appdb"), body.DatabaseName)
	assert.Equal(t, pointers.From("appuser"), body.DatabaseUser)
	assert.Equal(t, pointers.From(true), body.EnableHighAvailability)
	assert.Equal(t, pointers.From(100), body.DiskSizeGB)
	assert.Equal(t, pointers.From(true), body.EnableDiskAutoscaling)
	assert.Equal(t, pointers.From("dd-key"), body.DatadogAPIKey)
	assert.Equal(t, pointers.From("US3"), body.DatadogSite)
	assert.Equal(t, &envID, body.EnvironmentId)
	require.NotNil(t, body.IpAllowList)
	require.Len(t, *body.IpAllowList, 2)
	assert.Equal(t, "10.0.0.0/8", (*body.IpAllowList)[0].CidrBlock)
	assert.Equal(t, "internal", (*body.IpAllowList)[0].Description)
	assert.Equal(t, "203.0.113.5/32", (*body.IpAllowList)[1].CidrBlock)
	assert.Equal(t, "office", (*body.IpAllowList)[1].Description)
}

func TestBuildCreateRequest_ParameterOverrides(t *testing.T) {
	t.Run("parses KEY=VALUE pairs and trims whitespace", func(t *testing.T) {
		in := minimalInput()
		in.ParameterOverrides = []string{"max_connections=100", "  shared_buffers = 256MB  "}
		body, err := postgres.BuildCreateRequest(in)
		require.NoError(t, err)
		require.NotNil(t, body.ParameterOverrides)
		assert.Equal(t, "100", (*body.ParameterOverrides)["max_connections"])
		assert.Equal(t, "256MB", (*body.ParameterOverrides)["shared_buffers"])
	})

	t.Run("rejects when any entry is malformed", func(t *testing.T) {
		in := minimalInput()
		in.ParameterOverrides = []string{"max_connections=100", "noequals", "shared_buffers=256MB"}
		_, err := postgres.BuildCreateRequest(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"noequals"`)
	})

	t.Run("rejects malformed entries", func(t *testing.T) {
		cases := []struct {
			name  string
			entry string
		}{
			{"no equals sign", "noequals"},
			{"empty string", ""},
			{"missing key (=VALUE)", "=value"},
			{"missing value (KEY=)", "max_connections="},
			{"both sides empty (=)", "="},
			{"whitespace-only key", "  =value"},
			{"whitespace-only value", "max_connections=   "},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				in := minimalInput()
				in.ParameterOverrides = []string{tc.entry}
				_, err := postgres.BuildCreateRequest(in)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "KEY=VALUE")
			})
		}
	})
}

func TestBuildCreateRequest_ReadReplicas(t *testing.T) {
	in := minimalInput()
	in.ReadReplicas = []string{"replica-1", "replica-2"}
	body, err := postgres.BuildCreateRequest(in)
	require.NoError(t, err)
	require.NotNil(t, body.ReadReplicas)
	require.Len(t, *body.ReadReplicas, 2)
	assert.Equal(t, "replica-1", (*body.ReadReplicas)[0].Name)
	assert.Equal(t, "replica-2", (*body.ReadReplicas)[1].Name)
}

func TestBuildCreateRequest_OmitsOptionalsWhenUnset(t *testing.T) {
	body, err := postgres.BuildCreateRequest(minimalInput())
	require.NoError(t, err)

	assert.Nil(t, body.Region)
	assert.Nil(t, body.DatabaseName)
	assert.Nil(t, body.DatabaseUser)
	assert.Nil(t, body.EnableHighAvailability)
	assert.Nil(t, body.DiskSizeGB)
	assert.Nil(t, body.EnableDiskAutoscaling)
	assert.Nil(t, body.DatadogAPIKey)
	assert.Nil(t, body.DatadogSite)
	assert.Nil(t, body.EnvironmentId)
	assert.Nil(t, body.IpAllowList)
	assert.Nil(t, body.ParameterOverrides)
	assert.Nil(t, body.ReadReplicas)
}
