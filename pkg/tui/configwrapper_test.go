package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopView is a configView stand-in that does nothing on its own; the test
// drives wrapper.Update directly so configView behavior is irrelevant.
type noopView struct{}

func (noopView) Init() tea.Cmd                       { return nil }
func (noopView) Update(tea.Msg) (tea.Model, tea.Cmd) { return noopView{}, nil }
func (noopView) View() string                        { return "" }

// collectMsgs runs cmd and flattens any tea.BatchMsg into a list of concrete
// messages. Returns nil for a nil cmd or nil msg.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// stepper mimics bubbletea's model-replacement behavior: whatever Update
// returns becomes the current model. A test that just keeps calling
// wrapper.Update would hide the original bug, where the wrapper handed
// off root-ship to configView on the first message.
type stepper struct{ current tea.Model }

func (s *stepper) step(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.current, cmd = s.current.Update(msg)
	return cmd
}

// When a non-WindowSizeMsg arrives during the config phase (e.g. a login
// LoadingDataMsg) before WindowSizeMsg, the wrapper must still capture a
// subsequent WindowSizeMsg and replay it to c.next on the DoneMsg handoff.
// Pins the race that broke the real CLI: originally the wrapper handed off
// root-ship to configView on the first non-Done message, so a later
// WindowSizeMsg never reached the wrapper and nothing was available to
// replay on the eventual Done handoff.
func TestConfigWrapper_ReplaysLastWindowSizeWhenLoadingMsgArrivesFirst(t *testing.T) {
	wrapper := NewConfigWrapper(noopView{}, "Login", noopView{})
	s := &stepper{current: wrapper}

	s.step(LoadingDataMsg{})
	s.step(tea.WindowSizeMsg{Width: 80, Height: 24})
	cmd := s.step(DoneMsg{})
	require.NotNil(t, cmd, "DoneMsg handoff must produce a command")

	var got tea.WindowSizeMsg
	var found bool
	for _, msg := range collectMsgs(cmd) {
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			got, found = wsm, true
		}
	}
	require.True(t, found, "handoff cmd must emit a WindowSizeMsg for c.next")
	assert.Equal(t, 80, got.Width)
	assert.Equal(t, 24, got.Height)
}

// If no WindowSizeMsg was seen during the config phase, the handoff cmd
// must not fabricate one.
func TestConfigWrapper_HandoffWithoutWindowSizeEmitsNone(t *testing.T) {
	wrapper := NewConfigWrapper(noopView{}, "Login", noopView{})

	_, cmd := wrapper.Update(DoneMsg{})

	for _, msg := range collectMsgs(cmd) {
		if _, ok := msg.(tea.WindowSizeMsg); ok {
			t.Fatalf("handoff cmd emitted a WindowSizeMsg without a captured size: %#v", msg)
		}
	}
}
