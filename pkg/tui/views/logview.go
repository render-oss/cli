// pkg/tui/views/logsview.go
package views

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	lclient "github.com/renderinc/cli/pkg/client/logs"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/config"
	"github.com/renderinc/cli/pkg/logs"
	"github.com/renderinc/cli/pkg/pointers"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/tui"
)

type LogInput struct {
	ResourceIDs []string `cli:"resources"`
	Instance    []string `cli:"instance"`
	StartTime   *string  `cli:"start"`
	EndTime     *string  `cli:"end"`
	Text        []string `cli:"text"`
	Level       []string `cli:"level"`
	Type        []string `cli:"type"`

	Host       []string `cli:"host"`
	StatusCode []string `cli:"status-code"`
	Method     []string `cli:"method"`
	Path       []string `cli:"path"`

	Limit     int    `cli:"limit"`
	Direction string `cli:"direction"`
	Tail      bool   `cli:"tail"`

	ListResourceInput ListResourceInput
}

func (l LogInput) ToParam() (*client.ListLogsParams, error) {
	now := time.Now()
	ownerID, err := config.WorkspaceID()
	if err != nil {
		return nil, fmt.Errorf("error getting workspace ID: %v", err)
	}

	if l.Limit == 0 {
		l.Limit = 100
	}

	start, err := command.ParseTime(now, l.StartTime)
	if err != nil {
		return nil, err
	}
	end, err := command.ParseTime(now, l.EndTime)
	if err != nil {
		return nil, err
	}
	return &client.ListLogsParams{
		Resource:   l.ResourceIDs,
		OwnerId:    ownerID,
		Instance:   pointers.FromArray(l.Instance),
		Limit:      pointers.From(l.Limit),
		StartTime:  start,
		EndTime:    end,
		Text:       pointers.FromArray(l.Text),
		Level:      pointers.FromArray(l.Level),
		Type:       pointers.FromArray(l.Type),
		Host:       pointers.FromArray(l.Host),
		StatusCode: pointers.FromArray(l.StatusCode),
		Method:     pointers.FromArray(l.Method),
		Path:       pointers.FromArray(l.Path),
		Direction:  pointers.From(mapDirection(l.Direction)),
	}, nil
}

func mapDirection(direction string) lclient.LogDirection {
	switch direction {
	case "forward":
		return lclient.Forward
	case "backward":
		return lclient.Backward
	default:
		return lclient.Backward
	}
}

type LogsView struct {
	resourceTable *ResourceView
	logModel      *tui.LogModel
	filterModel   *tui.FilterModel
}

func LoadLogData(ctx context.Context, in LogInput) (*tui.LogResult, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	logRepo := logs.NewLogRepo(c)
	params, err := in.ToParam()
	if err != nil {
		return nil, fmt.Errorf("error converting input to params: %v", err)
	}

	if in.Tail {
		logChan, err := logRepo.TailLogs(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("error tailing logs: %v", err)
		}
		return &tui.LogResult{Logs: &client.Logs200Response{}, LogChannel: logChan}, nil
	}

	logs, err := logRepo.ListLogs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error listing logs: %v", err)
	}
	return &tui.LogResult{Logs: logs, LogChannel: nil}, nil
}

func NewLogsView(ctx context.Context, logsCmd *cobra.Command, interactiveLogsCommand func(ctx context.Context, input LogInput, breadcrumb string) tea.Cmd, input LogInput, opts ...tui.TableOption[resource.Resource]) *LogsView {
	view := &LogsView{}

	// If no resources specified, show resource selection view
	if len(input.ResourceIDs) == 0 {
		view.resourceTable = NewResourceView(ctx, input.ListResourceInput, func(r resource.Resource) tea.Cmd {
			input.ResourceIDs = []string{r.ID()}
			return interactiveLogsCommand(ctx, input, resource.BreadcrumbForResource(r))
		}, opts...)
	} else {
		// Create log filter form
		form, result := command.HuhForm(logsCmd, &input)
		view.filterModel = tui.NewFilterModel(form.WithHeight(10), func(form *huh.Form) tea.Cmd {
			var logInput LogInput
			err := command.StructFromFormValues(result, &logInput)
			if err != nil {
				return func() tea.Msg { return tui.ErrorMsg{Err: fmt.Errorf("failed to parse form values: %w", err)} }
			}

			return interactiveLogsCommand(ctx, logInput, "") // we don't need a breadcrumb for the filter window
		})

		// Create log view model
		view.logModel = tui.NewLogModel(
			view.filterModel,
			command.LoadCmd(ctx, LoadLogData, input),
		)
	}

	return view
}

func (v *LogsView) Init() tea.Cmd {
	if v.resourceTable != nil {
		return v.resourceTable.Init()
	}
	return v.logModel.Init()
}

func (v *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if v.resourceTable != nil {
		_, cmd := v.resourceTable.Update(msg)
		return v, cmd
	}
	_, cmd := v.logModel.Update(msg)
	return v, cmd
}

func (v *LogsView) View() string {
	if v.resourceTable != nil {
		return v.resourceTable.View()
	}
	return v.logModel.View()
}
