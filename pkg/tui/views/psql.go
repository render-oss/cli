package views

import (
	"context"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/tui"
)

type PSQLTool string

const PSQL PSQLTool = "psql"
const PGCLI PSQLTool = "pgcli"

type PSQLInput struct {
	PostgresID     string `cli:"arg:0"`
	Project        *client.Project
	EnvironmentIDs []string
	Tool           PSQLTool
}

type PSQLView struct {
	postgresTable *PostgresList
	execModel     *tui.ExecModel
}

func NewPSQLView(ctx context.Context, input *PSQLInput, opts ...tui.TableOption[*postgres.Model]) *PSQLView {
	psqlView := &PSQLView{
		execModel: tui.NewExecModel(command.LoadCmd(ctx, loadDataPSQL, input)),
	}

	if input.PostgresID == "" {
		// If a flag or temporary input is provided, that should take precedence. Only get the persistent filter
		// if no input is provided.
		if input.EnvironmentIDs == nil {
			defaultInput, err := DefaultListResourceInput(ctx)
			if err != nil {
				return &PSQLView{
					execModel: tui.NewExecModel(command.LoadCmd(ctx, func(_ context.Context, _ any) (*exec.Cmd, error) {
						return nil, fmt.Errorf("failed to load default project filter: %w", err)
					}, nil)),
				}
			}

			input.Project = defaultInput.Project
			input.EnvironmentIDs = defaultInput.EnvironmentIDs
		}

		if input.Project != nil {
			opts = append(opts, tui.WithHeader[*postgres.Model](
				fmt.Sprintf("Project: %s", input.Project.Name),
			))
		}

		psqlView.postgresTable = NewPostgresList(ctx, func(ctx context.Context, p *postgres.Model) tea.Cmd {
			return tea.Sequence(
				func() tea.Msg {
					input.PostgresID = p.ID()
					psqlView.postgresTable = nil
					return nil
				}, psqlView.execModel.Init())
		}, PostgresInput{EnvironmentIDs: input.EnvironmentIDs}, opts...)
	}
	return psqlView
}

func loadDataPSQL(ctx context.Context, in *PSQLInput) (*exec.Cmd, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	connectionInfo, err := postgres.NewRepo(c).GetPostgresConnectionInfo(ctx, in.PostgresID)
	if err != nil {
		return nil, err
	}

	return exec.Command(string(in.Tool), connectionInfo.ExternalConnectionString), nil
}

func (v *PSQLView) Init() tea.Cmd {
	if v.postgresTable != nil {
		return v.postgresTable.Init()
	}

	return v.execModel.Init()
}

func (v *PSQLView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if v.postgresTable != nil {
		_, cmd = v.postgresTable.Update(msg)
	} else {
		_, cmd = v.execModel.Update(msg)
	}

	return v, cmd
}

func (v *PSQLView) View() string {
	if v.postgresTable != nil {
		return v.postgresTable.View()
	}

	return v.execModel.View()
}
