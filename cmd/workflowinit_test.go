package cmd

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/render-oss/cli/pkg/workflows/scaffold"
)

func TestShellWrap(t *testing.T) {
	tests := []struct {
		name      string
		cmd       string
		width     int
		indent    int
		want      string
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

	out := ansi.Strip(formatNextSteps(result, "./my-project"))

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

	out := ansi.Strip(formatNextSteps(result, "./app"))

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

	out := ansi.Strip(formatNextSteps(result, "./my-workflows"))

	assert.NotContains(t, out, "{{dir}}")
	assert.Contains(t, out, "./my-workflows")
}
