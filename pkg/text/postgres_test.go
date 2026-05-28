package text_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/text"
)

func basicPostgres() *client.PostgresDetail {
	return &client.PostgresDetail{
		Id:           "dpg-abc123",
		Name:         "my-pg",
		Plan:         pgclient.Free,
		Version:      "18",
		Region:       client.Oregon,
		Status:       client.DatabaseStatusAvailable,
		DatabaseName: "appdb",
		DatabaseUser: "appuser",
		DashboardUrl: "https://dashboard.render.com/d/dpg-abc123",
		IpAllowList:  []client.CidrBlockAndDescription{},
	}
}

func TestPostgresDetail_BasicFields(t *testing.T) {
	out := text.PostgresDetail(basicPostgres())

	assert.Contains(t, out, "Name: my-pg")
	assert.Contains(t, out, "ID: dpg-abc123")
	assert.Contains(t, out, "Plan: free")
	assert.Contains(t, out, "Version: 18")
	assert.Contains(t, out, "Region: oregon")
	assert.Contains(t, out, "Status: available")
	assert.Contains(t, out, "Database: appdb")
	assert.Contains(t, out, "User: appuser")
	assert.Contains(t, out, "Dashboard: https://dashboard.render.com/d/dpg-abc123")
}

func TestPostgresDetail_OptionalFields(t *testing.T) {
	t.Run("omits disk size and environment ID when unset", func(t *testing.T) {
		out := text.PostgresDetail(basicPostgres())
		assert.NotContains(t, out, "Disk size:")
		assert.NotContains(t, out, "Environment ID:")
	})

	t.Run("includes disk size when set", func(t *testing.T) {
		pg := basicPostgres()
		pg.DiskSizeGB = pointers.From(100)
		assert.Contains(t, text.PostgresDetail(pg), "Disk size: 100 GB")
	})

	t.Run("includes environment ID when set", func(t *testing.T) {
		pg := basicPostgres()
		pg.EnvironmentId = pointers.From("evm-123")
		assert.Contains(t, text.PostgresDetail(pg), "Environment ID: evm-123")
	})
}

func TestPostgresDetail_BoolLabels(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		out := text.PostgresDetail(basicPostgres())
		assert.Contains(t, out, "Disk autoscaling: disabled")
		assert.Contains(t, out, "High availability: disabled")
	})

	t.Run("enabled when true", func(t *testing.T) {
		pg := basicPostgres()
		pg.DiskAutoscalingEnabled = true
		pg.HighAvailabilityEnabled = true
		out := text.PostgresDetail(pg)
		assert.Contains(t, out, "Disk autoscaling: enabled")
		assert.Contains(t, out, "High availability: enabled")
	})
}

func TestPostgresDetail_ReadReplicas(t *testing.T) {
	t.Run("omits the block when no replicas", func(t *testing.T) {
		assert.NotContains(t, text.PostgresDetail(basicPostgres()), "Read replicas")
	})

	t.Run("lists name and ID per replica", func(t *testing.T) {
		pg := basicPostgres()
		pg.ReadReplicas = client.ReadReplicas{
			{Id: "dpg-rep1", Name: "replica-1"},
			{Id: "dpg-rep2", Name: "replica-2"},
		}
		out := text.PostgresDetail(pg)
		assert.Contains(t, out, "Read replicas:")
		assert.Contains(t, out, "- replica-1 (dpg-rep1)")
		assert.Contains(t, out, "- replica-2 (dpg-rep2)")
	})
}

func TestPostgresDetail_IPAllowList(t *testing.T) {
	t.Run("renders empty allow-list as (empty)", func(t *testing.T) {
		assert.Contains(t, text.PostgresDetail(basicPostgres()), "IP allow-list: (empty)")
	})

	t.Run("renders populated entries", func(t *testing.T) {
		pg := basicPostgres()
		pg.IpAllowList = []client.CidrBlockAndDescription{
			{CidrBlock: "10.0.0.0/8", Description: "internal"},
			{CidrBlock: "203.0.113.5/32"},
		}
		out := text.PostgresDetail(pg)
		assert.Contains(t, out, "10.0.0.0/8 (internal)")
		assert.Contains(t, out, "203.0.113.5/32")
	})
}

func TestPostgresTable(t *testing.T) {
	out := text.PostgresTable([]*postgres.Model{{
		Postgres: &client.Postgres{
			Id:     "dpg-table",
			Name:   "table-pg",
			Plan:   pgclient.Basic256mb,
			Region: client.Oregon,
			Status: client.DatabaseStatusAvailable,
		},
		Project:     &client.Project{Name: "Project A"},
		Environment: &client.Environment{Name: "production"},
	}})

	assert.Contains(t, out, "table-pg")
	assert.Contains(t, out, "Project A")
	assert.Contains(t, out, "production")
	assert.Contains(t, out, "basic_256mb")
	assert.Contains(t, out, "dpg-table")
}

func TestPostgresTable_EmptyState(t *testing.T) {
	out := text.PostgresTable([]*postgres.Model{})

	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "No Postgres databases found.")
}

func TestPostgresGetDetail_ConnectionInfo(t *testing.T) {
	pg := basicPostgres()
	conn := &client.PostgresConnectionInfo{
		PsqlCommand:              "PGPASSWORD=secret psql postgres://internal",
		InternalConnectionString: "postgres://internal",
		ExternalConnectionString: "postgres://external",
		Password:                 "secret",
	}

	out := text.PostgresGetDetail(pg, conn)

	assert.Contains(t, out, "Name: my-pg")
	assert.Contains(t, out, "PSQL:")
	assert.Contains(t, out, "Internal: postgres://internal")
	assert.Contains(t, out, "External: postgres://external")
	assert.Contains(t, out, "Password: secret")
}
