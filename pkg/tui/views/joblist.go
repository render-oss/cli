package views

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/render-oss/cli/pkg/client"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/job"
	"github.com/render-oss/cli/pkg/tui"
)

type JobListInput struct {
	ServiceID string `cli:"arg:0"`
}

func (j JobListInput) Validate(interactive bool) error {
	if !interactive {
		return errors.New("service id must be specified when output is not interactive")
	}
	return nil
}

func LoadJobListData(ctx context.Context, input JobListInput, cur client.Cursor) (client.Cursor, []*clientjob.Job, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create client: %w", err)
	}

	jobRepo := job.NewRepo(c)

	return jobRepo.ListJobs(ctx, job.ListJobsInput{
		ServiceID: input.ServiceID,
	}, cur)
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
		"",
		command.PaginatedLoadCmd(ctx, LoadJobListData, *input),
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
