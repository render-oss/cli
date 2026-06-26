package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
)

func executePGCreate(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	return executePGCommand(t, server, append([]string{"pg", "create"}, extraArgs...)...)
}

func TestPGCreate_ZeroFlags_AppliesDefaults(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executePGCreate(t, server, "--output", "text")
	require.NoError(t, err)

	require.Len(t, server.Postgres.Instances, 1)
	pg := server.Postgres.Instances[0]
	assert.NotEmpty(t, pg.Name)
	assert.Equal(t, pgclient.Free, pg.Plan)
	assert.Equal(t, client.PostgresVersion("18"), pg.Version)
	assert.Equal(t, pgActiveWorkspaceID, pg.Owner.Id)
	assert.Equal(t, client.Oregon, pg.Region)
	assert.Contains(t, result.Stdout, pg.Name)
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGCreate_AllFlags(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executePGCreate(t, server,
		"--name", "analytics",
		"--plan", "pro_4gb",
		"--version", "17",
		"--region", "ohio",
		"--database-name", "metrics",
		"--database-user", "metrics_user",
		"--disk-size-gb", "105",
		"--disk-autoscaling",
		"--high-availability",
		"--datadog-api-key", "dd-key",
		"--datadog-site", "US3",
		"--ip-allow-list", "cidr=10.0.0.0/8,description=internal",
		"--read-replica", "analytics-replica-1",
		"--output", "text",
	)
	require.NoError(t, err)

	require.Len(t, server.Postgres.Instances, 1)
	pg := server.Postgres.Instances[0]
	assert.Equal(t, "analytics", pg.Name)
	assert.Equal(t, pgclient.Pro4gb, pg.Plan)
	assert.Equal(t, client.PostgresVersion("17"), pg.Version)
	assert.Equal(t, client.Region("ohio"), pg.Region)
	assert.Equal(t, "metrics", pg.DatabaseName)
	assert.Equal(t, "metrics_user", pg.DatabaseUser)
	require.NotNil(t, pg.DiskSizeGB)
	assert.Equal(t, 105, *pg.DiskSizeGB)
	assert.True(t, pg.DiskAutoscalingEnabled)
	assert.True(t, pg.HighAvailabilityEnabled)
	require.Len(t, pg.IpAllowList, 1)
	assert.Equal(t, "10.0.0.0/8", pg.IpAllowList[0].CidrBlock)
	assert.Equal(t, "internal", pg.IpAllowList[0].Description)
	require.Len(t, pg.ReadReplicas, 1)
	assert.Equal(t, "analytics-replica-1", pg.ReadReplicas[0].Name)
	assert.Nil(t, pg.ParameterOverrides)
	assert.Contains(t, result.Stdout, "analytics")
	assert.Contains(t, result.Stdout, "Read replicas:")
}

func TestPGCreate_OutputJSON_IsMachineReadable(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	result, err := executePGCreate(t, server,
		"--name", "my-pg",
		"--project", "My Project",
		"--output", "json",
	)
	require.NoError(t, err)

	body := unmarshalPGJSONOutput(t, result.Stdout)
	data := testrequire.SubMap(t, body, "data")
	assert.Equal(t, "my-pg", data["name"])
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, project.Project.Id, data["projectId"])
	assert.Equal(t, project.Env("production").Id, data["environmentId"])
	testrequire.SubSlice(t, data, "ipAllowList")
	testrequire.SubSlice(t, data, "readReplicas")
}

// Verifies that --confirm in interactive output mode creates without launching the wizard.
func TestPGCreate_InteractiveConfirm_PrintsTextSuccess(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executePGCreate(t, server,
		"--confirm",
		"--name", "confirm-pg",
		"--plan", "basic_256mb",
		"--version", "17",
		"--region", "oregon",
		"--output", "interactive",
	)
	require.NoError(t, err)

	require.Len(t, server.Postgres.Instances, 1)
	pg := server.Postgres.Instances[0]
	assert.Equal(t, "confirm-pg", pg.Name)
	assert.Equal(t, pgclient.Basic256mb, pg.Plan)
	assert.Equal(t, client.PostgresVersion("17"), pg.Version)
	assert.Equal(t, client.Oregon, pg.Region)
	assert.Contains(t, result.Stdout, "Created Postgres database")
	assert.Contains(t, result.Stdout, "confirm-pg")
	assert.Contains(t, result.Stdout, pg.Id)
}

func TestPGCreate_PostgresCommandName(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	_, err := executePGCommand(t, server,
		"postgres", "create",
		"--name", "alias-pg",
		"--output", "text",
	)
	require.NoError(t, err)

	require.Len(t, server.Postgres.Instances, 1)
	assert.Equal(t, "alias-pg", server.Postgres.Instances[0].Name)
}

func TestPGCreate_ParameterOverrideFlagIsUnknown(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executePGCreate(t, server,
		"--parameter-override", "max_connections=111",
		"--output", "text",
	)
	require.Error(t, err)

	assert.Contains(t, result.Stderr, "unknown flag: --parameter-override")
	assert.Empty(t, server.Postgres.Instances)
}

func TestPGCreate_NoWorkspaceConfigured(t *testing.T) {
	server := renderapi.NewServer(t)
	result, err := executePGCommand(t, server, "pg", "create", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, result.Stderr, "no workspace")
	assert.Empty(t, server.Postgres.Instances)
}

func TestPGCreate_ProjectFlag_SingleEnv(t *testing.T) {
	server := renderapi.NewServer(t)
	project := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)

	_, err := executePGCreate(t, server,
		"--project", "My Project",
		"--output", "text",
	)
	require.NoError(t, err)

	require.Len(t, server.Postgres.Instances, 1)
	require.NotNil(t, server.Postgres.Instances[0].EnvironmentId)
	assert.Equal(t, project.Env("production").Id, *server.Postgres.Instances[0].EnvironmentId)
}
