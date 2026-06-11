package postgres_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/postgres"
)

func TestNewPostgresListOut_ConstructsItems(t *testing.T) {
	createdAt := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	diskSizeGB := 10
	primaryPostgresID := "dpg-primary"
	replicaOverrides := client.PostgresParameterOverrides{"work_mem": "64MB"}

	pg := &client.Postgres{
		Id:                      "dpg-replica",
		Name:                    "analytics-replica",
		Plan:                    pgclient.Pro4gb,
		Version:                 client.PostgresVersion("17"),
		Region:                  client.Ohio,
		Status:                  client.DatabaseStatusAvailable,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		Owner:                   client.Owner{Id: "tea-owner", Type: client.OwnerTypeTeam},
		EnvironmentId:           pointers.From("evm-prod"),
		DatabaseName:            "analytics",
		DatabaseUser:            "analytics_user",
		DiskSizeGB:              &diskSizeGB,
		DiskAutoscalingEnabled:  true,
		HighAvailabilityEnabled: true,
		IpAllowList:             nil,
		ReadReplicas:            client.ReadReplicas{{Id: "dpg-replica-2", Name: "analytics-replica-2", ParameterOverrides: &replicaOverrides}},
		PrimaryPostgresID:       &primaryPostgresID,
		DashboardUrl:            "https://dashboard.render.com/d/dpg-replica",
	}

	out := postgres.NewPostgresListOut([]*postgres.Model{{
		Postgres:    pg,
		Project:     &client.Project{Id: "prj-prod", Name: "Production Project"},
		Environment: &client.Environment{Id: "evm-prod", Name: "production"},
	}})
	require.Len(t, out.Data, 1)

	expectedPostgres := *pg
	expectedPostgres.IpAllowList = []client.CidrBlockAndDescription{}
	assert.Equal(t, expectedPostgres, out.Data[0].Postgres)
	assert.Equal(t, pointers.From("prj-prod"), out.Data[0].ProjectID)
	assert.Equal(t, "Production Project", out.Data[0].ProjectName)
	assert.Equal(t, "production", out.Data[0].EnvironmentName)
}

// TestNewPostgresGetOut_JSONSerialization locks the public detail JSON contract:
// API-shaped resource fields, the data enrichment fields, and no project/environment names.
func TestNewPostgresGetOut_JSONSerialization(t *testing.T) {
	createdAt := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	primaryPostgresID := "dpg-primary"
	parameterOverrides := client.PostgresParameterOverrides{"max_connections": "100"}
	replicaOverrides := client.PostgresParameterOverrides{"work_mem": "64MB"}

	out := postgres.NewPostgresGetOut(&postgres.ResolvedPostgres{
		Postgres: &client.PostgresDetail{
			Id:                      "dpg-detail",
			Name:                    "analytics",
			Plan:                    pgclient.Pro8gb,
			Version:                 client.PostgresVersion("17"),
			Region:                  client.Virginia,
			Status:                  client.DatabaseStatusAvailable,
			CreatedAt:               createdAt,
			UpdatedAt:               updatedAt,
			Owner:                   client.Owner{Email: "team@example.com", Id: "tea-owner", Name: "Team", Type: client.OwnerTypeTeam},
			EnvironmentId:           pointers.From("evm-prod"),
			DatabaseName:            "analytics",
			DatabaseUser:            "analytics_user",
			DiskSizeGB:              pointers.From(20),
			DiskAutoscalingEnabled:  true,
			HighAvailabilityEnabled: true,
			IpAllowList:             []client.CidrBlockAndDescription{{CidrBlock: "203.0.113.5/32", Description: "office"}},
			ReadReplicas:            client.ReadReplicas{{Id: "dpg-replica", Name: "analytics-replica", ParameterOverrides: &replicaOverrides}},
			PrimaryPostgresID:       &primaryPostgresID,
			ParameterOverrides:      &parameterOverrides,
			Role:                    client.Primary,
			Suspended:               client.PostgresDetailSuspendedNotSuspended,
			Suspenders:              []client.SuspenderType{},
			DashboardUrl:            "https://dashboard.render.com/d/dpg-detail",
		},
		Project:     &client.Project{Id: "prj-prod", Name: "Production Project"},
		Environment: &client.Environment{Id: "evm-prod", Name: "production"},
	})
	out.Data.ConnectionInfo = &client.PostgresConnectionInfo{
		PsqlCommand:              "psql postgres://example",
		InternalConnectionString: "postgres://internal",
		ExternalConnectionString: "postgres://external",
		Password:                 "secret",
	}

	body := marshalJSONMap(t, out.Data)

	assert.NotContains(t, body, "postgres")
	assert.NotContains(t, body, "PostgresListItemOut")
	assert.NotContains(t, body, "postgresListItemOut")
	assert.NotContains(t, body, "projectName")
	assert.NotContains(t, body, "environmentName")
	assert.NotContains(t, body, "ownerId")
	assert.NotContains(t, body, "ownerType")

	assert.Equal(t, map[string]any{
		"id":                      "dpg-detail",
		"name":                    "analytics",
		"plan":                    "pro_8gb",
		"version":                 "17",
		"region":                  "virginia",
		"status":                  "available",
		"createdAt":               createdAt.Format(time.RFC3339Nano),
		"updatedAt":               updatedAt.Format(time.RFC3339Nano),
		"owner":                   map[string]any{"email": "team@example.com", "id": "tea-owner", "name": "Team", "type": "team"},
		"projectId":               "prj-prod",
		"environmentId":           "evm-prod",
		"databaseName":            "analytics",
		"databaseUser":            "analytics_user",
		"diskSizeGB":              float64(20),
		"diskAutoscalingEnabled":  true,
		"highAvailabilityEnabled": true,
		"ipAllowList":             []any{map[string]any{"cidrBlock": "203.0.113.5/32", "description": "office"}},
		"readReplicas":            []any{map[string]any{"id": "dpg-replica", "name": "analytics-replica", "parameterOverrides": map[string]any{"work_mem": "64MB"}}},
		"primaryPostgresID":       "dpg-primary",
		"role":                    "primary",
		"suspended":               "not_suspended",
		"suspenders":              []any{},
		"dashboardUrl":            "https://dashboard.render.com/d/dpg-detail",
		"parameterOverrides":      map[string]any{"max_connections": "100"},
		"connectionInfo": map[string]any{
			"psqlCommand":              "psql postgres://example",
			"internalConnectionString": "postgres://internal",
			"externalConnectionString": "postgres://external",
			"password":                 "secret",
		},
	}, body)
}

// TestNewPostgresGetOut_OmitsEmptyOptionalFields protects the places where nil and
// empty values have meaning in JSON output.
func TestNewPostgresGetOut_NormalizesEmptyOutputFields(t *testing.T) {
	out := postgres.NewPostgresGetOut(&postgres.ResolvedPostgres{
		Postgres: &client.PostgresDetail{
			Id:        "dpg-detail",
			Name:      "analytics",
			Owner:     client.Owner{Id: "tea-owner"},
			CreatedAt: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC),
		},
	})

	body := marshalJSONMap(t, out.Data)

	assert.Equal(t, []any{}, testrequire.SubSlice(t, body, "ipAllowList"))
	assert.Equal(t, []any{}, testrequire.SubSlice(t, body, "readReplicas"))
	assert.NotContains(t, body, "parameterOverrides")
	assert.NotContains(t, body, "connectionInfo")
	assert.NotContains(t, body, "primaryPostgresID")
}

// TestPostgresCommandOutputSerialization keeps each command output type aligned
// with the JSON shape its command loader returns.
func TestPostgresCommandOutputSerialization(t *testing.T) {
	resolved := &postgres.ResolvedPostgres{
		Postgres: &client.PostgresDetail{
			Id:        "dpg-detail",
			Name:      "analytics",
			Owner:     client.Owner{Id: "tea-owner"},
			CreatedAt: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC),
		},
	}

	t.Run("create", func(t *testing.T) {
		createOut := postgres.NewPostgresCreateOut(resolved)

		body := marshalJSONMap(t, createOut)
		assert.Equal(t, "dpg-detail", testrequire.SubMap(t, body, "data")["id"])
	})

	t.Run("get", func(t *testing.T) {
		getOut := postgres.NewPostgresGetOut(resolved)

		body := marshalJSONMap(t, getOut)
		assert.Equal(t, "dpg-detail", testrequire.SubMap(t, body, "data")["id"])
	})

	t.Run("delete", func(t *testing.T) {
		deleteOut := postgres.NewPostgresDeleteOut(resolved)
		deleteOut.Meta = postgres.DeleteOutMeta{
			Deleted: false,
			Message: "Re-run with --confirm to proceed",
		}

		body := marshalJSONMap(t, deleteOut)
		assert.Equal(t, "dpg-detail", testrequire.SubMap(t, body, "data")["id"])
		assert.Equal(t, map[string]any{
			"deleted": false,
			"message": "Re-run with --confirm to proceed",
		}, testrequire.SubMap(t, body, "meta"))
	})

	t.Run("resume", func(t *testing.T) {
		resumeOut := postgres.NewPostgresResumeOut(resolved)

		body := marshalJSONMap(t, resumeOut)
		assert.Equal(t, "dpg-detail", testrequire.SubMap(t, body, "data")["id"])
	})

	t.Run("suspend", func(t *testing.T) {
		suspendOut := postgres.NewPostgresSuspendOut(resolved)
		suspendOut.Meta = postgres.SuspendOutMeta{
			Suspended: false,
			Message:   "Re-run with --confirm to proceed",
		}

		body := marshalJSONMap(t, suspendOut)
		assert.Equal(t, "dpg-detail", testrequire.SubMap(t, body, "data")["id"])
		assert.Equal(t, map[string]any{
			"suspended": false,
			"message":   "Re-run with --confirm to proceed",
		}, testrequire.SubMap(t, body, "meta"))
	})

	t.Run("list", func(t *testing.T) {
		listOut := postgres.NewPostgresListOut([]*postgres.Model{{Postgres: &client.Postgres{
			Id:        "dpg-list",
			Name:      "analytics-list",
			Owner:     client.Owner{Id: "tea-owner"},
			CreatedAt: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC),
		}}})

		body := marshalJSONMap(t, listOut)
		data := testrequire.SubSlice(t, body, "data")
		require.Len(t, data, 1)
		assert.Equal(t, "dpg-list", data[0].(map[string]any)["id"])
	})
}

// TestNewPostgresUpdateOut verifies update output exposes the new resource
// state plus a field-level diff, not raw before/after API snapshots.
func TestNewPostgresUpdateOut(t *testing.T) {
	beforeOverrides := client.PostgresParameterOverrides{"max_connections": "100"}
	afterOverrides := client.PostgresParameterOverrides{"max_connections": "200"}
	beforeDiskSize := 10
	afterDiskSize := 20
	before := &client.PostgresDetail{
		Id:                      "dpg-detail",
		Name:                    "analytics",
		Plan:                    pgclient.Pro4gb,
		DiskSizeGB:              &beforeDiskSize,
		DiskAutoscalingEnabled:  false,
		HighAvailabilityEnabled: false,
		IpAllowList:             []client.CidrBlockAndDescription{{CidrBlock: "203.0.113.5/32", Description: "office"}},
		ParameterOverrides:      &beforeOverrides,
		Owner:                   client.Owner{Id: "tea-owner"},
		CreatedAt:               time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		UpdatedAt:               time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC),
	}
	after := &postgres.ResolvedPostgres{Postgres: &client.PostgresDetail{
		Id:                      "dpg-detail",
		Name:                    "analytics-renamed",
		Plan:                    pgclient.Pro8gb,
		DiskSizeGB:              &afterDiskSize,
		DiskAutoscalingEnabled:  true,
		HighAvailabilityEnabled: true,
		IpAllowList:             []client.CidrBlockAndDescription{},
		ParameterOverrides:      &afterOverrides,
		Owner:                   client.Owner{Id: "tea-owner"},
		CreatedAt:               time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		UpdatedAt:               time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC),
	}}

	updateOut := postgres.NewPostgresUpdateOut(before, after)
	body := marshalJSONMap(t, updateOut)

	testrequire.SubMap(t, body, "data")
	diff := testrequire.SubMap(t, body, "diff")

	assert.Equal(t, map[string]any{
		"name":                    map[string]any{"before": "analytics", "after": "analytics-renamed"},
		"plan":                    map[string]any{"before": "pro_4gb", "after": "pro_8gb"},
		"diskSizeGB":              map[string]any{"before": float64(10), "after": float64(20)},
		"diskAutoscalingEnabled":  map[string]any{"before": false, "after": true},
		"highAvailabilityEnabled": map[string]any{"before": false, "after": true},
		"ipAllowList": map[string]any{
			"before": []any{map[string]any{"cidrBlock": "203.0.113.5/32", "description": "office"}},
			"after":  []any{},
		},
		"parameterOverrides": map[string]any{
			"before": map[string]any{"max_connections": "100"},
			"after":  map[string]any{"max_connections": "200"},
		},
	}, diff)
}

func marshalJSONMap(t *testing.T, v any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(v)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body), "expected object JSON, got: %s", string(raw))
	return body
}
