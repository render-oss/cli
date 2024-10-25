package views

import (
	"context"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/tui"
)

type PSQLInput struct {
	PostgresID string `cli:"arg:0"`
}

type PSQLView struct {
	postgresTable *PostgresList
	execModel     *tui.ExecModel
}

func NewPSQLView(ctx context.Context, input *PSQLInput) *PSQLView {
	psqlView := &PSQLView{
		execModel: tui.NewExecModel(command.LoadCmd(ctx, loadDataPSQL, input)),
	}

	if input.PostgresID == "" {
		psqlView.postgresTable = NewPostgresList(ctx, func(ctx context.Context, p *postgres.Model) tea.Cmd {

			return tea.Sequence(
				func() tea.Msg {
					input.PostgresID = p.ID()
					psqlView.postgresTable = nil
					return nil
				}, psqlView.execModel.Init())
		})
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

	return exec.Command("psql", connectionInfo.ExternalConnectionString), nil
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
