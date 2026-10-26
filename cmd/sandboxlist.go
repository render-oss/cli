package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxListInput struct {
	SandboxGroupID string   `cli:"sandbox-group"`
	Status         []string `cli:"status"`
	All            bool     `cli:"all"`
}

func newSandboxListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sandboxes",
		Long: `List sandboxes in your workspace.

By default, terminated sandboxes are excluded. Use --all to include them, or --status to filter by specific statuses.

Examples:
  render ea sandboxes list
  render ea sandboxes list --all
  render ea sandboxes list --status=running
  render ea sandboxes list --status=running --status=creating
  render ea sandboxes list -o json
`,
	}

	cmd.Flags().StringArray("status", nil, "Filter by status (repeatable: "+strings.Join(sandboxStatusNames(), ", ")+")")
	cmd.Flags().Bool("all", false, "Include terminated sandboxes")
	cmd.Flags().String("sandbox-group", "", "Filter by sandbox group ID")
	setFlagPlaceholder(cmd.Flags(), "sandbox-group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() ([]*sandboxclient.Sandbox, error) {
			return deps.SandboxService().List(cmd.Context(), input.Status, input.All, input.SandboxGroupID)
		}, text.SandboxTable)
		return err
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
