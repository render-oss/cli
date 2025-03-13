package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

// redisCLICmd represents the redisCLI command
var redisCLICmd = &cobra.Command{
	Use:   "kv-cli [keyValueID|keyValueName]",
	Short: "Open a redis-cli or valkey-cli session to a Key Value instance",
	Long: `Open a redis-cli or valkey-cli session to a Key Value instance. Optionally pass the key value id or name as an argument.
To pass arguments to redis-cli or valkey-cli, use the following syntax: render kv-cli [keyValueID|keyValueName] -- [redis-cli args]`,
	GroupID: GroupSession.ID,
}

func InteractiveKeyValueCLIView(ctx context.Context, input *views.RedisCLIInput) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		redisCLICmd,
		"kv-cli",
		input,
		views.NewRedisCLIView(ctx, input, tui.WithCustomOptions[*keyvalue.Model](getRedisTableOptions(ctx, input))),
	)
}

func getRedisTableOptions(ctx context.Context, input *views.RedisCLIInput) []tui.CustomOption {
	return []tui.CustomOption{
		WithCopyID(ctx, servicesCmd),
		WithWorkspaceSelection(ctx),
		WithProjectFilter(ctx, redisCLICmd, "redisCLI", input, func(ctx context.Context, project *client.Project) tea.Cmd {
			if project != nil {
				input.EnvironmentIDs = project.EnvironmentIds
			}
			return InteractiveKeyValueCLIView(ctx, input)
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
			input.RedisIDOrName = ""
		}

		if cmd.ArgsLenAtDash() >= 0 {
			input.Args = args[cmd.ArgsLenAtDash():]
		}

		InteractiveKeyValueCLIView(ctx, &input)
		return nil
	}
}
