package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/redis"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/views"
)

// redisCLICmd represents the redisCLI command
var redisCLICmd = &cobra.Command{
	Use:   "redis-cli [redisID]",
	Short: "Open a redis-cli session to a Redis instance",
	Long: `Open a redis-cli session to a Redis instance. Optionally pass the redis id as an argument.
To pass arguments to redis-cli, use the following syntax: render redis-cli [redisID] -- [redis-cli args]`,
	GroupID: GroupSession.ID,
}

func InteractiveRedisView(ctx context.Context, input *views.RedisCLIInput) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		redisCLICmd,
		"redis-cli",
		input,
		views.NewRedisCLIView(ctx, input, tui.WithCustomOptions[*redis.Model](getRedisTableOptions(ctx))),
	)
}

func getRedisTableOptions(ctx context.Context) []tui.CustomOption {
	return []tui.CustomOption{
		WithCopyID(ctx, servicesCmd),
		WithWorkspaceSelection(ctx),
		WithProjectFilter(ctx, redisCLICmd, "redisCLI", &views.RedisCLIInput{}, func(ctx context.Context, project *client.Project) tea.Cmd {
			input := &views.RedisCLIInput{}
			if project != nil {
				input.EnvironmentIDs = project.EnvironmentIds
			}
			return InteractiveRedisView(ctx, input)
		}),
	}
}

func init() {
	rootCmd.AddCommand(redisCLICmd)

	redisCLICmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var input views.RedisCLIInput
		err := command.ParseCommandInteractiveOnly(cmd, args, &input)
		if err != nil {
			return err
		}

		if cmd.ArgsLenAtDash() == 0 {
			input.RedisID = ""
		}

		if cmd.ArgsLenAtDash() >= 0 {
			input.Args = args[cmd.ArgsLenAtDash():]
		}

		InteractiveRedisView(ctx, &input)
		return nil
	}
}
