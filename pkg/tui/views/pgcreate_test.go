package views

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

func TestPostgresCreateViewShowsCanceledWhenConfirmDeclined(t *testing.T) {
	m := NewPostgresCreateModel(context.Background(), PostgresCreateRepos{}, pgtypes.CreatePostgresInput{})
	m.currentStep = len(pgCreateSteps) - 1
	m.confirmValue = false

	next, cmd := m.onFormCompleted()

	require.NotNil(t, cmd)
	view := next.(*PostgresCreateModel).View()
	assert.Contains(t, view, "Canceled.")
}

func TestPostgresCreateRequestInputPreservesFlagOnlyFields(t *testing.T) {
	diskSize := 25
	databaseName := "app"
	databaseUser := "app_user"
	datadogAPIKey := "dd-key"
	datadogSite := "US3"
	m := NewPostgresCreateModel(context.Background(), PostgresCreateRepos{}, pgtypes.CreatePostgresInput{
		DiskSizeGB:         &diskSize,
		DatabaseName:       &databaseName,
		DatabaseUser:       &databaseUser,
		DatadogAPIKey:      &datadogAPIKey,
		DatadogSite:        &datadogSite,
		IPAllowList:        []string{"cidr=10.0.0.0/8,description=internal"},
		ParameterOverrides: []string{"max_connections=111"},
		ReadReplicas:       []string{"analytics-replica-1"},
	})
	m.draft = pgCreateDraft{
		workspaceID: "tea-123",
		name:        "my-db",
		plan:        "pro_4gb",
		version:     18,
		region:      "oregon",
	}

	reqInput := m.createRequestInput()

	assert.Equal(t, &diskSize, reqInput.DiskSizeGB)
	assert.Equal(t, &databaseName, reqInput.DatabaseName)
	assert.Equal(t, &databaseUser, reqInput.DatabaseUser)
	assert.Equal(t, &datadogAPIKey, reqInput.DatadogAPIKey)
	assert.Equal(t, &datadogSite, reqInput.DatadogSite)
	assert.Equal(t, []string{"cidr=10.0.0.0/8,description=internal"}, reqInput.IPAllowList)
	assert.Equal(t, []string{"max_connections=111"}, reqInput.ParameterOverrides)
	assert.Equal(t, []string{"analytics-replica-1"}, reqInput.ReadReplicas)
}

func TestPostgresCreateWizardHappyPathCreatesDatabase(t *testing.T) {
	ctx := context.Background()
	server := renderapi.NewServer(t)
	workspace := server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-workspace-123", Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", workspace.Id)
	seededProject := server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: workspace.Id},
		renderapi.EnvAttrs{Name: "Staging"},
	)

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	m := NewPostgresCreateModel(ctx, PostgresCreateRepos{
		Owners:   owner.NewRepo(c),
		Projects: project.NewRepo(c),
		Envs:     environment.NewRepo(c),
		Postgres: postgres.NewRepo(c),
	}, pgtypes.CreatePostgresInput{})
	tm := teatest.NewTestModel(t, m)
	t.Cleanup(func() { _ = tm.Quit() })

	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 40})

	testhelper.WaitForContains(t, tm.Output(), "Workspace")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // active workspace

	testhelper.WaitForContains(t, tm.Output(), "Project")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // My Project

	testhelper.WaitForContains(t, tm.Output(), "Environment")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // Staging

	testhelper.WaitForContains(t, tm.Output(), "Name")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("web-app-db")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	testhelper.WaitForContains(t, tm.Output(), "Plan")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // free

	testhelper.WaitForContains(t, tm.Output(), "Postgres Version")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // 18

	testhelper.WaitForContains(t, tm.Output(), "Region")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // oregon

	testhelper.WaitForContains(t, tm.Output(), "Enable Disk Autoscaling?")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // no

	testhelper.WaitForContains(t, tm.Output(), "Create this Postgres instance?")
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // yes

	testhelper.WaitForContains(t, tm.Output(), "render ea pg get web-app-db")

	require.Len(t, server.Postgres.Instances, 1)
	pg := server.Postgres.Instances[0]
	assert.Equal(t, "web-app-db", pg.Name)
	assert.Equal(t, workspace.Id, pg.Owner.Id)
	require.NotNil(t, pg.EnvironmentId)
	assert.Equal(t, seededProject.Env("Staging").Id, *pg.EnvironmentId)
	assert.Equal(t, "free", string(pg.Plan))
	assert.Equal(t, client.PostgresVersion("18"), pg.Version)
	assert.Equal(t, client.Oregon, pg.Region)
	assert.False(t, pg.HighAvailabilityEnabled)
	assert.False(t, pg.DiskAutoscalingEnabled)
	assert.Nil(t, pg.DiskSizeGB)
}
