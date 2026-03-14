package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	renderstyle "github.com/render-oss/cli/pkg/style"
)

// SetupStep is a named operation in the init pipeline.
type SetupStep struct {
	// Label is the display name shown when the step succeeds (e.g. "Template copied").
	Label string
	// FailLabel is shown when the step fails (e.g. "Dependency installation failed").
	// If empty, Label is used.
	FailLabel string
	// ActiveLabel is shown while the step is running
	// (e.g. "Installing dependencies..."). If empty, Label is used.
	ActiveLabel string
	// Detail is an optional function that returns a string to display below
	// the label when the step completes successfully (e.g. the command that ran).
	// Called after Run succeeds. If nil or returns empty, no detail is shown.
	Detail func() string
	// Critical means this step's failure aborts all subsequent steps,
	// even in interactive mode (stopOnError=false).
	Critical bool
	Run      func() error
}

// StepObserver receives progress notifications during step execution.
type StepObserver interface {
	OnStart(totalSteps int)
	OnStepStart(idx int)
	OnStepDone(idx int, detail string)
	OnStepError(idx int, err error)
	OnAllDone(stepErrors []StepError)
}

// StepError records which step failed and why.
type StepError struct {
	Index int
	Label string
	Err   error
}

// RunSteps executes steps in order, notifying the observer of progress.
// In non-interactive mode (stopOnError=true), it returns on the first error.
// In interactive mode (stopOnError=false), it continues past errors and
// collects them for the observer's OnAllDone callback.
func RunSteps(steps []SetupStep, obs StepObserver, stopOnError bool) error {
	obs.OnStart(len(steps))

	var stepErrors []StepError
	for i, step := range steps {
		obs.OnStepStart(i)

		if err := step.Run(); err != nil {
			obs.OnStepError(i, err)
			if stopOnError || step.Critical {
				return fmt.Errorf("step '%s' failed: %w", step.Label, err)
			}
			failLabel := step.FailLabel
			if failLabel == "" {
				failLabel = step.Label
			}
			stepErrors = append(stepErrors, StepError{Index: i, Label: failLabel, Err: err})
		} else {
			detail := ""
			if step.Detail != nil {
				detail = step.Detail()
			}
			obs.OnStepDone(i, detail)
		}
	}

	obs.OnAllDone(stepErrors)
	return nil
}

// checklistObserver renders an ANSI progress checklist for interactive mode.
// When stdout is not a TTY (e.g. piped, redirected, or over SSH), it falls
// back to appending lines without ANSI cursor movement.
type checklistObserver struct {
	mu        sync.Mutex
	cmd       *cobra.Command
	isTTY     bool
	steps     []checklistEntry
	prevLines int          // total lines in the previous render (for cursor movement)
	stopAnim  chan struct{} // signals the animation goroutine to stop
}

type checklistEntry struct {
	label       string
	failLabel   string
	activeLabel string
	detail      string // optional detail shown below the label when done (e.g. the command that ran)
	done        bool
	failed      bool
	active      bool
}

func newChecklistObserver(cmd *cobra.Command, steps []SetupStep) *checklistObserver {
	entries := make([]checklistEntry, len(steps))
	for i, s := range steps {
		failLabel := s.FailLabel
		if failLabel == "" {
			failLabel = s.Label
		}
		entries[i] = checklistEntry{
			label:       s.Label,
			failLabel:   failLabel,
			activeLabel: s.ActiveLabel,
		}
	}
	return &checklistObserver{
		cmd:   cmd,
		isTTY: isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()),
		steps: entries,
	}
}

func (o *checklistObserver) OnStart(_ int) {
	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)
	command.Println(o.cmd, "")
	command.Println(o.cmd, "  %s", dim.Render("Project initializing..."))
	command.Println(o.cmd, "")
	if o.isTTY {
		o.renderInitial()
		time.Sleep(300 * time.Millisecond)
	}
}

func (o *checklistObserver) OnStepStart(idx int) {
	o.mu.Lock()
	o.steps[idx].active = true
	if o.isTTY {
		o.renderAll()
	}
	o.mu.Unlock()
	if o.isTTY {
		time.Sleep(150 * time.Millisecond)
		o.startAnimation(idx)
	}
}

func (o *checklistObserver) OnStepDone(idx int, detail string) {
	o.stopAnimation()
	o.mu.Lock()
	defer o.mu.Unlock()
	o.steps[idx].active = false
	o.steps[idx].done = true
	o.steps[idx].detail = detail
	if o.isTTY {
		o.renderAll()
	} else {
		o.printEntry(o.steps[idx], false, false)
	}
}

func (o *checklistObserver) OnStepError(idx int, _ error) {
	o.stopAnimation()
	o.mu.Lock()
	defer o.mu.Unlock()
	o.steps[idx].active = false
	o.steps[idx].failed = true
	if o.isTTY {
		o.renderAll()
	} else {
		o.printEntry(o.steps[idx], false, false)
	}
}

// startAnimation launches a goroutine that cycles an ellipsis on the
// active step's label, redrawing the checklist every 300ms.
func (o *checklistObserver) startAnimation(idx int) {
	o.stopAnim = make(chan struct{})
	baseLabel := o.steps[idx].activeLabel
	if baseLabel == "" {
		baseLabel = o.steps[idx].label
	}
	// Strip trailing dots/ellipsis from the base label
	baseLabel = strings.TrimRight(baseLabel, ".")

	go func() {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		dots := 0
		for {
			select {
			case <-o.stopAnim:
				return
			case <-ticker.C:
				dots = (dots + 1) % 4 // cycles: "", ".", "..", "..."
				o.mu.Lock()
				o.steps[idx].activeLabel = baseLabel + strings.Repeat(".", dots)
				o.renderAll()
				o.mu.Unlock()
			}
		}
	}()
}

// stopAnimation stops the animation goroutine if one is running.
func (o *checklistObserver) stopAnimation() {
	if o.stopAnim != nil {
		close(o.stopAnim)
		o.stopAnim = nil
	}
}

func (o *checklistObserver) OnAllDone(stepErrors []StepError) {
	okStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorOK)
	command.Println(o.cmd, "")

	if len(stepErrors) == 0 {
		command.Println(o.cmd, "  %s Project initialized!", okStyle.Render("✓"))
	} else {
		warn := lipgloss.NewStyle().Foreground(renderstyle.ColorWarning)
		fail := lipgloss.NewStyle().Foreground(renderstyle.ColorError)
		dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

		command.Println(o.cmd, "  %s Project initialized with issues:", warn.Render("⚠"))
		command.Println(o.cmd, "")
		for _, se := range stepErrors {
			command.Println(o.cmd, "    %s %s", warn.Render("•"), fail.Render(se.Label))
			// Indent each line of the error message
			for _, line := range strings.Split(strings.TrimSpace(se.Err.Error()), "\n") {
				command.Println(o.cmd, "      %s", dim.Render(line))
			}
			command.Println(o.cmd, "")
		}
	}
}

// printEntry prints a single checklist entry.
// Done entries with a detail string render as 2 lines; all others are 1 line.
// When useActive is true and the entry is active, uses the activeLabel.
// When clearLine is true, emits ANSI erase-line before writing.
func (o *checklistObserver) printEntry(s checklistEntry, useActive bool, clearLine bool) {
	okStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorOK)
	fail := lipgloss.NewStyle().Foreground(renderstyle.ColorError)
	activeStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorInfo)
	dim := lipgloss.NewStyle().Foreground(renderstyle.ColorDeprioritized)

	if clearLine {
		fmt.Fprintf(o.cmd.OutOrStdout(), "\033[2K")
	}

	switch {
	case s.failed:
		command.Println(o.cmd, "  %s %s", fail.Render("✗"), s.failLabel)
	case s.done:
		command.Println(o.cmd, "  %s %s", okStyle.Render("■"), s.label)
		if s.detail != "" {
			if clearLine {
				fmt.Fprintf(o.cmd.OutOrStdout(), "\033[2K")
			}
			detailStyle := lipgloss.NewStyle().Foreground(renderstyle.ColorBorder)
			command.Println(o.cmd, "    %s", detailStyle.Render(s.detail))
		}
	case s.active:
		label := s.label
		if useActive && s.activeLabel != "" {
			label = s.activeLabel
		}
		command.Println(o.cmd, "  %s %s",
			activeStyle.Render("▶"), activeStyle.Render(label))
	default:
		command.Println(o.cmd, "  %s %s", dim.Render("□"), s.label)
	}
}

// renderInitial prints the first checklist frame (all pending).
func (o *checklistObserver) renderInitial() {
	for _, s := range o.steps {
		o.printEntry(s, false, false)
	}
	// Track line count: step lines only (blank before is part of OnStart)
	o.prevLines = len(o.steps)
}

// renderAll redraws the entire checklist using ANSI cursor movement.
func (o *checklistObserver) renderAll() {
	w := o.cmd.OutOrStdout()

	fmt.Fprintf(w, "\033[%dA", o.prevLines)

	for _, s := range o.steps {
		o.printEntry(s, true, true)
	}

	// Update line count: entry lines only (done entries with detail use 2 lines)
	o.prevLines = 0
	for _, s := range o.steps {
		o.prevLines++
		if s.done && s.detail != "" {
			o.prevLines++
		}
	}
}

// silentObserver prints simple status lines for non-interactive mode.
type silentObserver struct {
	cmd *cobra.Command
}

func newSilentObserver(cmd *cobra.Command) *silentObserver {
	return &silentObserver{cmd: cmd}
}

func (o *silentObserver) OnStart(_ int)                   {}
func (o *silentObserver) OnStepStart(_ int)              {}
func (o *silentObserver) OnStepDone(_ int, _ string)     {}
func (o *silentObserver) OnStepError(_ int, _ error)     {}

func (o *silentObserver) OnAllDone(_ []StepError) {
	// Non-interactive output is handled by the caller after RunSteps returns,
	// since it needs access to the result values.
}
