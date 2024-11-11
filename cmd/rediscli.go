package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/redis"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"
)

// redisCLICmd represents the redisCLI command
var redisCLICmd = &cobra.Command{
	Use:     "redis-cli [redisID]",
	Args:    cobra.MaximumNArgs(1),
	Short:   "Open a redis-cli session to a Redis instance",
	Long:    `Open a redis-cli session to a Redis instance. Optionally pass the redis id as an argument.`,
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

		InteractiveRedisView(ctx, &input)
		return nil
	}
}
