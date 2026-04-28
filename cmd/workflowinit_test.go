package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/workflows/scaffold"
)

func TestShellWrap(t *testing.T) {
	tests := []struct {
		name   string
		cmd    string
		width  int
		indent int
		want   string
	}{
		{
			name:   "fits on one line",
			cmd:    "pip install -r requirements.txt",
			width:  40,
			indent: 3,
			want:   "pip install -r requirements.txt",
		},
		{
			name:   "wraps with backslash",
			cmd:    "cd ./my-project && python3 -m venv .venv && source .venv/bin/activate",
			width:  40,
			indent: 3,
			want:   "cd ./my-project && python3 -m venv .venv \\\n     && source .venv/bin/activate",
		},
		{
			name:   "single long word exceeds width",
			cmd:    "superlongcommandthatcannotbesplit",
			width:  10,
			indent: 3,
			want:   "superlongcommandthatcannotbesplit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellWrap(tt.cmd, tt.width, tt.indent)
			if got != tt.want {
				t.Errorf("shellWrap(%q, %d, %d)\n  got:  %q\n  want: %q", tt.cmd, tt.width, tt.indent, got, tt.want)
			}
		})
	}
}

func TestFormatNextSteps_RendersAllSteps(t *testing.T) {
	result := &scaffold.Result{
		BuildCommand: "pip install",
		StartCommand: "python main.py",
		NextSteps: []scaffold.NextStep{
			{Label: "Build your project", Command: "{{buildCommand}}"},
			{Label: "Start the server", Command: "render workflows dev -- {{startCommand}}"},
			{Label: "Check it out", Hint: "Visit the dashboard"},
		},
	}

	out := ansi.Strip(formatNextSteps(result, "./my-project", false))

	assert.Contains(t, out, "1.")
	assert.Contains(t, out, "2.")
	assert.Contains(t, out, "3.")
	assert.Contains(t, out, "Build your project")
	assert.Contains(t, out, "Start the server")
	assert.Contains(t, out, "Check it out")
	assert.Contains(t, out, "Visit the dashboard")
}

func TestFormatNextSteps_InterpolatesPlaceholders(t *testing.T) {
	result := &scaffold.Result{
		BuildCommand: "npm install",
		StartCommand: "node index.js",
		NextSteps: []scaffold.NextStep{
			{
				Label:   "Deploy with {{buildCommand}}",
				Command: "{{startCommand}}",
				Hint:    "Build: {{buildCommand}}, Start: {{startCommand}}",
			},
		},
	}

	out := ansi.Strip(formatNextSteps(result, "./app", false))

	assert.NotContains(t, out, "{{buildCommand}}")
	assert.NotContains(t, out, "{{startCommand}}")
	assert.Contains(t, out, "npm install")
	assert.Contains(t, out, "node index.js")
}

func TestFormatNextSteps_InterpolatesDirPlaceholder(t *testing.T) {
	result := &scaffold.Result{
		BuildCommand: "pip install",
		StartCommand: "python main.py",
		NextSteps: []scaffold.NextStep{
			{Label: "Enter directory", Command: "cd {{dir}}"},
		},
	}

	out := ansi.Strip(formatNextSteps(result, "./my-workflows", false))

	assert.NotContains(t, out, "{{dir}}")
	assert.Contains(t, out, "./my-workflows")
}

func TestWorkflowInitCmd_GitFlagUsesGitNaming(t *testing.T) {
	assert.Contains(t, workflowInitCmd.Flag("git").Usage, "Initialize a Git repository")
}

// splitShellArgs splits a command string into args, handling double-quoted values.
func splitShellArgs(t *testing.T, s string) []string {
	t.Helper()
	var args []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// reassembleCommand joins the multi-line deploy command into a flat arg string
// by stripping continuation backslashes and collapsing whitespace.
func reassembleCommand(lines []string) string {
	var parts []string
	for _, line := range lines {
		parts = append(parts, strings.TrimSuffix(strings.TrimSpace(line), "\\"))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func TestBuildDeployCommand_ParsesAgainstWorkflowCreateCmd(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		runtime  string
		buildCmd string
		runCmd   string
	}{
		{
			name:     "node project",
			dir:      "my-workflow",
			runtime:  "node",
			buildCmd: "npm install",
			runCmd:   "npm start",
		},
		{
			name:     "python project",
			dir:      "my-workflow",
			runtime:  "python",
			buildCmd: "pip install -r requirements.txt",
			runCmd:   "python -m workflows.app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := buildDeployCommand(tt.dir, tt.runtime, tt.buildCmd, tt.runCmd, false)
			cmdLine := reassembleCommand(lines)
			cmdLine = strings.TrimPrefix(cmdLine, "render workflows create ")

			// Build a fresh command with the same flag schema to avoid
			// shared CobraEnum state between subtests.
			cmd := &cobra.Command{Use: "create", RunE: func(c *cobra.Command, args []string) error { return nil }}
			cmd.Flags().String("name", "", "")
			cmd.Flags().String("repo", "", "")
			cmd.Flags().String("build-command", "", "")
			cmd.Flags().String("run-command", "", "")
			cmd.Flags().String("runtime", "", "")

			cmd.SetArgs(splitShellArgs(t, cmdLine))

			err := cmd.Execute()
			require.NoError(t, err, "generated command must parse without errors")

			flags := cmd.Flags()
			val, _ := flags.GetString("name")
			require.Equal(t, tt.dir, val)

			val, _ = flags.GetString("runtime")
			require.Equal(t, tt.runtime, val)

			val, _ = flags.GetString("build-command")
			require.Equal(t, tt.buildCmd, val)

			val, _ = flags.GetString("run-command")
			require.Equal(t, tt.runCmd, val)

			val, _ = flags.GetString("repo")
			require.Equal(t, "<your-repo-url>", val)
		})
	}
}

func TestBuildDeployCommand_IncludesAllRequiredFlags(t *testing.T) {
	lines := buildDeployCommand("test", "node", "npm install", "npm start", false)
	combined := strings.Join(lines, " ")

	for _, flag := range []string{"--name", "--runtime", "--build-command", "--run-command", "--repo"} {
		assert.Contains(t, combined, flag, "deploy command must include %s", flag)
	}
}

func TestFormatNextSteps_IncludesDeployStep(t *testing.T) {
	result := &scaffold.Result{
		Language:           scaffold.TypeScript,
		RenderBuildCommand: "npm install",
		RenderStartCommand: "npm start",
		NextSteps:          []scaffold.NextStep{},
	}

	out := ansi.Strip(formatNextSteps(result, "my-workflow", false))

	assert.Contains(t, out, "render workflows create")
	assert.Contains(t, out, "--name")
	assert.Contains(t, out, "--runtime node")
	assert.Contains(t, out, "<your-repo-url>")
}

func TestFormatNextSteps_UsesLocalRepoWhenGitInitialized(t *testing.T) {
	result := &scaffold.Result{
		Language:           scaffold.TypeScript,
		RenderBuildCommand: "npm install",
		RenderStartCommand: "npm start",
		NextSteps:          []scaffold.NextStep{},
	}

	out := ansi.Strip(formatNextSteps(result, "my-workflow", true))

	assert.Contains(t, out, "--repo .")
	assert.NotContains(t, out, "<your-repo-url>")
	assert.Contains(t, out, "my-workflow directory")
}

func TestBuildDeployCommand_LocalRepoFlag(t *testing.T) {
	lines := buildDeployCommand("test", "node", "npm install", "npm start", true)
	combined := strings.Join(lines, " ")
	assert.Contains(t, combined, "--repo .")
	assert.NotContains(t, combined, "<your-repo-url>")
}

func TestBuildDeployStep(t *testing.T) {
	tests := []struct {
		name           string
		relDir         string
		gitInitialized bool
		wantRepoArg    string
		wantHint       string
		wantName       string
	}{
		{
			name:           "placeholder URL when git not initialized",
			relDir:         "./my-workflow",
			gitInitialized: false,
			wantRepoArg:    "--repo <your-repo-url>",
			wantHint:       "Replace <your-repo-url> with your Git repository URL.",
			wantName:       "my-workflow",
		},
		{
			name:           "local repo when git initialized",
			relDir:         "./my-workflow",
			gitInitialized: true,
			wantRepoArg:    "--repo .",
			wantHint:       "Run this from the ./my-workflow directory after pushing to your Git provider.",
			wantName:       "my-workflow",
		},
		{
			name:           "derives name from basename of nested path",
			relDir:         "./some/nested/project",
			gitInitialized: true,
			wantRepoArg:    "--repo .",
			wantHint:       "Run this from the ./some/nested/project directory after pushing to your Git provider.",
			wantName:       "project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildDeployStep(tc.relDir, "node", "npm install", "npm start", tc.gitInitialized)

			assert.Equal(t, "Deploy your workflow on Render:", got.Label)
			assert.Equal(t, tc.wantHint, got.Hint)

			combined := strings.Join(got.CmdLines, " ")
			assert.Contains(t, combined, tc.wantRepoArg)
			assert.Contains(t, combined, fmt.Sprintf("--name %q", tc.wantName))
		})
	}
}

func TestFormatNextSteps_SkipsLegacyDeployStep(t *testing.T) {
	result := &scaffold.Result{
		Language:           scaffold.TypeScript,
		RenderBuildCommand: "npm install",
		RenderStartCommand: "npm start",
		NextSteps: []scaffold.NextStep{
			{Label: "Build your project", Command: "npm install"},
			{Label: "Deploy your workflow service to Render", Hint: "Legacy hint from template"},
		},
	}

	out := ansi.Strip(formatNextSteps(result, "my-workflow", false))

	// Legacy step should be filtered out
	assert.NotContains(t, out, "Deploy your workflow service to Render")
	assert.NotContains(t, out, "Legacy hint from template")

	// Remaining template step + our own deploy step should be present
	assert.Contains(t, out, "1. Build your project")
	assert.Contains(t, out, "2.")
	assert.Contains(t, out, "render workflows create")
}
