package views

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/redis"
	"github.com/render-oss/cli/pkg/tui"
)

type RedisCLIInput struct {
	RedisID        string `cli:"arg:0"`
	Project        *client.Project
	EnvironmentIDs []string

	Args []string
}

type RedisCLIView struct {
	redisTable *RedisList
	execModel  *tui.ExecModel
}

func NewRedisCLIView(ctx context.Context, input *RedisCLIInput, opts ...tui.TableOption[*redis.Model]) *RedisCLIView {
	psqlView := &RedisCLIView{
		execModel: tui.NewExecModel(command.LoadCmd(ctx, loadDataRedisCLI, input)),
	}

	if input.RedisID == "" {
		// If a flag or temporary input is provided, that should take precedence. Only get the persistent filter
		// if no input is provided.
		if input.EnvironmentIDs == nil {
			defaultInput, err := DefaultListResourceInput(ctx)
			if err != nil {
				return &RedisCLIView{
					execModel: tui.NewExecModel(command.LoadCmd(ctx, func(_ context.Context, _ any) (*exec.Cmd, error) {
						return nil, fmt.Errorf("failed to load default project filter: %w", err)
					}, nil)),
				}
			}

			input.Project = defaultInput.Project
			input.EnvironmentIDs = defaultInput.EnvironmentIDs
		}

		if input.Project != nil {
			opts = append(opts, tui.WithHeader[*redis.Model](
				fmt.Sprintf("Project: %s", input.Project.Name),
			))
		}

		psqlView.redisTable = NewRedisList(ctx, func(ctx context.Context, p *redis.Model) tea.Cmd {
			return tea.Sequence(
				func() tea.Msg {
					input.RedisID = p.ID()
					psqlView.redisTable = nil
					return nil
				}, psqlView.execModel.Init())
		}, RedisInput{EnvironmentIDs: input.EnvironmentIDs}, opts...)
	}
	return psqlView
}

func loadDataRedisCLI(ctx context.Context, in *RedisCLIInput) (*exec.Cmd, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	connectionInfo, err := redis.NewRepo(c).GetRedisConnectionInfo(ctx, in.RedisID)
	if err != nil {
		return nil, err
	}

	rawCmd := connectionInfo.RedisCLICommand
	cmdParts := strings.Split(rawCmd, " ")
	var env []string
	var cmdArgs []string
	var pastRedisCLI bool
	for _, part := range cmdParts {
		if part == "redis-cli" {
			pastRedisCLI = true
			continue
		}

		if pastRedisCLI {
			cmdArgs = append(cmdArgs, part)
		} else {
			env = append(env, part)
		}
	}

	for _, arg := range in.Args {
		cmdArgs = append(cmdArgs, arg)
	}

	cmd := exec.Command("redis-cli", cmdArgs...)
	cmd.Env = env
	return cmd, nil
}

func (v *RedisCLIView) Init() tea.Cmd {
	if v.redisTable != nil {
		return v.redisTable.Init()
	}

	return v.execModel.Init()
}

func (v *RedisCLIView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if v.redisTable != nil {
		_, cmd = v.redisTable.Update(msg)
	} else {
		_, cmd = v.execModel.Update(msg)
	}

	return v, cmd
}

func (v *RedisCLIView) View() string {
	if v.redisTable != nil {
		return v.redisTable.View()
	}

	return v.execModel.View()
}
