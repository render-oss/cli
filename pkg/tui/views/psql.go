package views

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/tui"
)

type PSQLTool string

const PSQL PSQLTool = "psql"
const PGCLI PSQLTool = "pgcli"

type PSQLInput struct {
	PostgresIDOrName string `cli:"arg:0"`
	Project          *client.Project
	EnvironmentIDs   []string
	Tool             PSQLTool

	Args []string
}

type PSQLView struct {
	postgresTable *PostgresList
	execModel     *tui.ExecModel
}

func NewPSQLView(ctx context.Context, input *PSQLInput, opts ...tui.TableOption[*postgres.Model]) *PSQLView {
	psqlView := &PSQLView{
		execModel: tui.NewExecModel(string(input.Tool), handlePSQLError(input.Tool), command.LoadCmd(ctx, loadDataPSQL, input)),
	}

	if input.PostgresIDOrName == "" {
		// If a flag or temporary input is provided, that should take precedence. Only get the persistent filter
		// if no input is provided.
		if input.EnvironmentIDs == nil {
			defaultInput, err := DefaultListResourceInput(ctx)
			if err != nil {
				return &PSQLView{
					execModel: tui.NewExecModel(string(input.Tool), handlePSQLError(input.Tool), command.LoadCmd(ctx, func(_ context.Context, _ any) (*exec.Cmd, error) {
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
					input.PostgresIDOrName = p.ID()
					psqlView.postgresTable = nil
					return nil
				}, psqlView.execModel.Init())
		}, PostgresInput{EnvironmentIDs: input.EnvironmentIDs}, opts...)
	}
	return psqlView
}

func handlePSQLError(tool PSQLTool) func(err error) error {
	return func(err error) error {
		return tui.UserFacingError{
			Title: fmt.Sprintf("An error occurred while running %s", tool),
			Err:   err,
		}
	}
}

func getPostgresFromIDOrName(ctx context.Context, c *client.ClientWithResponses, idOrName string) (*client.PostgresDetail, error) {
	pgc := postgres.NewRepo(c)

	if matchesPostgresId(idOrName) {
		// We can't easily disambiguate between an ID and a name (since technically a name could be
		// a valid ID), so we'll prefer the ID if it's valid.
		postgres, err := pgc.GetPostgres(ctx, idOrName)
		if err == nil {
			return postgres, nil
		}
	}

	postgreses, err := pgc.ListPostgres(ctx, &client.ListPostgresParams{
		Name: &client.NameParam{idOrName},
	})

	if err != nil {
		return nil, err
	}

	if len(postgreses) == 0 {
		return nil, tui.UserFacingError{Message: fmt.Sprintf("No Postgres instance found with name or ID '%s'", idOrName)}
	}

	if len(postgreses) > 1 {
		return nil, tui.UserFacingError{Message: fmt.Sprintf("Multiple Postgres instances found with name '%s'. Please specify the Postgres ID instead.", idOrName)}
	}

	return &client.PostgresDetail{
		Name:        postgreses[0].Name,
		Id:          postgreses[0].Id,
		IpAllowList: postgreses[0].IpAllowList,
	}, nil
}

func loadDataPSQL(ctx context.Context, in *PSQLInput) (*exec.Cmd, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	pgc := postgres.NewRepo(c)

	pg, err := getPostgresFromIDOrName(ctx, c, in.PostgresIDOrName)
	if err != nil {
		return nil, err
	}

	// only check access if error is nil in case ipify is down
	userIP, ok := getUserIP()
	if ok {
		hasAccess, err := hasAccessToPostgres(pg, userIP)
		if err != nil {
			return nil, err
		}

		if !hasAccess {
			return nil, fmt.Errorf("IP address (%s) not in allow list for %s", userIP, pg.Name)
		}
	}

	connectionInfo, err := pgc.GetPostgresConnectionInfo(ctx, pg.Id)
	if err != nil {
		return nil, err
	}

	args := []string{connectionInfo.ExternalConnectionString}
	for _, arg := range in.Args {
		args = append(args, arg)
	}

	return exec.Command(string(in.Tool), args...), nil
}

func hasAccessToPostgres(pg *client.PostgresDetail, userIP net.IP) (bool, error) {
	for _, allowedIPs := range pg.IpAllowList {
		_, cidr, err := net.ParseCIDR(allowedIPs.CidrBlock)
		if err != nil {
			return false, err
		}

		if cidr.Contains(userIP) {
			return true, nil
		}
	}
	return false, nil
}

func getUserIP() (net.IP, bool) {
	userIPRes, err := http.Get("https://api.ipify.org")
	if err != nil {
		return nil, false
	}

	userIPBytes, err := io.ReadAll(userIPRes.Body)
	if err != nil {
		return nil, false
	}

	return net.ParseIP(string(userIPBytes)), true
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
