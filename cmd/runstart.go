package cmd

import (
	"fmt"
	"os"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/spf13/cobra"
)

func NewRunStartCmd(deps flows.WorkflowDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [taskSlug]",
		Short: "Start a task run with the provided input",
		Long: `Start a task with the provided input. In non-interactive mode, provide input with --input or --input-file.

You can specify the task by its workflow slug and task name (e.g., my-workflow/my-task), either as a positional argument or with --task.

Input Format:
The input should be a JSON array where each element is an argument to the task. For example, if your task takes two arguments, provide: ["arg1", "arg2"]

You can provide input via:
  • --input with inline JSON
  • --input-file with a path to a JSON file

In interactive mode, you will be prompted to select the task and provide the input.`,
		Example: `  # Start a task run with inline JSON input
  render workflows tasks runs start --task tsk-1234 --input='["arg1", "arg2"]'

  # Start a task run by passing the task as a positional argument
  render workflows tasks runs start my-workflow/my-task --input='["arg1"]'

  # Start a task run with input from a file
  render workflows tasks runs start --task my-task --input-file=input.json

  # Start a task run against local workflow development server
  render workflows tasks runs start --task my-task --local --input='["test"]'`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.TaskRunInput

			if fileName, err := cmd.Flags().GetString("input-file"); err == nil && fileName != "" {
				fileName, err = command.ExpandPath(fileName)
				if err != nil {
					return fmt.Errorf("failed to resolve input file path: %w", err)
				}
				content, err := os.ReadFile(fileName)
				if err != nil {
					return fmt.Errorf("failed to read input file: %w", err)
				}
				cmd.Flags().Set("input", string(content))
			}

			err = command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse input: %w", err)
			}

			if input.TaskSlug == "" {
				if taskFlag, ferr := cmd.Flags().GetString("task"); ferr == nil && taskFlag != "" {
					input.TaskSlug = taskFlag
				}
			}

			if input.TaskSlug != "" && cmd.Flags().Changed("input") {
				command.DefaultFormatNonInteractive(cmd)
			}

			if nonInteractive, err := command.NonInteractive(cmd, func() (*workflows.TaskRun, error) {
				taskLoader := deps.WorkflowLoader()
				return taskLoader.CreateTaskRun(cmd.Context(), input)
			}, func(j *workflows.TaskRun) string {
				return text.FormatStringF("Created task run %s for %s", j.Id, input.TaskSlug)
			}); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps, flows.WithLocal(local)), local).TaskRunFlow(cmd.Context(), &input)

			return nil
		},
	}

	cmd.Flags().String(
		"input", "",
		"Provide task input as a JSON array",
	)
	cmd.Flags().String("input-file", "", "Read task input from a JSON file path")
	cmd.Flags().String("task", "", "ID or slug of the task to run (alternative to the positional argument)")
	cmd.MarkFlagFilename("input-file")
	setAnnotationBestEffort(cmd.Flags(), "input", command.FlagPlaceholderAnnotation, []string{"JSON"})
	setAnnotationBestEffort(cmd.Flags(), "input-file", command.FlagPlaceholderAnnotation, []string{"PATH"})
	setAnnotationBestEffort(cmd.Flags(), "task", command.FlagPlaceholderAnnotation, []string{"TASK"})

	return cmd
}
