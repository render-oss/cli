package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	clientjob "github.com/renderinc/render-cli/pkg/client/jobs"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/job"
	"github.com/renderinc/render-cli/pkg/tui"
)

type JobListInput struct {
	ServiceID string `cli:"arg:0"`
}

func LoadJobListData(ctx context.Context, input JobListInput) ([]*clientjob.Job, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	jobRepo := job.NewRepo(c)

	return jobRepo.ListJobs(ctx, job.ListJobsInput{
		ServiceID: input.ServiceID,
	})
}

type JobListView struct {
	list    *tui.List[*clientjob.Job]
	palette *PaletteView
}

func NewJobListView(ctx context.Context, input *JobListInput, generateCommands func(*clientjob.Job) tea.Cmd) *JobListView {
	listView := &JobListView{}

	onSelect := func(selectedItem tui.ListItem) tea.Cmd {
		selectedJob := selectedItem.(job.ListItem).Job()
		return generateCommands(selectedJob)
	}

	listView.list = tui.NewList(
		"Jobs",
		command.LoadCmd(ctx, LoadJobListData, *input),
		func(j *clientjob.Job) tui.ListItem {
			return job.NewListItem(j)
		},
		tui.WithOnSelect[*clientjob.Job](onSelect),
	)

	return listView
}

func (v *JobListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if v.palette != nil {
		_, cmd = v.palette.Update(msg)
	} else {
		_, cmd = v.list.Update(msg)
	}

	return v, cmd
}

func (v *JobListView) Init() tea.Cmd {
	return v.list.Init()
}

func (v *JobListView) View() string {
	if v.palette != nil {
		return v.palette.View()
	}

	return v.list.View()
}
