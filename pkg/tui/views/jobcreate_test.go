package views_test

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/cmd"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/stretchr/testify/require"
)

func TestJobCreate(t *testing.T) {
	ctx := context.Background()

	input := views.JobCreateInput{
		ServiceID: "service-id",
	}

	var createJobInput views.JobCreateInput

	createJob := func(ctx context.Context, input views.JobCreateInput) (*clientjob.Job, error) {
		createJobInput = input
		return &clientjob.Job{Id: "foo"}, nil
	}

	action := func(j *clientjob.Job) tea.Cmd {
		return nil
	}

	m := views.NewJobCreateView(ctx, &input, cmd.JobCreateCmd, createJob, action)
	tm := teatest.NewTestModel(t, testhelper.Stackify(m))

	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 80})

	// Add start command, then walk through the rest of the fields by hitting
	// Enter until huh submits naturally on the last field.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("echo 'hello world'")})
	for i := 0; i < 5; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(20 * time.Millisecond)
	}

	require.Eventually(t, func() bool {
		return createJobInput.StartCommand != nil && *createJobInput.StartCommand == "echo 'hello world'"
	}, time.Second, time.Millisecond*10)

	err := tm.Quit()
	require.NoError(t, err)
}
