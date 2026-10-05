package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/render-oss/cli/pkg/skills"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/testhelper"
	"github.com/stretchr/testify/require"
)

func TestSkillsInstallSelection(t *testing.T) {
	testSkillsInstallSelection(t, false)
}

func TestSkillsInstallAgentSelection(t *testing.T) {
	testSkillsInstallSelection(t, true)
}

func testSkillsInstallSelection(t *testing.T, selectAgents bool) {
	t.Helper()
	wantAll := []string{"render-deploy", "render-debug"}
	if selectAgents {
		wantAll = []string{"Claude Code", "Cursor"}
	}
	for _, tc := range []struct {
		name       string
		individual bool
		backToAll  bool
		empty      bool
		abort      bool
		want       []string
	}{
		{name: "install all by default", want: wantAll},
		{name: "choose individual skills", individual: true, want: wantAll[1:]},
		{name: "return to install all after deselecting", individual: true, backToAll: true, want: wantAll},
		{name: "empty selection does not install", individual: true, empty: true},
		{name: "cancel does not install", abort: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := NewSkillsInstallView(SkillsInstallViewInput{})
			v.available = []skills.SkillInfo{
				{Name: "Deploy to Render", DirName: "render-deploy"},
				{Name: "Debug on Render", DirName: "render-debug"},
			}
			v.buildSkillForm()
			v.step = installStepSelectSkills
			form := v.skillForm
			allLabel := "Install all skills (2)"
			individualTitle := "Select skills to install"
			selectionStep, nextStep := installStepSelectSkills, installStepInstalling
			emptyError := "no skills selected"
			if selectAgents {
				v.allTools = []skills.Tool{
					{Name: "Claude Code", SkillsDir: "claude/skills"},
					{Name: "Cursor", SkillsDir: "cursor/skills"},
				}
				v.buildToolForm()
				form = v.toolForm
				allLabel = "All agents (2)"
				individualTitle = "Select agents to install skills to"
				selectionStep, nextStep = installStepSelectTools, installStepCloning
				emptyError = "no tools selected"
			}
			v.step = selectionStep
			form.SubmitCmd = tea.Quit
			form.CancelCmd = tea.Quit
			tm := teatest.NewTestModel(t, form, teatest.WithInitialTermSize(100, 30))
			t.Cleanup(func() { _ = tm.Quit() })
			testhelper.WaitForContains(t, tm.Output(), allLabel)

			if tc.individual {
				tm.Send(tea.KeyMsg{Type: tea.KeyDown})
				tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
				testhelper.WaitForContains(t, tm.Output(), individualTitle)
				tm.Send(tea.KeyMsg{Type: tea.KeySpace})
				if tc.empty {
					tm.Send(tea.KeyMsg{Type: tea.KeyDown})
					tm.Send(tea.KeyMsg{Type: tea.KeySpace})
				}
				if tc.backToAll {
					tm.Send(tea.KeyMsg{Type: tea.KeyShiftTab})
					testhelper.WaitForContains(t, tm.Output(), allLabel)
					tm.Send(tea.KeyMsg{Type: tea.KeyUp})
				}
			}
			if tc.abort {
				tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
			} else {
				tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
			}
			completed := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(*huh.Form)
			var cmd tea.Cmd
			if selectAgents {
				v.toolForm = completed
				_, cmd = v.updateSelectTools(nil)
			} else {
				v.skillForm = completed
				_, cmd = v.updateSelectSkills(nil)
			}
			require.NotNil(t, cmd)
			switch {
			case tc.abort:
				require.Equal(t, selectionStep, v.step)
				require.IsType(t, tea.QuitMsg{}, cmd())
			case tc.empty:
				require.Equal(t, selectionStep, v.step)
				errMsg, ok := cmd().(tui.ErrorMsg)
				require.True(t, ok)
				require.EqualError(t, errMsg.Err, emptyError)
			default:
				require.Equal(t, nextStep, v.step)
				if selectAgents {
					require.Equal(t, tc.want, skills.ToolNames(v.selectedTools))
				} else {
					require.Equal(t, tc.want, v.selectedSkillNames)
				}
			}
		})
	}
}
