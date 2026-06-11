package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	servicepkg "github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/text"
)

type serviceDeleteInput struct {
	IDOrName string `cli:"arg:0"`
}

func newServiceDeleteCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <serviceID|serviceName>",
		Short:        "Delete a service",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Delete a service on Render.

Without --confirm, this command previews what would be deleted and makes no
changes. Pass --confirm to actually delete the service.

The positional argument accepts a service ID (including srv- or crn- IDs) or a
name. Name lookup is scoped to your active workspace. If the name matches more
than one service, pass the service ID directly.

This command only runs non-interactively. If --output interactive is requested,
it falls back to text output.`,
		Example: `  # Preview deletion (no changes made)
  render services delete srv-abc123def456ghi789jkl0

  # Delete by ID
  render services delete srv-abc123def456ghi789jkl0 --confirm

  # Delete by name
  render services delete my-api --confirm

  # JSON output
  render services delete srv-abc123def456ghi789jkl0 --confirm --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input serviceDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		confirm := command.GetConfirmFromContext(cmd.Context())

		loadData := func() (*servicepkg.DeleteOut, error) {
			if _, err := config.WorkspaceID(); err != nil {
				return nil, err
			}

			repo := deps.ServiceRepo()
			serviceID, err := repo.ResolveServiceIDFromNameOrID(cmd.Context(), input.IDOrName)
			if err != nil {
				return nil, err
			}

			model, err := deps.ServiceService().GetService(cmd.Context(), serviceID)
			if err != nil {
				return nil, err
			}
			out := servicepkg.NewDeleteOutFromModel(model)
			out.Meta = servicepkg.DeleteOutMeta{
				Deleted: confirm,
			}
			if confirm {
				if err := repo.DeleteService(cmd.Context(), model.Service.Id); err != nil {
					return nil, err
				}
			} else {
				out.Meta.Message = "re-run with --confirm to delete"
			}
			return &out, nil
		}

		_, err := command.NonInteractive(cmd, loadData, serviceDeleteTextOutput)
		return err
	}

	return cmd
}

func serviceDeleteTextOutput(r *servicepkg.DeleteOut) string {
	if r.Meta.Deleted {
		return "Deleted this service:\n\n" + text.ServiceDetail(&r.Data) + "\n"
	}
	return "This command would delete this service:\n\n" +
		text.ServiceDetail(&r.Data) +
		"\n\nRe-run with --confirm to proceed\n"
}
