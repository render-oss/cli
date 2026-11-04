package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/flowrun"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui"
	tuiflows "github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/render-oss/cli/pkg/workflow"
)

func newFlowsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flows",
		Short: "List flows and manage their runs",
	}
	cmd.AddCommand(children...)
	return cmd
}

func newFlowRunsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Start, list, and inspect flow runs",
	}
	cmd.AddCommand(children...)
	return cmd
}

func newFlowRunStepsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "steps",
		Short: "List and inspect flow run steps",
	}
	cmd.AddCommand(children...)
	return cmd
}

// FlowListInput is the raw command input parsed from Cobra flags for
// `render workflows flows list`.
type FlowListInput struct {
	FlowSlugs          []string `cli:"flow-slug"`
	WorkflowIDs        []string `cli:"workflow"`
	WorkflowVersionIDs []string `cli:"workflow-version"`
	Limit              int      `cli:"limit"`
	Cursor             string   `cli:"cursor"`
}

func newFlowListCmd(deps *dependencies.Dependencies, runListCmd, runStartCmd, runShowCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "List flows in the active workspace",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List flows accessible to your active workspace.

Flows are filtered by the active workspace. Narrow further with --flow-slug,
--workflow, or --workflow-version. The flag can be repeated.

In interactive mode, you will be prompted to select a workflow if no filter is
provided. Select a flow to view its runs or start a new run, then view a
run's logs or results.`,
		Example: `  # List all flows in the active workspace
  render workflows flows list

  # Filter by flow slug
  render workflows flows list --flow-slug my-workflow/my-flow

  # Filter by workflow ID or slug
  render workflows flows list --workflow my-workflow

  # JSON output
  render workflows flows list --output json`,
	}

	cmd.Flags().StringArray("flow-slug", nil, "Filter by flow slug (workflow-slug/flow-name[:version], repeatable)")
	cmd.Flags().StringArray("workflow", nil, "Filter by workflow ID or slug (repeatable)")
	cmd.Flags().StringArray("workflow-version", nil, "Filter by workflow version ID (repeatable)")
	addPaginationFlags(cmd)
	setAllFlagPlaceholders(cmd, map[string]string{
		"flow-slug":        "SLUG",
		"workflow":         "WORKFLOW",
		"workflow-version": "VERSION_ID",
		"limit":            "COUNT",
		"cursor":           "CURSOR",
	})

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input FlowListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		if err := validatePagination(input.Limit); err != nil {
			return err
		}

		workspaceID, err := config.WorkspaceID()
		if err != nil {
			return err
		}

		params := &client.ListFlowsParams{
			OwnerId: pointers.From([]string{workspaceID}),
			Limit:   pointers.From(input.Limit),
		}
		if input.Cursor != "" {
			params.Cursor = &input.Cursor
		}
		if len(input.FlowSlugs) > 0 {
			params.FlowSlug = &input.FlowSlugs
		}
		if len(input.WorkflowIDs) > 0 {
			workflowIDs, err := resolveWorkflowIDs(cmd.Context(), deps.WorkflowRepo(), input.WorkflowIDs)
			if err != nil {
				return err
			}
			params.WorkflowId = &workflowIDs
		}
		if len(input.WorkflowVersionIDs) > 0 {
			params.WorkflowVersionId = &input.WorkflowVersionIDs
		}

		var cursor client.Cursor
		nonInteractive, err := command.NonInteractive(cmd, func() ([]*workflows.Flow, error) {
			var flows []*workflows.Flow
			var err error
			cursor, flows, err = deps.FlowRunRepo().ListFlows(cmd.Context(), params)
			return flows, err
		}, text.FlowTable)
		if err != nil {
			return err
		}
		if nonInteractive {
			printNextCursorHint(cmd, cursor)
			return nil
		}

		screens := &flowTUI{deps: deps, listCmd: cmd, runListCmd: runListCmd, runStartCmd: runStartCmd, runShowCmd: runShowCmd}
		screens.flowList(cmd.Context(), &input, params)
		return nil
	}

	return cmd
}

// flowTUI opens the interactive flow screens. Each screen is pushed under the
// command that prints the same data, so the stack shows the equivalent command.
type flowTUI struct {
	deps        *dependencies.Dependencies
	listCmd     *cobra.Command
	runListCmd  *cobra.Command
	runStartCmd *cobra.Command
	runShowCmd  *cobra.Command
}

// newFlowTUI opens the flow screens under the commands setupFlowCommands
// registered.
func newFlowTUI(deps *dependencies.Dependencies) *flowTUI {
	cmds := deps.Commands.Workflow
	return &flowTUI{
		deps:        deps,
		listCmd:     cmds.FlowListCmd,
		runListCmd:  cmds.FlowRunListCmd,
		runStartCmd: cmds.FlowRunStartCmd,
		runShowCmd:  cmds.FlowRunShowCmd,
	}
}

// workflowFlows shows the flows of one workflow, as flows list --workflow does.
func (s *flowTUI) workflowFlows(ctx context.Context, workflowID string) tea.Cmd {
	workspaceID, err := config.WorkspaceID()
	if err != nil {
		return command.AddErrToStack(ctx, s.listCmd, err)
	}

	input := &FlowListInput{WorkflowIDs: []string{workflowID}}
	return s.flowTable(ctx, "Flows", input, &client.ListFlowsParams{
		OwnerId:    pointers.From([]string{workspaceID}),
		WorkflowId: &input.WorkflowIDs,
		Limit:      pointers.From(defaultFlowListPageSize),
	})
}

// flowList prompts for a workflow when no filter narrows the list, then shows
// the matching flows.
func (s *flowTUI) flowList(ctx context.Context, input *FlowListInput, params *client.ListFlowsParams) tea.Cmd {
	if len(input.FlowSlugs) == 0 && len(input.WorkflowIDs) == 0 && len(input.WorkflowVersionIDs) == 0 {
		return command.AddToStack(s.deps.Stack(), s.listCmd, "Workflows", input,
			workflowviews.NewWorkflowList(ctx, s.deps.WorkflowLoader(), workflowviews.WorkflowInput{}, func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.WorkflowIDs = []string{r.ID()}
				params.WorkflowId = &input.WorkflowIDs
				return s.flowTable(ctx, resource.BreadcrumbForResource(r), input, params)
			}),
		)
	}

	return s.flowTable(ctx, "Flows", input, params)
}

func (s *flowTUI) flowTable(ctx context.Context, breadcrumb string, input *FlowListInput, params *client.ListFlowsParams) tea.Cmd {
	loadFlows := func(ctx context.Context, params *client.ListFlowsParams) ([]*workflows.Flow, error) {
		_, flows, err := s.deps.FlowRunRepo().ListFlows(ctx, params)
		return flows, err
	}

	return command.AddToStack(s.deps.Stack(), s.listCmd, breadcrumb, input,
		workflowviews.NewFlowListView(command.LoadCmd(ctx, loadFlows, params), func(f *workflows.Flow) tea.Cmd {
			return s.flowPalette(ctx, f)
		}),
	)
}

func (s *flowTUI) flowPalette(ctx context.Context, f *workflows.Flow) tea.Cmd {
	return command.AddToStack(s.deps.Stack(), s.listCmd, f.Name, &views.PaletteCommand{},
		views.NewPaletteView(ctx, []views.PaletteCommand{
			{
				Name:        "runs",
				Description: "View all runs for this flow",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return s.runList(ctx, f)
				},
			},
			{
				Name:        "run",
				Description: "Start a new flow run with inputs",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return s.runStart(ctx, f)
				},
			},
		}),
	)
}

func (s *flowTUI) runList(ctx context.Context, f *workflows.Flow) tea.Cmd {
	return command.AddToStack(s.deps.Stack(), s.runListCmd, "Runs", &FlowRunListInput{FlowIDs: []string{f.Id}},
		workflowviews.NewFlowRunListView(command.LoadCmd(ctx, s.loadRuns, f.Id), func(run *workflows.FlowRun) tea.Cmd {
			return s.runPalette(ctx, f, run)
		}),
	)
}

func (s *flowTUI) runPalette(ctx context.Context, f *workflows.Flow, run *workflows.FlowRun) tea.Cmd {
	return command.AddToStack(s.deps.Stack(), s.runListCmd, run.Id, &views.PaletteCommand{},
		views.NewPaletteView(ctx, []views.PaletteCommand{
			{
				Name:        "logs",
				Description: "View flow run logs",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return s.runLogs(ctx, f, run)
				},
			},
			{
				Name:        "results",
				Description: "View flow run results",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return s.runDetails(ctx, run.Id)
				},
			},
		}),
	)
}

// runLogs opens a flow run's logs.
func (s *flowTUI) runLogs(ctx context.Context, f *workflows.Flow, run *workflows.FlowRun) tea.Cmd {
	if f.WorkflowId == nil {
		return command.AddErrToStack(ctx, s.runListCmd, fmt.Errorf("flow %s has no workflow", f.Id))
	}

	return tuiflows.NewLogFlow(s.deps).LogsFlow(ctx, views.RunLogInput(
		*f.WorkflowId, run.Id, run.StartedAt, run.CompletedAt, flowrun.IsFinished(run.Status),
	))
}

func (s *flowTUI) loadRuns(ctx context.Context, flowID string) ([]*workflows.FlowRun, error) {
	workspaceID, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	_, runs, err := s.deps.FlowRunRepo().ListFlowRuns(ctx, &client.ListFlowRunsParams{
		OwnerId: pointers.From([]string{workspaceID}),
		FlowId:  pointers.From([]string{flowID}),
		Limit:   pointers.From(defaultFlowListPageSize),
	})
	return runs, err
}

func (s *flowTUI) runStart(ctx context.Context, f *workflows.Flow) tea.Cmd {
	input := &workflowviews.FlowRunInput{}
	startRun := func(ctx context.Context, in workflowviews.FlowRunInput) (*workflows.FlowRun, error) {
		return s.startRun(ctx, f, in)
	}

	return command.AddToStack(s.deps.Stack(), s.runStartCmd, "Run", input,
		workflowviews.NewRunView(ctx, startRun, input, s.runStartCmd, func(run *workflows.FlowRun) tea.Cmd {
			return tuiflows.NewLogFlow(s.deps).LogsFlow(ctx, views.StartedRunLogInput(pointers.StringValue(f.WorkflowId), run.Id))
		}),
	)
}

// startRun starts a run of the selected flow. It passes the flow's ID rather
// than a slug so the run uses the flow's own version, not the latest.
func (s *flowTUI) startRun(ctx context.Context, f *workflows.Flow, in workflowviews.FlowRunInput) (*workflows.FlowRun, error) {
	var flowInput workflows.FlowInput
	if err := json.Unmarshal([]byte(in.Input), &flowInput); err != nil {
		return nil, fmt.Errorf("input must be valid JSON: %w", err)
	}

	return s.deps.FlowRunRepo().StartFlowRun(ctx, workflows.StartFlowRun{
		Flow:  f.Id,
		Input: flowInput,
	})
}

func (s *flowTUI) runDetails(ctx context.Context, flowRunID string) tea.Cmd {
	return command.AddToStack(s.deps.Stack(), s.runShowCmd, "Details", &FlowRunShowInput{FlowRunID: flowRunID},
		tui.NewDetailsModel(
			"Flow Run Details",
			command.LoadCmd(ctx, s.deps.FlowRunRepo().GetFlowRun, flowRunID),
			flowrun.RunDetailsFormat,
		),
	)
}

func newFlowGetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <flowID>",
		Short:        "Show details for a flow",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Example: `  # Show flow details
  render workflows flows get flw-1234

  # JSON output
  render workflows flows get flw-1234 --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		flowID := args[0]
		_, err := command.NonInteractive(cmd, func() (*workflows.Flow, error) {
			return deps.FlowRunRepo().GetFlow(cmd.Context(), flowID)
		}, flowrun.FlowDetails)
		return err
	}

	return cmd
}

// FlowRunListInput is the raw command input parsed from Cobra flags for
// `render workflows flows runs list`.
type FlowRunListInput struct {
	FlowSlugs          []string `cli:"flow-slug"`
	FlowIDs            []string `cli:"flow"`
	Statuses           []string `cli:"status"`
	WorkflowIDs        []string `cli:"workflow"`
	WorkflowVersionIDs []string `cli:"workflow-version"`
	Limit              int      `cli:"limit"`
	Cursor             string   `cli:"cursor"`
}

func newFlowRunListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "List flow runs",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List flow runs accessible to your active workspace.

At least one of --flow-slug, --flow, --workflow, or --workflow-version
is required. Narrow further with --status. Each flag is repeatable.`,
		Example: `  # Filter by flow slug
  render workflows flows runs list --flow-slug my-workflow/my-flow

  # Filter by flow and status
  render workflows flows runs list --flow flw-1234 --status running --status pending

  # Filter by workflow slug, with JSON output
  render workflows flows runs list --workflow my-workflow --output json`,
	}

	cmd.Flags().StringArray("flow-slug", nil, "Filter by flow slug (repeatable)")
	cmd.Flags().StringArray("flow", nil, "Filter by flow ID (repeatable)")
	cmd.Flags().StringArray("status", nil, "Filter by flow run status (pending|running|succeeded|failed|canceled|paused, repeatable)")
	cmd.Flags().StringArray("workflow", nil, "Filter by workflow ID or slug (repeatable)")
	cmd.Flags().StringArray("workflow-version", nil, "Filter by workflow version ID (repeatable)")
	cmd.MarkFlagsOneRequired("flow-slug", "flow", "workflow", "workflow-version")
	addPaginationFlags(cmd)
	setAllFlagPlaceholders(cmd, map[string]string{
		"flow-slug":        "SLUG",
		"flow":             "FLOW_ID",
		"status":           "STATUS",
		"workflow":         "WORKFLOW",
		"workflow-version": "VERSION_ID",
		"limit":            "COUNT",
		"cursor":           "CURSOR",
	})

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input FlowRunListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		if err := validatePagination(input.Limit); err != nil {
			return err
		}

		workspaceID, err := config.WorkspaceID()
		if err != nil {
			return err
		}

		params := &client.ListFlowRunsParams{
			OwnerId: pointers.From([]string{workspaceID}),
			Limit:   pointers.From(input.Limit),
		}
		if input.Cursor != "" {
			params.Cursor = &input.Cursor
		}
		if len(input.FlowSlugs) > 0 {
			params.FlowSlug = &input.FlowSlugs
		}
		if len(input.FlowIDs) > 0 {
			params.FlowId = &input.FlowIDs
		}
		if len(input.Statuses) > 0 {
			statuses := make([]workflows.FlowRunStatus, 0, len(input.Statuses))
			for _, s := range input.Statuses {
				statuses = append(statuses, workflows.FlowRunStatus(s))
			}
			params.Status = &statuses
		}
		if len(input.WorkflowIDs) > 0 {
			workflowIDs, err := resolveWorkflowIDs(cmd.Context(), deps.WorkflowRepo(), input.WorkflowIDs)
			if err != nil {
				return err
			}
			params.WorkflowId = &workflowIDs
		}
		if len(input.WorkflowVersionIDs) > 0 {
			params.WorkflowVersionId = &input.WorkflowVersionIDs
		}

		var cursor client.Cursor
		if _, err := command.NonInteractive(cmd, func() ([]*workflows.FlowRun, error) {
			var runs []*workflows.FlowRun
			var err error
			cursor, runs, err = deps.FlowRunRepo().ListFlowRuns(cmd.Context(), params)
			return runs, err
		}, text.FlowRunTable); err != nil {
			return err
		}
		printNextCursorHint(cmd, cursor)
		return nil
	}

	return cmd
}

// FlowRunStartInput is the raw command input parsed from Cobra flags for
// `render workflows flows runs start`.
type FlowRunStartInput struct {
	Flow           string `cli:"flow"`
	Input          string `cli:"input"`
	IdempotencyKey string `cli:"idempotency-key"`
}

func newFlowRunStartCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "start <flowSlug>",
		Short:        "Start a flow run with the provided input",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		Long: `Start a flow run with the provided input.

The flow is identified by its slug in the format workflow-slug/flow-name. An
optional version can be appended (workflow-slug/flow-name:version); without a
version, the latest release is used. Pass the slug as a positional argument or
with --flow.

Input is a JSON value: a JSON array for positional arguments, or a JSON object
for named parameters. Provide it inline with --input or load it from a file
with --input-file.`,
		Example: `  # Start a flow run with positional input
  render workflows flows runs start my-workflow/my-flow --input='["arg1", "arg2"]'

  # Start a flow run with named parameters from a file
  render workflows flows runs start my-workflow/my-flow --input-file=input.json

  # Make the request safe to retry with an idempotency key
  render workflows flows runs start my-workflow/my-flow \
    --input='[]' --idempotency-key=1234-5678`,
	}

	cmd.Flags().String("input", "", "Flow input as JSON (array for positional args, object for named parameters)")
	cmd.Flags().String("input-file", "", "Read flow input from a JSON file path")
	cmd.Flags().String("flow", "", "Flow slug (alternative to the positional argument)")
	cmd.Flags().String("idempotency-key", "", "Client-generated key that makes starting the run safe to retry within 24 hours")
	_ = cmd.MarkFlagFilename("input-file")
	setAllFlagPlaceholders(cmd, map[string]string{
		"input":           "JSON",
		"input-file":      "PATH",
		"flow":            "SLUG",
		"idempotency-key": "KEY",
	})

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		if fileName, err := cmd.Flags().GetString("input-file"); err == nil && fileName != "" {
			fileName, err = command.ExpandPath(fileName)
			if err != nil {
				return fmt.Errorf("failed to resolve input file path: %w", err)
			}
			content, err := os.ReadFile(fileName)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}
			if err := cmd.Flags().Set("input", string(content)); err != nil {
				return fmt.Errorf("failed to set input: %w", err)
			}
		}

		var input FlowRunStartInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		flowSlug := input.Flow
		if flowSlug == "" && len(args) == 1 {
			flowSlug = args[0]
		}
		if flowSlug == "" {
			return fmt.Errorf("flow slug is required (positional argument or --flow)")
		}
		if input.Input == "" {
			return fmt.Errorf("--input or --input-file is required")
		}

		var flowInput workflows.FlowInput
		if err := json.Unmarshal([]byte(input.Input), &flowInput); err != nil {
			return fmt.Errorf("failed to parse --input as JSON: %w", err)
		}

		body := workflows.StartFlowRun{
			Flow:  flowSlug,
			Input: flowInput,
		}
		if input.IdempotencyKey != "" {
			body.IdempotencyKey = &input.IdempotencyKey
		}

		_, err := command.NonInteractive(cmd, func() (*workflows.FlowRun, error) {
			run, err := deps.FlowRunRepo().StartFlowRun(cmd.Context(), body)
			var notFound *flowrun.NotFoundError
			if errors.As(err, &notFound) {
				return nil, fmt.Errorf("%w. Identify your flow with `workflow-slug/flow-name`. Run `render workflows flows list` to see your flows", err)
			}
			return run, err
		}, func(run *workflows.FlowRun) string {
			return text.FormatStringF("Started flow run %s for %s", run.Id, flowSlug)
		})
		return err
	}

	return cmd
}

func newFlowRunCancelCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "cancel <flowRunID>",
		Short:        "Cancel a running flow run",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Example: `  # Cancel a flow run
  render workflows flows runs cancel frn-1234`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		flowRunID := args[0]
		_, err := command.NonInteractive(cmd, func() (string, error) {
			run, err := deps.FlowRunRepo().GetFlowRun(cmd.Context(), flowRunID)
			if err != nil {
				return "", err
			}
			if flowrun.IsFinished(run.Status) {
				return "", fmt.Errorf("flow run %s already finished (%s)", flowRunID, run.Status)
			}

			if err := deps.FlowRunRepo().CancelFlowRun(cmd.Context(), flowRunID); err != nil {
				return "", err
			}
			return fmt.Sprintf("Canceled flow run %s", flowRunID), nil
		}, text.FormatString)
		return err
	}

	return cmd
}

// FlowRunShowInput is the raw command input for `render workflows flows runs show`.
type FlowRunShowInput struct {
	FlowRunID string `cli:"arg:0"`
}

func newFlowRunShowCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "show <flowRunID>",
		Short:        "Show detailed information about a flow run",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Display a flow run's status, input, and result.

If the run failed because a replay didn't match its history, the output names
the step and what changed. If it failed on a limit, the output names the limit
and the run's value.`,
		Example: `  # Show details for a flow run
  render workflows flows runs show frn-1234

  # JSON output
  render workflows flows runs show frn-1234 --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		flowRunID := args[0]
		_, err := command.NonInteractive(cmd, func() (*workflows.FlowRunDetails, error) {
			return deps.FlowRunRepo().GetFlowRun(cmd.Context(), flowRunID)
		}, flowrun.Details)
		return err
	}

	return cmd
}

// FlowRunStepListInput is the raw command input parsed from Cobra flags for
// `render workflows flows runs steps list`.
type FlowRunStepListInput struct {
	Limit  int    `cli:"limit"`
	Cursor string `cli:"cursor"`
}

func newFlowRunStepListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list <flowRunID>",
		Aliases:      []string{"ls"},
		Short:        "List the steps a flow run has recorded",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Example: `  # List steps for a flow run
  render workflows flows runs steps list frn-1234

  # JSON output
  render workflows flows runs steps list frn-1234 --output json`,
	}

	addPaginationFlags(cmd)
	setAllFlagPlaceholders(cmd, map[string]string{
		"limit":  "COUNT",
		"cursor": "CURSOR",
	})

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input FlowRunStepListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		if err := validatePagination(input.Limit); err != nil {
			return err
		}

		flowRunID := args[0]
		params := &client.ListFlowRunStepsParams{Limit: pointers.From(input.Limit)}
		if input.Cursor != "" {
			params.Cursor = &input.Cursor
		}

		var cursor client.Cursor
		if _, err := command.NonInteractive(cmd, func() ([]*workflows.FlowRunStep, error) {
			var steps []*workflows.FlowRunStep
			var err error
			cursor, steps, err = deps.FlowRunRepo().ListFlowRunSteps(cmd.Context(), flowRunID, params)
			return steps, err
		}, text.FlowRunStepTable); err != nil {
			return err
		}
		printNextCursorHint(cmd, cursor)
		return nil
	}

	return cmd
}

func newFlowRunStepShowCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "show <flowRunID> <stepSeq>",
		Short:        "Show detailed information about a flow run step",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		Example: `  # Show details for a flow run step
  render workflows flows runs steps show frn-1234 3`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		flowRunID := args[0]
		seq, err := parseStepSeq(args[1])
		if err != nil {
			return err
		}
		_, err = command.NonInteractive(cmd, func() (*workflows.FlowRunStepDetails, error) {
			return deps.FlowRunRepo().GetFlowRunStep(cmd.Context(), flowRunID, seq)
		}, flowrun.StepDetails)
		return err
	}

	return cmd
}

// defaultFlowListPageSize matches other paginated list commands (e.g. object
// storage) so flows list behaves the same by default.
const defaultFlowListPageSize = 100

// addPaginationFlags adds the shared --limit and --cursor flags used by the
// flow list commands.
func addPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().Int("limit", defaultFlowListPageSize, "Maximum number of items to return per page")
	cmd.Flags().String("cursor", "", "Pagination cursor from a previous page (printed to stderr when more results are available)")
}

// validatePagination rejects a non-positive --limit.
func validatePagination(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("--limit must be greater than 0")
	}
	return nil
}

// printNextCursorHint writes the next-page cursor to stderr when the server
// signaled more results. Stdout stays clean for JSON/YAML/text piping.
func printNextCursorHint(cmd *cobra.Command, cursor client.Cursor) {
	if cursor == "" {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "More results available. Pass --cursor %s to fetch the next page.\n", cursor)
}

// resolveWorkflowIDs maps each --workflow value to a workflow ID. Values that
// look like IDs pass through; anything else is treated as a slug and looked up
// in the active workspace, since the flow endpoints only filter by ID.
func resolveWorkflowIDs(ctx context.Context, repo *workflow.Repo, values []string) ([]string, error) {
	if !slices.ContainsFunc(values, isWorkflowSlug) {
		return values, nil
	}

	workflows, err := repo.ListAllWorkflows(ctx, &client.ListWorkflowsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to look up workflows: %w", err)
	}
	idsBySlug := make(map[string]string, len(workflows))
	for _, wf := range workflows {
		if wf.Slug != nil {
			idsBySlug[*wf.Slug] = wf.Id
		}
	}

	ids := make([]string, 0, len(values))
	for _, v := range values {
		if !isWorkflowSlug(v) {
			ids = append(ids, v)
			continue
		}
		id, ok := idsBySlug[v]
		if !ok {
			return nil, fmt.Errorf("no workflow found with ID or slug %q in the active workspace", v)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func isWorkflowSlug(v string) bool {
	return !strings.HasPrefix(v, "wfl-")
}

func parseStepSeq(s string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid step sequence %q: must be a non-negative integer", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid step sequence %d: must be non-negative", n)
	}
	return n, nil
}
