package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	clientjob "github.com/renderinc/cli/pkg/client/jobs"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/text"
	"github.com/renderinc/cli/pkg/tui/views"
)

var jobCreateCmd = &cobra.Command{
	Use:   "create [serviceID]",
	Short: "Create a new job for a service",
	Args:  cobra.MaximumNArgs(1),
}

var InteractiveJobCreate = func(ctx context.Context, input *views.JobCreateInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		jobCreateCmd,
		breadcrumb,
		input,
		views.NewJobCreateView(ctx, input, jobCreateCmd, func(j *clientjob.Job) tea.Cmd {
			return InteractiveLogs(ctx, views.LogInput{
				ResourceIDs: []string{j.Id},
				Tail:        true,
			}, "Logs")
		}),
	)
}

func interactiveJobCreate(cmd *cobra.Command, input *views.JobCreateInput) tea.Cmd {
	ctx := cmd.Context()
	if input.ServiceID == "" {
		return command.AddToStackFunc(
			ctx,
			cmd,
			"Create Job",
			input,
			views.NewServiceList(ctx, views.ServiceInput{
				Types: []client.ServiceType{client.WebService, client.BackgroundWorker, client.PrivateService, client.CronJob},
			}, func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceID = r.ID()
				return InteractiveJobCreate(ctx, input, resource.BreadcrumbForResource(r))
			}),
		)
	}

	service, err := resource.GetResource(ctx, input.ServiceID)
	if err != nil {
		command.Fatal(cmd, err)
	}

	return InteractiveJobCreate(ctx, input, "Create Job for "+resource.BreadcrumbForResource(service))
}

func init() {
	jobCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.JobCreateInput

		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() (*clientjob.Job, error) {
			return views.CreateJob(cmd.Context(), input)
		}, func(j *clientjob.Job) string {
			return text.FormatStringF("Created job %s for %s", j.Id, input.ServiceID)
		}); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		interactiveJobCreate(cmd, &input)
		return nil
	}

	jobCreateCmd.Flags().String("start-command", "", "The command to run for the job")
	jobCreateCmd.Flags().String("plan-id", "", "The plan ID for the job (optional)")
}
