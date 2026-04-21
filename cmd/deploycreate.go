package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/deploy"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/render-oss/cli/pkg/types"
)

var deployCmd = &cobra.Command{
	Use:     "deploys",
	Short:   "List, create, and cancel deploys",
	GroupID: GroupCore.ID,
	Example: `  # List deploys for a service
  render deploys list srv-abc123

  # Trigger a deploy for a service
  render deploys create srv-abc123`,
}

var deployCreateCmd = &cobra.Command{
	Use:   "create [serviceID]",
	Short: "Trigger a service deploy and stream logs in real time",
	Args:  cobra.MaximumNArgs(1),
	Example: `  # Trigger a deploy for a service
  render deploys create srv-abc123

  # Deploy a specific commit
  render deploys create srv-abc123 --commit 0123abcd

  # Wait until deploy completes
  render deploys create srv-abc123 --wait`,
}

var InteractiveDeployCreate = func(ctx context.Context, input types.DeployInput, breadcrumb string) tea.Cmd {
	deps := dependencies.GetFromContext(ctx)
	return command.AddToStackFunc(
		ctx,
		deployCreateCmd,
		breadcrumb,
		&input,
		views.NewDeployCreateView(ctx, input, func(d *client.Deploy) tea.Cmd {
			return flows.NewLogFlow(deps).TailLogsFlow(ctx, input.ServiceID)
		}))
}

func interactiveDeployCreate(cmd *cobra.Command, input types.DeployInput) tea.Cmd {
	ctx := cmd.Context()
	if input.ServiceID == "" {
		return command.AddToStackFunc(
			ctx,
			cmd,
			"Create Deploy",
			&input,
			views.NewServiceList(ctx, views.ServiceInput{}, func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceID = r.ID()
				return InteractiveDeployCreate(ctx, input, resource.BreadcrumbForResource(r))
			}),
		)
	}

	service, err := resource.GetResource(ctx, input.ServiceID)
	if err != nil {
		command.Fatal(cmd, err)
	}

	return InteractiveDeployCreate(ctx, input, "Create Deploy for "+resource.BreadcrumbForResource(service))
}

func init() {
	deployCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input types.DeployInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		// if wait flag is used, default to non-interactive output
		if input.Wait {
			command.DefaultFormatNonInteractive(cmd)
		}

		nonInteractive := nonInteractiveDeployCreate(cmd, input)
		if nonInteractive {
			return nil
		}

		interactiveDeployCreate(cmd, input)
		return nil
	}

	deployCreateCmd.Flags().Bool("clear-cache", false, "Clear build cache before deploying")
	deployCreateCmd.Flags().String("commit", "", "Deploy the specified commit ID")
	deployCreateCmd.Flags().String("image", "", "Deploy the specified Docker image URL")
	deployCreateCmd.Flags().Bool("wait", false, "Wait for deploy completion and exit non-zero if deploy fails")
	setAnnotationBestEffort(deployCreateCmd.Flags(), "commit", command.FlagPlaceholderAnnotation, []string{"COMMIT_ID"})
	setAnnotationBestEffort(deployCreateCmd.Flags(), "image", command.FlagPlaceholderAnnotation, []string{"IMAGE_URL"})

	deployCmd.AddCommand(deployCreateCmd)
	rootCmd.AddCommand(deployCmd)
}

func nonInteractiveDeployCreate(cmd *cobra.Command, input types.DeployInput) bool {
	var dep *client.Deploy
	createDeploy := func() (*client.Deploy, error) {
		d, err := views.CreateDeploy(cmd.Context(), input)
		if err != nil {
			return nil, err
		}

		if d == nil {
			_, err = fmt.Fprintf(cmd.OutOrStderr(), "Waiting for deploy to be created...\n\n")
			if err != nil {
				return nil, err
			}
			dep, err = views.WaitForDeployCreate(cmd.Context(), input.ServiceID)
			if err != nil {
				return nil, err
			}

			d = dep
		}

		if input.Wait {
			_, err = fmt.Fprintf(cmd.OutOrStderr(), "Waiting for deploy %s to complete...\n\n", d.Id)
			if err != nil {
				return nil, err
			}
			dep, err = views.WaitForDeploy(cmd.Context(), input.ServiceID, d.Id)
			return dep, err
		}

		return d, err
	}

	nonInteractive, err := command.NonInteractiveWithConfirm(cmd, createDeploy, text.Deploy(input.ServiceID), views.DeployCreateConfirm(cmd.Context(), input))
	if err != nil {
		fmt.Fprintf(cmd.OutOrStderr(), "%s\n", err.Error())
		os.Exit(1)
	}
	if !nonInteractive {
		return false
	}

	if input.Wait && !deploy.IsSuccessful(dep.Status) {
		os.Exit(1)
	}

	return nonInteractive
}
