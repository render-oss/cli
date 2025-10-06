package flows

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dashboard"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/taskrun"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
)

type WorkflowDeps interface {
	Stack() *tui.StackModel
	WorkflowLoader() *workflowviews.WorkflowLoader
	ResourceService() *resource.Service
	ListTaskRuns() *cobra.Command
	RunTask() *cobra.Command
	ListTask() *cobra.Command
	ListVersions() *cobra.Command
	ReleaseVersion() *cobra.Command
	ListWorkflow() *cobra.Command
	TaskRunDetailsCmd() *cobra.Command
	LogFlowDeps
}

type Workflow struct {
	deps     WorkflowDeps
	logsFlow *LogFlow
	local    bool
}

func NewWorkflow(deps WorkflowDeps, logsFlow *LogFlow, local bool) *Workflow {
	return &Workflow{deps: deps, logsFlow: logsFlow, local: local}
}

func (f *Workflow) TaskRunFlow(ctx context.Context, input *workflowviews.TaskRunInput) tea.Cmd {
	if input.TaskID == "" {
		return f.unspecifiedTask(ctx, func(t *workflows.Task) tea.Cmd {
			input.TaskID = t.Id
			return f.taskRun(ctx, input)
		})
	}

	return f.taskRun(ctx, input)
}

func (f *Workflow) TaskListFlow(ctx context.Context, input *workflowviews.TaskListInput) tea.Cmd {
	if input.WorkflowVersionID == "" {
		return f.unspecifiedTask(ctx, func(t *workflows.Task) tea.Cmd {
			return f.taskListPalette(ctx, t)
		})
	}

	return f.taskList(ctx, input, func(t *workflows.Task) tea.Cmd {
		return f.taskListPalette(ctx, t)
	})
}

func (f *Workflow) TaskRunListFlow(ctx context.Context, input *workflowviews.TaskRunListInput) tea.Cmd {
	if input.TaskID == "" {
		return f.unspecifiedTask(ctx, func(t *workflows.Task) tea.Cmd {
			input.TaskID = t.Id
			return f.taskRunList(ctx, input, func(tr *workflows.TaskRun) tea.Cmd {
				return f.taskRunListPalette(ctx, tr)
			})
		})
	}

	return f.taskRunList(ctx, input, func(tr *workflows.TaskRun) tea.Cmd {
		return f.taskRunListPalette(ctx, tr)
	})
}

func (f *Workflow) taskRun(ctx context.Context, input *workflowviews.TaskRunInput) tea.Cmd {
	return command.AddToStack(
		f.deps.Stack(),
		f.deps.RunTask(),
		"Run",
		input,
		workflowviews.NewTaskRunView(ctx, f.deps.WorkflowLoader(), input, f.deps.RunTask(), func(j *workflows.TaskRun) tea.Cmd {
			workflowID, err := f.getWorkflowID(ctx, j.TaskId)
			if err != nil {
				return command.AddErrToStack(ctx, f.deps.RunTask(), err)
			}
			return f.logsFlow.LogsFlow(ctx, views.LogInput{
				// Start querying logs from 10 seconds ago to avoid missing any logs
				StartTime:   &command.TimeOrRelative{T: pointers.From(time.Now().Add(-10 * time.Second))},
				ResourceIDs: []string{workflowID},
				TaskRunID:   []string{j.Id},
				Tail:        true,
			})
		}),
	)
}

func (f *Workflow) getWorkflowID(ctx context.Context, taskId string) (string, error) {
	if f.local {
		return "wfl-local", nil
	}

	task, err := f.deps.WorkflowLoader().GetTask(ctx, taskId)
	if err != nil {
		return "", err
	}
	return *task.WorkflowId, nil
}

func (f *Workflow) taskList(ctx context.Context, input *workflowviews.TaskListInput, action func(t *workflows.Task) tea.Cmd) tea.Cmd {
	return command.AddToStack(
		f.deps.Stack(),
		f.deps.ListTask(),
		"Tasks",
		input,
		workflowviews.NewTaskListView(ctx, f.deps.WorkflowLoader(), *input, action),
	)
}

func (f *Workflow) workflowList(ctx context.Context, input *workflowviews.WorkflowInput, action func(ctx context.Context, r resource.Resource) tea.Cmd) tea.Cmd {
	return command.AddToStack(
		f.deps.Stack(),
		f.deps.ListWorkflow(),
		"Workflows",
		input,
		workflowviews.NewWorkflowList(ctx, f.deps.WorkflowLoader(), *input, action),
	)
}

func (f *Workflow) VersionList(ctx context.Context, input *workflowviews.VersionListInput) tea.Cmd {
	if input.WorkflowID == "" {
		return f.workflowList(ctx, &workflowviews.WorkflowInput{}, func(ctx context.Context, r resource.Resource) tea.Cmd {
			input.WorkflowID = r.ID()
			return f.versionList(ctx, input, func(v *workflows.WorkflowVersion) tea.Cmd {
				return f.versionListPalette(ctx, v)
			})
		})
	}

	return f.versionList(ctx, input, func(v *workflows.WorkflowVersion) tea.Cmd {
		return f.versionListPalette(ctx, v)
	})
}

func (f *Workflow) VersionRelease(ctx context.Context, input *workflowviews.VersionReleaseInput) tea.Cmd {
	if input.WorkflowID == "" {
		return f.workflowList(ctx, &workflowviews.WorkflowInput{}, func(ctx context.Context, r resource.Resource) tea.Cmd {
			input.WorkflowID = r.ID()
			return f.versionRelease(ctx, input)
		})
	}

	return f.versionRelease(ctx, input)
}

func (f *Workflow) versionRelease(ctx context.Context, input *workflowviews.VersionReleaseInput) tea.Cmd {
	return command.AddToStack(
		f.deps.Stack(),
		f.deps.ReleaseVersion(),
		"Release",
		input,
		workflowviews.NewVersionReleaseView(ctx, f.deps.WorkflowLoader(), input, func(v *workflows.WorkflowVersion) tea.Cmd {
			return f.logsFlow.LogsFlow(ctx, views.LogInput{
				ResourceIDs: []string{v.WorkflowId},
				Type:        []string{"build"},
				StartTime:   &command.TimeOrRelative{T: pointers.From(time.Now())},
				Tail:        true,
			})
		}),
	)
}

func (f *Workflow) unspecifiedTask(ctx context.Context, action func(t *workflows.Task) tea.Cmd) tea.Cmd {
	if f.local {
		// when running locally, we don't have a workflow version id, so we just use a dummy one
		return f.taskList(ctx, &workflowviews.TaskListInput{WorkflowVersionID: "local"}, action)
	}

	return f.workflowList(ctx, &workflowviews.WorkflowInput{}, func(ctx context.Context, r resource.Resource) tea.Cmd {
		return f.versionList(ctx, &workflowviews.VersionListInput{WorkflowID: r.ID()}, func(v *workflows.WorkflowVersion) tea.Cmd {
			return f.taskList(ctx, &workflowviews.TaskListInput{WorkflowVersionID: v.Id}, action)
		})
	})
}

func (f *Workflow) versionList(ctx context.Context, input *workflowviews.VersionListInput, action func(v *workflows.WorkflowVersion) tea.Cmd) tea.Cmd {
	return command.AddToStack(
		f.deps.Stack(),
		f.deps.ListVersions(),
		"Versions",
		input,
		workflowviews.NewVersionListView(ctx, f.deps.WorkflowLoader(), *input, action),
	)
}

func (f *Workflow) versionListPalette(ctx context.Context, v *workflows.WorkflowVersion) tea.Cmd {
	return command.AddToStack(f.deps.Stack(), f.deps.ListVersions(), v.Name, &views.PaletteCommand{},
		views.NewPaletteView(ctx, []views.PaletteCommand{
			{
				Name:        "tasks",
				Description: "View all tasks for this version",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return f.taskList(ctx, &workflowviews.TaskListInput{WorkflowVersionID: v.Id}, func(t *workflows.Task) tea.Cmd {
						return f.taskListPalette(ctx, t)
					})
				},
			},
			{
				Name:        "dashboard",
				Description: "Open Render Dashboard to the service's page",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					err := dashboard.OpenVersion(v.WorkflowId, v.Id)
					return command.AddErrToStack(ctx, f.deps.ListVersions(), err)
				},
			},
			{
				Name:        "logs",
				Description: "View version logs",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return f.logsFlow.LogsFlow(ctx, views.LogInput{
						ResourceIDs: []string{v.WorkflowId},
						Tail:        true,
					})
				},
			},
		}),
	)
}

func (f *Workflow) taskListPalette(ctx context.Context, t *workflows.Task) tea.Cmd {
	return command.AddToStack(f.deps.Stack(), f.deps.ListTask(), t.Name, &views.PaletteCommand{},
		views.NewPaletteView(ctx, []views.PaletteCommand{
			{
				Name:        "runs",
				Description: "View all runs for this task",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return f.TaskRunListFlow(ctx, &workflowviews.TaskRunListInput{TaskID: t.Id})
				},
			},
			{
				Name:        "run",
				Description: "Start a new task run with inputs",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return f.TaskRunFlow(ctx, &workflowviews.TaskRunInput{TaskID: t.Id})
				},
			},
		}),
	)
}

func (f *Workflow) taskRunList(ctx context.Context, input *workflowviews.TaskRunListInput, action func(tr *workflows.TaskRun) tea.Cmd) tea.Cmd {
	return command.AddToStack(f.deps.Stack(), f.deps.ListTaskRuns(), "Runs", input, workflowviews.NewTaskRunListView(
		ctx,
		f.deps.WorkflowLoader(),
		*input,
		action,
	))
}

func (f *Workflow) taskRunListPalette(ctx context.Context, taskRun *workflows.TaskRun) tea.Cmd {
	return command.AddToStack(f.deps.Stack(), f.deps.ListTaskRuns(), taskRun.Id, &views.PaletteCommand{},
		views.NewPaletteView(ctx, []views.PaletteCommand{
			{
				Name:        "logs",
				Description: "View task run logs",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					workflowID, err := f.getWorkflowID(ctx, taskRun.TaskId)
					if err != nil {
						return command.AddErrToStack(ctx, f.deps.RunTask(), err)
					}
					var startTime *command.TimeOrRelative
					if taskRun.StartedAt != nil {
						startTime = &command.TimeOrRelative{T: taskRun.StartedAt}
					}
					var endTime *command.TimeOrRelative
					if taskRun.CompletedAt != nil {
						endTime = &command.TimeOrRelative{T: taskRun.CompletedAt}
					}
					var tail bool
					if taskRun.Status != workflows.Completed && taskRun.Status != workflows.Failed {
						tail = true
					}
					return f.logsFlow.LogsFlow(ctx, views.LogInput{
						ResourceIDs: []string{workflowID},
						TaskRunID:   []string{taskRun.Id},
						StartTime:   startTime,
						EndTime:     endTime,
						Tail:        tail,
					})
				},
			},
			{
				Name:        "results",
				Description: "View task run results",
				Action: func(ctx context.Context, args []string) tea.Cmd {
					return f.taskRunDetails(ctx, &workflowviews.TaskRunDetailsInput{TaskRunID: taskRun.Id})
				},
			},
		}),
	)
}

func (f *Workflow) taskRunDetails(ctx context.Context, input *workflowviews.TaskRunDetailsInput) tea.Cmd {
	return command.AddToStack(f.deps.Stack(), f.deps.TaskRunDetailsCmd(), "Details", input, tui.NewDetailsModel[*workflows.TaskRunDetails](
		"Task Run Details",
		command.LoadCmd(ctx, f.deps.WorkflowLoader().LoadTaskRunDetails, input),
		taskrun.TaskRunDetailsFormat,
	))
}

func (f *Workflow) TaskRunDetailsFlow(ctx context.Context, input *workflowviews.TaskRunDetailsInput) tea.Cmd {
	if input.TaskRunID == "" {
		return f.unspecifiedTask(ctx, func(t *workflows.Task) tea.Cmd {
			return f.taskRunList(ctx, &workflowviews.TaskRunListInput{TaskID: t.Id}, func(tr *workflows.TaskRun) tea.Cmd {
				input.TaskRunID = tr.Id
				return f.taskRunDetails(ctx, input)
			})
		})
	}

	return f.taskRunDetails(ctx, input)
}
