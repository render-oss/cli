package text_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/render-oss/cli/internal/testassert"
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
	out := text.PostgresAPIDetail(basicPostgres())

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
		out := text.PostgresAPIDetail(basicPostgres())
		assert.NotContains(t, out, "Disk size:")
		assert.NotContains(t, out, "Environment ID:")
	})

	t.Run("includes disk size when set", func(t *testing.T) {
		pg := basicPostgres()
		pg.DiskSizeGB = pointers.From(100)
		assert.Contains(t, text.PostgresAPIDetail(pg), "Disk size: 100 GB")
	})

	t.Run("includes environment ID when set", func(t *testing.T) {
		pg := basicPostgres()
		pg.EnvironmentId = pointers.From("evm-123")
		assert.Contains(t, text.PostgresAPIDetail(pg), "Environment ID: evm-123")
	})
}

func TestPostgresDetail_BoolLabels(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		out := text.PostgresAPIDetail(basicPostgres())
		assert.Contains(t, out, "Disk autoscaling: disabled")
		assert.Contains(t, out, "High availability: disabled")
	})

	t.Run("enabled when true", func(t *testing.T) {
		pg := basicPostgres()
		pg.DiskAutoscalingEnabled = true
		pg.HighAvailabilityEnabled = true
		out := text.PostgresAPIDetail(pg)
		assert.Contains(t, out, "Disk autoscaling: enabled")
		assert.Contains(t, out, "High availability: enabled")
	})
}

func TestPostgresDetail_ReadReplicas(t *testing.T) {
	t.Run("omits the block when no replicas", func(t *testing.T) {
		assert.NotContains(t, text.PostgresAPIDetail(basicPostgres()), "Read replicas")
	})

	t.Run("lists name and ID per replica", func(t *testing.T) {
		pg := basicPostgres()
		pg.ReadReplicas = client.ReadReplicas{
			{Id: "dpg-rep1", Name: "replica-1"},
			{Id: "dpg-rep2", Name: "replica-2"},
		}
		out := text.PostgresAPIDetail(pg)
		assert.Contains(t, out, "Read replicas:")
		assert.Contains(t, out, "- replica-1 (dpg-rep1)")
		assert.Contains(t, out, "- replica-2 (dpg-rep2)")
	})
}

func TestPostgresDetail_IPAllowList(t *testing.T) {
	t.Run("renders empty allow-list as (empty)", func(t *testing.T) {
		assert.Contains(t, text.PostgresAPIDetail(basicPostgres()), "IP allow-list: (empty)")
	})

	t.Run("renders populated entries", func(t *testing.T) {
		pg := basicPostgres()
		pg.IpAllowList = []client.CidrBlockAndDescription{
			{CidrBlock: "10.0.0.0/8", Description: "internal"},
			{CidrBlock: "203.0.113.5/32"},
		}
		out := text.PostgresAPIDetail(pg)
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

func TestPostgresUpdateDiff(t *testing.T) {
	t.Run("returns empty string when nothing changed", func(t *testing.T) {
		pg := basicPostgres()
		assert.Empty(t, text.PostgresUpdateDiff(pg, pg))
	})

	t.Run("single changed field: name", func(t *testing.T) {
		before := basicPostgres()
		after := basicPostgres()
		after.Name = "new-name"

		out := text.PostgresUpdateDiff(before, after)
		testassert.ContainsInOrder(t, out, "Name:", "my-pg → new-name")
	})

	t.Run("single changed field: plan", func(t *testing.T) {
		before := basicPostgres()
		after := basicPostgres()
		after.Plan = pgclient.Basic256mb

		out := text.PostgresUpdateDiff(before, after)
		testassert.ContainsInOrder(t, out, "Plan:", "free → basic_256mb")
	})

	t.Run("disk size nil to set", func(t *testing.T) {
		before := basicPostgres()
		after := basicPostgres()
		after.DiskSizeGB = pointers.From(100)

		out := text.PostgresUpdateDiff(before, after)
		testassert.ContainsInOrder(t, out, "Disk size:", "(unset) → 100 GB")
	})

	t.Run("bool fields, both directions", func(t *testing.T) {
		before := basicPostgres()
		before.HighAvailabilityEnabled = true
		after := basicPostgres()
		after.DiskAutoscalingEnabled = true
		after.HighAvailabilityEnabled = false

		out := text.PostgresUpdateDiff(before, after)
		// Disk autoscaling renders before high availability, so assert the full
		// layout in order: a field that turned on, then one that turned off.
		testassert.ContainsInOrder(t, out,
			"Disk autoscaling:", "disabled → enabled",
			"High availability:", "enabled → disabled")
	})

	t.Run("IP allow-list change", func(t *testing.T) {
		before := basicPostgres()
		after := basicPostgres()
		after.IpAllowList = []client.CidrBlockAndDescription{
			{CidrBlock: "10.0.0.0/8", Description: "internal"},
		}

		out := text.PostgresUpdateDiff(before, after)
		testassert.ContainsInOrder(t, out, "IP allow-list:", "(empty) → 1 entry")
	})

	t.Run("parameter overrides change", func(t *testing.T) {
		before := basicPostgres()
		after := basicPostgres()
		after.ParameterOverrides = &client.PostgresParameterOverrides{"max_connections": "200"}

		out := text.PostgresUpdateDiff(before, after)
		testassert.ContainsInOrder(t, out, "Parameter overrides:", "updated")
	})

	t.Run("multiple changed fields", func(t *testing.T) {
		before := basicPostgres()
		after := basicPostgres()
		after.Name = "renamed"
		after.HighAvailabilityEnabled = true

		out := text.PostgresUpdateDiff(before, after)
		testassert.ContainsInOrder(t, out, "Name:", "High availability:")

		// Only changed fields appear; untouched fields are omitted.
		assert.NotContains(t, out, "Plan:")
		assert.NotContains(t, out, "Disk size:")
		assert.NotContains(t, out, "Disk autoscaling:")
		assert.NotContains(t, out, "IP allow-list:")
		assert.NotContains(t, out, "Parameter overrides:")
	})
}

func TestPostgresTable_EmptyState(t *testing.T) {
	out := text.PostgresTable([]*postgres.Model{})

	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "No Postgres databases found.")
}

func TestPostgresGetDetail_ConnectionInfo(t *testing.T) {
	out := postgres.NewPostgresGetOut(&postgres.ResolvedPostgres{Postgres: basicPostgres()})
	conn := &client.PostgresConnectionInfo{
		PsqlCommand:              "PGPASSWORD=secret psql postgres://internal",
		InternalConnectionString: "postgres://internal",
		ExternalConnectionString: "postgres://external",
		Password:                 "secret",
	}
	out.Data.ConnectionInfo = conn

	detail := text.PostgresGetDetail(&out.Data)

	assert.Contains(t, detail, "Name: my-pg")
	assert.Contains(t, detail, "PSQL:")
	assert.Contains(t, detail, "Internal: postgres://internal")
	assert.Contains(t, detail, "External: postgres://external")
	assert.Contains(t, detail, "Password: secret")
}
