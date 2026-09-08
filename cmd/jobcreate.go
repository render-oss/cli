package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	logclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/job"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
)

type jobCreateCommandConfig struct {
	pollInterval time.Duration
	newTicker    func(time.Duration) job.TickSource
}

var JobCreateCmd = newJobCreateCmd()

var InteractiveJobCreate = func(ctx context.Context, input *views.JobCreateInput, breadcrumb string) tea.Cmd {
	return interactiveJobCreateWithCommand(ctx, input, breadcrumb, JobCreateCmd)
}

func interactiveJobCreateWithCommand(ctx context.Context, input *views.JobCreateInput, breadcrumb string, cobraCmd *cobra.Command) tea.Cmd {
	deps := dependencies.GetFromContext(ctx)
	return command.AddToStackFunc(
		ctx,
		cobraCmd,
		breadcrumb,
		input,
		views.NewJobCreateView(ctx, input, cobraCmd, views.CreateJob, func(j *clientjob.Job) tea.Cmd {
			return flows.NewLogFlow(deps).LogsFlow(ctx, views.LogInput{
				ResourceIDs: []string{j.Id},
				Tail:        true,
			})
		}),
	)
}

func interactiveJobCreate(cmd *cobra.Command, input *views.JobCreateInput) (tea.Cmd, error) {
	ctx := cmd.Context()
	if input.ServiceID == "" {
		return command.AddToStackFunc(
			ctx,
			cmd,
			"Create Job",
			input,
			views.NewServiceList(ctx, views.ServiceInput{
				Types: []client.ServiceType{
					client.WebService, client.BackgroundWorker, client.PrivateService, client.CronJob,
				},
			}, func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceID = r.ID()
				return interactiveJobCreateWithCommand(ctx, input, resource.BreadcrumbForResource(r), cmd)
			}),
		), nil
	}

	service, err := resource.GetResource(ctx, input.ServiceID)
	if err != nil {
		return nil, err
	}

	return interactiveJobCreateWithCommand(ctx, input, "Create Job for "+resource.BreadcrumbForResource(service), cmd), nil
}

func newJobCreateCmd(configs ...jobCreateCommandConfig) *cobra.Command {
	var config jobCreateCommandConfig
	if len(configs) > 0 {
		config = configs[0]
	}

	cmd := &cobra.Command{
		Use:   "create [serviceID]",
		Short: "Create a new job for a service",
		Args:  cobra.MaximumNArgs(1),
		Example: `  # Create a job and return immediately
  render jobs create srv-abc123 --start-command "bundle exec rake task"

  # Wait for success in CI; failed or canceled jobs exit nonzero
  render jobs create srv-abc123 --confirm --wait --start-command "npm test"

  # Tail logs with a bounded wait (--tail implies --wait)
  render jobs create srv-abc123 --confirm --tail --timeout 20m --start-command "npm run worker"

  # Keep stdout as one JSON document; progress and streamed logs go to stderr
  render jobs create srv-abc123 --confirm --tail --output json > job.json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.JobCreateInput

		wait, err := cmd.Flags().GetBool("wait")
		if err != nil {
			return err
		}
		tail, err := cmd.Flags().GetBool("tail")
		if err != nil {
			return err
		}
		if tail {
			wait = true
		}

		timeout, err := cmd.Flags().GetDuration("timeout")
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("timeout") && !wait {
			return errors.New("--timeout requires --wait or --tail")
		}
		if wait && timeout <= 0 {
			return errors.New("--timeout must be positive")
		}
		if wait {
			command.DefaultFormatNonInteractive(cmd)
		}

		err = command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		if wait {
			return nonInteractiveJobCreateAndWait(cmd, input, tail, timeout, config)
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

		_, err = interactiveJobCreate(cmd, &input)
		return err
	}

	cmd.Flags().String("start-command", "", "Set the job start command")
	cmd.Flags().String("plan-id", "", "Set the plan ID for the job (Optional)")
	cmd.Flags().Bool("wait", false, "Wait for the job to finish and exit nonzero if it fails or is canceled")
	cmd.Flags().Bool("tail", false, "Stream job logs and wait for the job to finish")
	cmd.Flags().Duration("timeout", job.DefaultTimeout, "Maximum time to wait for the job")
	setFlagPlaceholder(cmd.Flags(), "start-command", "COMMAND")
	setFlagPlaceholder(cmd.Flags(), "plan-id", "PLAN_ID")
	setFlagPlaceholder(cmd.Flags(), "timeout", "DURATION")

	return cmd
}

func nonInteractiveJobCreateAndWait(cmd *cobra.Command, input views.JobCreateInput, tail bool, timeout time.Duration, config jobCreateCommandConfig) error {
	deps := dependencies.GetFromContext(cmd.Context())
	created, err := deps.JobRepo().CreateJob(cmd.Context(), job.CreateJobInput{
		ServiceID:    input.ServiceID,
		StartCommand: pointers.ValueOrDefault(input.StartCommand, ""),
		PlanID:       pointers.ValueOrDefault(input.PlanID, ""),
	})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for job %s to finish...\n", created.Id)
	if err != nil {
		return err
	}

	pollInterval := config.pollInterval
	if pollInterval <= 0 {
		pollInterval = job.DefaultPollInterval
	}
	runner := job.Runner{
		Retrieve: func(ctx context.Context) (*clientjob.Job, error) {
			return deps.JobRepo().GetJob(ctx, input.ServiceID, created.Id)
		},
		PollInterval:         pollInterval,
		MaxTransientAttempts: job.DefaultTransientAttempts,
		NewTicker:            config.newTicker,
		OnRetry: func(message string) {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), message)
		},
	}

	if tail {
		runner.OpenLogs = func(ctx context.Context, startTime *time.Time) (*logs.TailStream, error) {
			logInput := views.LogInput{ResourceIDs: []string{created.Id}, Tail: true}
			if startTime != nil {
				logInput.StartTime = &command.TimeOrRelative{T: startTime}
			}
			return deps.LogLoader().LoadLogStream(ctx, logInput)
		}
		runner.OnLog = func(entry *logclient.Log) error {
			out := cmd.OutOrStdout()
			format := command.GetFormatFromContext(cmd.Context())
			if format != nil && (*format == command.JSON || *format == command.YAML) {
				out = cmd.ErrOrStderr()
			}
			return writeLog(command.TEXT, out, entry)
		}
	}

	finalJob, waitErr := runner.Run(cmd.Context(), created.Id, tail, timeout)
	if finalJob != nil {
		_, printErr := command.PrintData(cmd, finalJob, func(result *clientjob.Job) string {
			status := "unknown"
			if result.Status != nil {
				status = string(*result.Status)
			}
			return text.FormatStringF("Job %s finished with status %s", result.Id, status)
		})
		if printErr != nil {
			return printErr
		}
	}
	if waitErr != nil {
		var terminalErr *job.TerminalError
		if errors.As(waitErr, &terminalErr) {
			return command.NewExitError(1, waitErr)
		}
		return waitErr
	}
	return nil
}
