package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
)

// redisCLICmd represents the redisCLI command
var redisCLICmd = &cobra.Command{
	Use:   "kv-cli [keyValueID|keyValueName]",
	Short: "Open a session for a Render Key Value instance",
	Long: `Open a redis-cli or valkey-cli session for a Render Key Value instance. This command only supports interactive mode.

You can optionally pass the key value ID or name as an argument. To pass arguments to redis-cli or valkey-cli, use:
  render kv-cli [keyValueID|keyValueName] -- [redis-cli args]`,
	GroupID: GroupSession.ID,
	Example: `  # Open an interactive kv-cli session
  render kv-cli kv-abc123

  # Pass through redis-cli arguments
  render kv-cli kv-abc123 -- --scan`,
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
		flows.WithCopyID(ctx, servicesCmd),
		flows.WithWorkspaceSelection(ctx),
		flows.WithProjectFilter(ctx, redisCLICmd, "redisCLI", input, func(ctx context.Context, project *client.Project) tea.Cmd {
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
