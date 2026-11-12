package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui"
)

type SandboxListInput struct {
	SandboxGroupID string   `cli:"group"`
	Name           string   `cli:"name"`
	Status         []string `cli:"status"`
	All            bool     `cli:"all"`
}

func newSandboxListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sandboxes",
		Long: `List sandboxes in your workspace.

In a terminal, this opens an interactive list. Use -o text for a plain table.

By default, terminated sandboxes are excluded. Use --all to include them, or --status to filter by specific statuses.

Examples:
  render sandboxes list
  render sandboxes list --all
  render sandboxes list --name=my-agent
  render sandboxes list --status=running
  render sandboxes list --status=running --status=creating
  render sandboxes list -o text
  render sandboxes list -o json
`,
	}

	cmd.Flags().StringArray("status", nil, "Filter by status (repeatable: "+strings.Join(sandboxStatusNames(), ", ")+")")
	cmd.Flags().Bool("all", false, "Include terminated sandboxes")
	cmd.Flags().String("name", "", "Filter by exact sandbox name. Names aren't unique, so this can match more than one sandbox")
	setFlagPlaceholder(cmd.Flags(), "name", "NAME")
	cmd.Flags().String("group", "", "Filter by sandbox group ID")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")
	setFlagAlias(cmd.Flags(), "sandbox-group", "group")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input SandboxListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		listInput := sandbox.ListInput{
			SandboxGroupID: input.SandboxGroupID,
			Name:           input.Name,
			Statuses:       input.Status,
			All:            input.All,
		}
		nonInteractive, err := command.NonInteractive(cmd, func() ([]*sandboxclient.Sandbox, error) {
			return deps.SandboxService().List(cmd.Context(), listInput)
		}, text.SandboxTable)
		if err != nil || nonInteractive {
			return err
		}

		ctx := cmd.Context()
		command.AddToStackFunc(ctx, cmd, "Sandboxes", &input, tui.NewList(
			"",
			command.PaginatedLoadCmd(ctx, deps.SandboxService().ListPage, listInput),
			func(s *sandboxclient.Sandbox) tui.ListItem { return sandbox.NewListItem(s) },
		))
		return nil
	}

	return cmd
}

func sandboxStatusNames() []string {
	statuses := sandbox.AllStatuses()
	out := make([]string, len(statuses))
	for i, s := range statuses {
		out[i] = string(s)
	}
	return out
}
