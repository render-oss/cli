package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxCreateInput struct {
	Plan    string `cli:"plan"`
	Region  string `cli:"region"`
	Timeout int    `cli:"timeout"`
}

func (i *SandboxCreateInput) Validate(_ bool) error {
	if i.Plan == "" {
		return nil
	}
	if !sandboxclient.SandboxPlan(i.Plan).Valid() {
		return fmt.Errorf("invalid plan %q: use %s", i.Plan, strings.Join(sandboxPlanNames(), ", "))
	}
	return nil
}

// sandboxPlanNames returns the valid sandbox plan names, sourced from the
// generated SandboxPlanValues() so help and error text stay in sync with the
// schema. SandboxPlan has no "custom" sentinel, so no filtering is needed.
func sandboxPlanNames() []string {
	plans := sandboxclient.SandboxPlanValues()
	out := make([]string, len(plans))
	for i, p := range plans {
		out[i] = string(p)
	}
	return out
}

func newSandboxCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new sandbox",
		Long: `Create a new sandbox in the current workspace.

Examples:
  render ea sandboxes create
  render ea sandboxes create --plan=standard --region=oregon
  render ea sandboxes create --timeout=3600
`,
	}

	cmd.Flags().String("plan", "", "Compute plan: "+strings.Join(sandboxPlanNames(), ", "))
	cmd.Flags().String("region", "", "Region to run the sandbox in")
	cmd.Flags().Int("timeout", 0, "Maximum sandbox lifetime in seconds")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		// Stream status updates to stderr only for text output, so JSON/YAML
		// consumers get a single clean payload on stdout.
		var onEvent func(*sandboxclient.Sandbox)
		if format := command.GetFormatFromContext(cmd.Context()); format != nil && *format == command.TEXT {
			onEvent = func(sb *sandboxclient.Sandbox) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %s %s\n", sb.Id, sb.Status)
			}
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxclient.Sandbox, error) {
			return deps.SandboxService().Create(cmd.Context(), sandbox.CreateInput{
				Plan:    input.Plan,
				Region:  input.Region,
				Timeout: input.Timeout,
			}, onEvent)
		}, text.SandboxDetail)
		return err
	}

	return cmd
}
