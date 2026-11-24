package cmd

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/render-oss/cli/pkg/tui/views"
)

var projectFilterClearWorkspaceID = testids.WorkspaceID("project-filter-clear")

// datastorePicker launches one interactive datastore picker and exposes the
// project scope its command input holds.
type datastorePicker struct {
	name   string
	seed   func(server *renderapi.Server, name string)
	launch func(ctx context.Context) tea.Cmd
	scope  func() (*client.Project, []string)
}

func datastorePickers() []func() datastorePicker {
	seedPostgres := func(server *renderapi.Server, name string) {
		server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
			Name:  name,
			Owner: client.Owner{Id: projectFilterClearWorkspaceID},
		}))
	}
	seedKeyValue := func(server *renderapi.Server, name string) {
		server.KV.Add(renderapi.NewKV(client.KeyValueDetail{
			Name:  name,
			Owner: client.Owner{Id: projectFilterClearWorkspaceID},
		}))
	}

	return []func() datastorePicker{
		func() datastorePicker {
			input := &views.PSQLInput{}
			return datastorePicker{
				name:   "psql",
				seed:   seedPostgres,
				launch: func(ctx context.Context) tea.Cmd { return InteractivePSQLView(ctx, input) },
				scope:  func() (*client.Project, []string) { return input.Project, input.EnvironmentIDs },
			}
		},
		func() datastorePicker {
			input := &views.PSQLInput{}
			return datastorePicker{
				name:   "pgcli",
				seed:   seedPostgres,
				launch: func(ctx context.Context) tea.Cmd { return InteractivePGCLIView(ctx, input) },
				scope:  func() (*client.Project, []string) { return input.Project, input.EnvironmentIDs },
			}
		},
		func() datastorePicker {
			input := &views.RedisCLIInput{}
			return datastorePicker{
				name:   "kv-cli",
				seed:   seedKeyValue,
				launch: func(ctx context.Context) tea.Cmd { return InteractiveKeyValueCLIView(ctx, input) },
				scope:  func() (*client.Project, []string) { return input.Project, input.EnvironmentIDs },
			}
		},
	}
}

func startDatastorePicker(t *testing.T, picker datastorePicker) *teatest.TestModel {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: projectFilterClearWorkspaceID, Name: "Test Workspace"}))
	server.CreateProject(
		renderapi.ProjectAttrs{Name: "scoped-project", OwnerId: projectFilterClearWorkspaceID},
		renderapi.EnvAttrs{Name: "empty"},
	)
	picker.seed(server, "unassigned-datastore")

	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_WORKSPACE", projectFilterClearWorkspaceID)

	stack := tui.NewStack()
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	ctx := dependencies.SetInContext(context.Background(), dependencies.New(c))
	ctx = tui.SetStackInContext(ctx, stack)
	require.NotNil(t, picker.launch(ctx))

	tm := teatest.NewTestModel(t, stack, teatest.WithInitialTermSize(200, 40))
	t.Cleanup(func() { _ = tm.Quit() })
	return tm
}

var projectFilterClearWait = testhelper.WaitForContainsOptions{Duration: 5 * time.Second}

func TestDatastorePickerFreshProcessShowsWorkspaceDatastore(t *testing.T) {
	for _, newPicker := range datastorePickers() {
		picker := newPicker()
		t.Run(picker.name, func(t *testing.T) {
			tm := startDatastorePicker(t, picker)
			testhelper.WaitForContains(t, tm.Output(), "unassigned-datastore", projectFilterClearWait)

			project, envIDs := picker.scope()
			assert.Nil(t, project)
			assert.Nil(t, envIDs)
		})
	}
}

func TestDatastorePickerClearingProjectFilterRestoresWorkspaceScope(t *testing.T) {
	for _, newPicker := range datastorePickers() {
		picker := newPicker()
		t.Run(picker.name, func(t *testing.T) {
			tm := startDatastorePicker(t, picker)
			out := tm.Output()
			testhelper.WaitForContains(t, out, "unassigned-datastore", projectFilterClearWait)

			// Select the project: its only environment is empty, so the
			// unassigned datastore is excluded from the scoped list.
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
			testhelper.WaitForContains(t, out, "scoped-project", projectFilterClearWait)
			tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
			testhelper.WaitForContains(t, out, "No Results", projectFilterClearWait)

			project, envIDs := picker.scope()
			if picker.name != "kv-cli" {
				require.NotNil(t, project)
				assert.Equal(t, "scoped-project", project.Name)
			}
			assert.Len(t, envIDs, 1)

			// Clear the filter from the project chooser.
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
			testhelper.WaitForContains(t, out, "scoped-project", projectFilterClearWait)
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
			testhelper.WaitForContains(t, out, "unassigned-datastore", projectFilterClearWait)

			project, envIDs = picker.scope()
			assert.Nil(t, project, "cleared filter must drop the previously selected project")
			assert.Nil(t, envIDs, "cleared filter must drop the previously selected environments")

			projectID, projectName, err := config.GetProjectFilter()
			require.NoError(t, err)
			assert.Empty(t, projectID)
			assert.Empty(t, projectName)
		})
	}
}
