package views

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/tui/layouts"
	"github.com/spf13/cobra"

	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui"
)

var (
	enter      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	esc        = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close menu"))
	openFilter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"))

	filterKeyBinds = []key.Binding{enter, esc}
)

const (
	sidebarWidth = 60
	footerHeight = 4
)

type LogInput struct {
	ResourceIDs []string `cli:"resources"`
	Instance    []string `cli:"instance"`
	Text        []string `cli:"text"`
	Level       []string `cli:"level"`
	Type        []string `cli:"type"`

	StartTime *command.TimeOrRelative `cli:"start"`
	EndTime   *command.TimeOrRelative `cli:"end"`

	Host       []string `cli:"host"`
	StatusCode []string `cli:"status-code"`
	Method     []string `cli:"method"`
	Path       []string `cli:"path"`

	TaskID    []string `cli:"task-id"`
	TaskRunID []string `cli:"task-run-id"`

	Limit     int    `cli:"limit"`
	Direction string `cli:"direction"`
	Tail      bool   `cli:"tail"`

	ListResourceInput ListResourceInput
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
	tabModel    *tui.TabModel
	logModel    *tui.LogModel
	footerModel *FooterModel

	layout *layouts.SidebarLayout

	onFilter    func() tea.Cmd
	isSearching bool
}

type FooterModel struct {
	help   func() string
	width  int
	height int
}

func (f *FooterModel) Init() tea.Cmd {
	return nil
}

func (f *FooterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return f, nil
}

func (f *FooterModel) View() string {
	dividingLine := lipgloss.NewStyle().Foreground(style.ColorBorder).Render(strings.Repeat("─", f.width))

	footerText := ansi.Wrap(f.help(), f.width, "-")

	// Keep the footer a constant height
	footerStyle := lipgloss.NewStyle().Height(f.height).Width(f.width)

	return footerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, dividingLine, footerText))
}

func (f *FooterModel) SetWidth(width int) {
	f.width = width
}

func (f *FooterModel) SetHeight(height int) {
	f.height = height
}

type tabDefinition struct {
	TabName    string
	FieldNames []string
}

func tabModel(fields []huh.Field) *tui.TabModel {
	tabDefinitions := []tabDefinition{
		{TabName: "Filter", FieldNames: []string{"resources", "instance", "text", "level", "type"}},
		{TabName: "Time", FieldNames: []string{"start", "end"}},
		{TabName: "Request", FieldNames: []string{"host", "status-code", "method", "path"}},
		{TabName: "Query", FieldNames: []string{"limit", "direction", "tail"}},
	}

	var tabs []*tui.Tab
	for _, tabDefinition := range tabDefinitions {
		tab := &tui.Tab{
			Name: tabDefinition.TabName,
		}

		var fieldsForTab []huh.Field
		for _, field := range fields {
			if slices.Contains(tabDefinition.FieldNames, field.GetKey()) {
				fieldsForTab = append(fieldsForTab, field)
			}
		}

		content := formFromFields(fieldsForTab)
		tab.Content = content

		tabs = append(tabs, tab)
	}

	return tui.NewTabModel(tabs)
}

func formFromFields(fields []huh.Field) *tui.Form {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Input.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next"))
	keyMap.Select.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next"))
	keyMap.Select.Filter = key.NewBinding()
	keyMap.MultiSelect.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next"))
	keyMap.MultiSelect.Toggle = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle"))
	keyMap.MultiSelect.Filter = key.NewBinding()

	return tui.NewForm(huh.NewForm(huh.NewGroup(fields...)).WithKeyMap(keyMap).WithShowHelp(false))
}

func NewLogsView(
	ctx context.Context,
	logsCmd *cobra.Command,
	interactiveLogsCommand func(ctx context.Context, input LogInput, breadcrumb string) tea.Cmd,
	input LogInput,
	loadLogFunc func(ctx context.Context, in LogInput) (*tui.LogResult, error),
) *LogsView {
	view := &LogsView{}
	// Create log filter form
	fields, result := command.HuhFormFields(logsCmd, &input)

	tabs := tabModel(fields)
	view.onFilter = func() tea.Cmd {
		var logInput LogInput
		err := command.StructFromFormValues(result, &logInput)
		if err != nil {
			return func() tea.Msg { return tui.ErrorMsg{Err: fmt.Errorf("failed to parse form values: %w", err)} }
		}

		return interactiveLogsCommand(ctx, logInput, "") // we don't need a breadcrumb for the filter window
	}
	view.tabModel = tabs

	// Create log view model
	view.logModel = tui.NewLogModel(command.LoadCmd(ctx, loadLogFunc, input))

	// Set the direction (need this in the model for pagination logic)
	view.logModel.SetDirection(mapDirection(input.Direction))

	// Set up the load more function for pagination
	view.logModel.SetLoadMoreFunc(func(startTime, endTime *time.Time) tea.Cmd {
		paginatedInput := input
		paginatedInput.StartTime = &command.TimeOrRelative{T: startTime}
		paginatedInput.EndTime = &command.TimeOrRelative{T: endTime}

		// Always use pagination limit (1000) for subsequent requests, not initial limit
		paginatedInput.Limit = logs.PaginationLogLimit

		// Load data silently without LoadingDataMsg/DoneLoadingDataMsg to avoid
		// triggering the spinner
		return func() tea.Msg {
			data, err := loadLogFunc(ctx, paginatedInput)
			if err != nil {
				return tui.ErrorMsg{Err: err}
			}
			return tui.LoadDataMsg[*tui.LogResult]{Data: data}
		}
	})

	view.footerModel = &FooterModel{help: view.logsHelp}
	view.layout = layouts.NewSidebarLayout(layouts.NewBoxLayout(lipgloss.NewStyle().PaddingRight(1), view.tabModel), view.logModel, view.footerModel)
	view.layout.SetSidebarWidth(sidebarWidth)
	view.layout.SetFooterHeight(footerHeight)

	return view
}

func (v *LogsView) Init() tea.Cmd {
	return v.layout.Init()
}

func (v *LogsView) filterHelp() string {
	keys := append(v.tabModel.KeyBinds(), filterKeyBinds...)

	currentTab := v.tabModel.CurrentTab().Content
	if form, ok := currentTab.(*tui.Form); ok {
		keys = append(keys, form.KeyBinds()...)
	}

	return help.New().ShortHelpView(keys)
}

func (v *LogsView) logsHelp() string {
	keys := append(v.logModel.KeyBinds(), openFilter)

	return help.New().ShortHelpView(keys)
}

func (v *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return v, v.onFilter()
		default:
			if k := msg.String(); k == "/" && !v.isSearching {
				v.isSearching = true
				v.footerModel.help = v.filterHelp
				v.layout.SetSidebarVisible(true)
				// Return nil to prevent the filter from handling the keypress
				return v, nil
			}
		}
	case *tui.BackMsg:
		if v.isSearching {
			msg.Handled = true
			v.isSearching = false
			v.footerModel.help = v.logsHelp
			v.layout.SetSidebarVisible(false)
		}
	}

	_, cmd := v.layout.Update(msg)
	return v, cmd
}

func (v *LogsView) View() string {
	return v.layout.View()
}
