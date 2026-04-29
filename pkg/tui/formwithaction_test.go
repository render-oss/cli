package tui_test

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
)

// waitOpts is the standard polling/timeout config used by these tests, matching
// pkg/tui/views/logview_test.go.
var waitOpts = []teatest.WaitForOption{
	teatest.WithCheckInterval(time.Millisecond * 10),
	teatest.WithDuration(time.Second * 3),
}

// formHarness wires up a FormWithAction inside a real StackModel and returns
// the pieces a test needs. The action pushes a sentinel model so the stack has
// a frame to pop when we send Esc.
type formHarness struct {
	stack    *tui.StackModel
	tm       *teatest.TestModel
	fieldPtr *string
	sentinel string
}

// newFormHarness returns a harness whose form factory captures the
// caller-provided field instance. Reusing the same field across rebuilds
// matches the pattern in workflowcreate.go / jobcreate.go / taskrun.go, where
// fields outlive the huh.Form they're wrapped in so user input persists across
// re-entry.
func newFormHarness(t *testing.T, field huh.Field, fieldPtr *string) *formHarness {
	t.Helper()
	stack := tui.NewStack()

	const sentinel = "SENTINEL_VIEW"
	sentinelModel := &testhelper.SimpleModel{Str: sentinel}

	action := tui.NewFormAction(
		func(string) tea.Cmd {
			return stack.Push(tui.ModelWithCmd{Model: sentinelModel, Breadcrumb: "Sentinel"})
		},
		tui.TypedCmd[string](func() tea.Msg {
			return tui.LoadDataMsg[string]{Data: "ok"}
		}),
	)

	buildForm := func() *huh.Form {
		return huh.NewForm(huh.NewGroup(field))
	}

	fwa := tui.NewFormWithAction(action, buildForm)
	stack.Push(tui.ModelWithCmd{Model: fwa, Breadcrumb: "Form"})

	tm := teatest.NewTestModel(t, stack)
	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	return &formHarness{stack: stack, tm: tm, fieldPtr: fieldPtr, sentinel: sentinel}
}

func (h *formHarness) cleanup(t *testing.T) {
	t.Helper()
	h.tm.Quit()
	h.tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))
}

// TestFormWithAction_RendersAfterReInit guards the regression where, after huh
// transitions to StateCompleted on submit, navigating back to the form (e.g.
// via Esc on a follow-up view) left the user on a blank screen because huh
// exposes no API to reset f.quitting.
//
// Each teatest.WaitFor consumes the output stream as it reads, so the final
// WaitFor sees only post-Esc output. If the form fails to re-render (the
// original bug), "FieldHeading" never appears in that fresh window and the
// WaitFor times out.
func TestFormWithAction_RendersAfterReInit(t *testing.T) {
	var fieldValue string
	field := huh.NewInput().Title("FieldHeading").Value(&fieldValue)
	h := newFormHarness(t, field, &fieldValue)
	defer h.cleanup(t)

	// 1. Initial render.
	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("FieldHeading"))
	}, waitOpts...)

	// 2. Submit. The action pushes the sentinel onto the stack.
	h.tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(h.sentinel))
	}, waitOpts...)

	// 3. Esc pops the sentinel; StackModel re-Inits the form. With the factory
	// pattern, this rebuilds a fresh huh.Form. Without it, the reused form is
	// in StateCompleted and View() returns "", so "FieldHeading" never appears
	// in this fresh output window and the WaitFor times out.
	h.tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("FieldHeading"))
	}, waitOpts...)
}

// TestFormWithAction_PreservesValuesOnReInit guards the design choice behind
// reusing huh.Field instances across factory rebuilds in production callers
// (workflowcreate.go, jobcreate.go, taskrun.go). Because the field's value
// pointer survives a rebuild, anything the user typed before submit should
// still be visible when they navigate back to the form.
func TestFormWithAction_PreservesValuesOnReInit(t *testing.T) {
	var fieldValue string
	field := huh.NewInput().Title("FieldHeading").Value(&fieldValue)
	h := newFormHarness(t, field, &fieldValue)
	defer h.cleanup(t)

	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("FieldHeading"))
	}, waitOpts...)

	// Type a marker value that wouldn't appear anywhere else in the form
	// chrome, so we can unambiguously detect its survival across re-init.
	const typed = "rendywashere"
	h.tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(typed)})
	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(typed))
	}, waitOpts...)

	// Submit and navigate to the sentinel.
	h.tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(h.sentinel))
	}, waitOpts...)

	// Esc back to the form. The rebuilt huh.Form wraps the same field instance,
	// whose Value pointer still references our typed string.
	h.tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(typed))
	}, waitOpts...)

	require.Equal(t, typed, *h.fieldPtr,
		"field's bound value should still hold the typed string after re-init")
}

// TestFormWithAction_ClearsScreenOnSubmit guards the secondary fix that
// prepends tea.ClearScreen to huh.Form.SubmitCmd, so the renderer wipes any
// leftover form chrome (titles, labels) before the loading state takes over.
// Without ClearScreen, leftover lines could persist briefly during the
// form-to-loading transition.
//
// We verify by looking for the EraseEntireDisplay ANSI sequence emitted by the
// bubble tea renderer when it processes a clearScreenMsg, which only happens
// via tea.ClearScreen in this flow. Using the ansi package's constant (rather
// than a hard-coded escape) means we follow the renderer's source of truth
// across upstream version bumps.
func TestFormWithAction_ClearsScreenOnSubmit(t *testing.T) {
	var fieldValue string
	field := huh.NewInput().Title("FieldHeading").Value(&fieldValue)
	h := newFormHarness(t, field, &fieldValue)
	defer h.cleanup(t)

	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("FieldHeading"))
	}, waitOpts...)

	h.tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(ansi.EraseEntireDisplay))
	}, waitOpts...)
}
