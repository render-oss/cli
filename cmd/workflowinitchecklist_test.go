package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingObserver captures all observer calls for assertions.
type recordingObserver struct {
	startCalled int
	starts      []int
	dones       []int
	errs        []StepError
	allDone     bool
	allDoneErrs []StepError
}

func (o *recordingObserver) OnStart(totalSteps int)             { o.startCalled = totalSteps }
func (o *recordingObserver) OnStepStart(idx int)                { o.starts = append(o.starts, idx) }
func (o *recordingObserver) OnStepDone(idx int, _ string)       { o.dones = append(o.dones, idx) }
func (o *recordingObserver) OnStepError(idx int, err error) {
	o.errs = append(o.errs, StepError{Index: idx, Err: err})
}
func (o *recordingObserver) OnAllDone(stepErrors []StepError) {
	o.allDone = true
	o.allDoneErrs = stepErrors
}

func TestRunSteps_AllSucceed(t *testing.T) {
	var order []string
	steps := []SetupStep{
		{Label: "first", Run: func() error { order = append(order, "first"); return nil }},
		{Label: "second", Run: func() error { order = append(order, "second"); return nil }},
		{Label: "third", Run: func() error { order = append(order, "third"); return nil }},
	}

	obs := &recordingObserver{}
	err := RunSteps(steps, obs, false)

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second", "third"}, order)
	assert.Equal(t, 3, obs.startCalled)
	assert.Equal(t, []int{0, 1, 2}, obs.starts)
	assert.Equal(t, []int{0, 1, 2}, obs.dones)
	assert.Empty(t, obs.errs)
	assert.True(t, obs.allDone)
	assert.Empty(t, obs.allDoneErrs)
}

func TestRunSteps_StopOnError(t *testing.T) {
	var order []string
	steps := []SetupStep{
		{Label: "first", Run: func() error { order = append(order, "first"); return nil }},
		{Label: "second", Run: func() error { return errors.New("boom") }},
		{Label: "third", Run: func() error { order = append(order, "third"); return nil }},
	}

	obs := &recordingObserver{}
	err := RunSteps(steps, obs, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "second")
	assert.Contains(t, err.Error(), "boom")
	assert.Equal(t, []string{"first"}, order, "third step should not run")
	assert.Equal(t, []int{0, 1}, obs.starts)
	assert.Equal(t, []int{0}, obs.dones)
	assert.Len(t, obs.errs, 1)
	assert.False(t, obs.allDone, "OnAllDone should not be called when stopping on error")
}

func TestRunSteps_ContinueOnError(t *testing.T) {
	var order []string
	steps := []SetupStep{
		{Label: "first", Run: func() error { order = append(order, "first"); return nil }},
		{Label: "second", Run: func() error { order = append(order, "second"); return errors.New("partial failure") }},
		{Label: "third", Run: func() error { order = append(order, "third"); return nil }},
	}

	obs := &recordingObserver{}
	err := RunSteps(steps, obs, false)

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second", "third"}, order, "all steps should run")
	assert.Equal(t, []int{0, 1, 2}, obs.starts)
	assert.Equal(t, []int{0, 2}, obs.dones)
	assert.Len(t, obs.errs, 1)
	assert.True(t, obs.allDone)
	assert.Len(t, obs.allDoneErrs, 1)
	assert.Equal(t, "second", obs.allDoneErrs[0].Label)
}

func TestChecklistObserver_OnAllDone_WithErrors(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	steps := []SetupStep{
		{Label: "Scaffold", Run: func() error { return nil }},
		{Label: "Dependencies", Run: func() error { return errors.New("pip not found") }},
		{Label: "Git", Run: func() error { return errors.New("git not installed") }},
	}
	obs := newChecklistObserver(cmd, steps)
	obs.isTTY = false

	_ = RunSteps(steps, obs, false)

	output := buf.String()
	assert.Contains(t, output, "initialized with issues")
	assert.Contains(t, output, "Dependencies")
	assert.Contains(t, output, "pip not found")
	assert.Contains(t, output, "Git")
	assert.Contains(t, output, "git not installed")
}

func TestChecklistObserver_NonTTY_NoANSIEscapes(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	steps := []SetupStep{
		{Label: "Step one", Run: func() error { return nil }},
		{Label: "Step two", Run: func() error { return errors.New("oops") }},
	}
	obs := newChecklistObserver(cmd, steps)
	obs.isTTY = false

	_ = RunSteps(steps, obs, false)

	output := buf.String()
	assert.NotContains(t, output, "\033[", "output should not contain ANSI escape sequences when not a TTY")
	assert.Contains(t, output, "Step one")
	assert.Contains(t, output, "Step two")
	assert.Contains(t, output, "oops")
}

func TestRunSteps_CriticalStepAbortsEvenWithoutStopOnError(t *testing.T) {
	var order []string
	steps := []SetupStep{
		{Label: "scaffold", Critical: true, Run: func() error { return errors.New("scaffold failed") }},
		{Label: "deps", Run: func() error { order = append(order, "deps"); return nil }},
	}

	obs := &recordingObserver{}
	err := RunSteps(steps, obs, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scaffold")
	assert.Empty(t, order, "deps should not run after critical step fails")
}

func TestRunSteps_NonCriticalErrorDoesNotBlockSubsequentSteps(t *testing.T) {
	var order []string
	steps := []SetupStep{
		{Label: "scaffold", Critical: true, Run: func() error { order = append(order, "scaffold"); return nil }},
		{Label: "deps", Run: func() error { return errors.New("deps failed") }},
		{Label: "git", Run: func() error { order = append(order, "git"); return nil }},
	}

	obs := &recordingObserver{}
	err := RunSteps(steps, obs, false)

	require.NoError(t, err)
	assert.Equal(t, []string{"scaffold", "git"}, order, "git should run even after deps fail")
}

func TestChecklistObserver_ErrorShowsFullMessageInSummary(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	steps := []SetupStep{
		{Label: "Dependencies installed", FailLabel: "Dependency installation failed", Run: func() error {
			return fmt.Errorf("exit status 1\nsh: pip: command not found")
		}},
	}
	obs := newChecklistObserver(cmd, steps)
	obs.isTTY = false

	_ = RunSteps(steps, obs, false)

	output := buf.String()
	// Checklist should show fail label, not error detail
	assert.Contains(t, output, "Dependency installation failed")
	// Summary should show full error
	assert.Contains(t, output, "exit status 1")
	assert.Contains(t, output, "command not found")
}
