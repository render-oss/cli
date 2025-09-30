package flows

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

type LogFlowDeps interface {
	Stack() *tui.StackModel
	LogsCmd() *cobra.Command
	LogLoader() *views.LogLoader
	ResourceLoader() *views.ResourceLoader
}

type LogFlowOption func(*LogFlow)

func WithLocal(local bool) LogFlowOption {
	return func(f *LogFlow) {
		f.local = local
	}
}

type LogFlow struct {
	deps  LogFlowDeps
	local bool
}

func NewLogFlow(deps LogFlowDeps, opts ...LogFlowOption) *LogFlow {
	f := &LogFlow{deps: deps}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *LogFlow) LogsFlow(ctx context.Context, input views.LogInput) tea.Cmd {
	if len(input.ResourceIDs) == 0 && !f.local {
		return f.unspecifiedResource(ctx, input, f.logsFlow)
	}

	return f.logsFlow(ctx, input, "Logs")
}

func (f *LogFlow) TailLogsFlow(ctx context.Context, resourceID string) tea.Cmd {
	return f.logsFlow(ctx, views.LogInput{
		ResourceIDs: []string{resourceID},
		StartTime:   &command.TimeOrRelative{T: pointers.From(time.Now())},
		Tail:        true,
	}, "Logs")
}

func (f *LogFlow) logsFlow(ctx context.Context, input views.LogInput, breadcrumb string) tea.Cmd {
	return command.AddToStack(
		f.deps.Stack(),
		f.deps.LogsCmd(),
		breadcrumb,
		&input,
		views.NewLogsView(
			ctx,
			f.deps.LogsCmd(),
			f.filterLogs,
			input,
			f.deps.LogLoader().LoadLogData),
	)
}

func (f *LogFlow) unspecifiedResource(ctx context.Context, input views.LogInput, action func(ctx context.Context, input views.LogInput, breadcrumb string) tea.Cmd) tea.Cmd {
	resourceTable := views.NewResourceView(ctx, input.ListResourceInput, f.deps.ResourceLoader().LoadResourceData, func(r resource.Resource) tea.Cmd {
		input.ResourceIDs = []string{r.ID()}
		return action(ctx, input, resource.BreadcrumbForResource(r))
	}, tui.WithCustomOptions[resource.Resource](f.getLogsOptions(ctx)))

	return command.AddToStack(f.deps.Stack(), f.deps.LogsCmd(), "Resources", &input, resourceTable)
}

func (f *LogFlow) filterLogs(ctx context.Context, in views.LogInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, f.deps.LogsCmd(), breadcrumb, &in, views.NewLogsView(ctx, f.deps.LogsCmd(), f.filterLogs, in, f.deps.LogLoader().LoadLogData))
}

func (f *LogFlow) getLogsOptions(ctx context.Context) []tui.CustomOption {
	return []tui.CustomOption{
		WithCopyID(ctx, f.deps.LogsCmd()),
		WithWorkspaceSelection(ctx),
		WithProjectFilter(ctx, f.deps.LogsCmd(), "Project Filter", &views.LogInput{}, func(ctx context.Context, project *client.Project) tea.Cmd {
			logInput := views.LogInput{}
			if project != nil {
				logInput.ListResourceInput.Project = project
				logInput.ListResourceInput.EnvironmentIDs = project.EnvironmentIds
			}
			return f.LogsFlow(ctx, logInput)
		}),
	}
}
