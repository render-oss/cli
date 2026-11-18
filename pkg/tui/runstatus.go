package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	renderstyle "github.com/render-oss/cli/pkg/style"
)

func RunStatusLabel(status string) string {
	switch status {
	case "pending":
		return "Queued"
	case "running", "paused":
		return "In Progress"
	case "sleeping":
		return "Sleeping"
	case "succeeded", "completed":
		return "Succeeded"
	case "failed":
		return "Failed"
	case "canceled":
		return "Canceled"
	default:
		return status
	}
}

func RunStatusStyle(status string) lipgloss.Style {
	switch status {
	case "succeeded", "completed":
		return renderstyle.Status.Foreground(renderstyle.ColorOK)
	case "failed":
		return renderstyle.Status.Foreground(renderstyle.ColorError)
	case "running", "paused":
		return renderstyle.Status.Foreground(renderstyle.ColorInfo)
	case "sleeping":
		return renderstyle.Status.Foreground(renderstyle.ColorWarningDeprioritized)
	default:
		return renderstyle.Status.Foreground(renderstyle.ColorDeprioritized)
	}
}

type runStatusTableValue struct {
	raw   string
	label string
}

func RunStatusTableValue(status string) fmt.Formatter {
	return runStatusTableValue{
		raw:   status,
		label: RunStatusLabel(status),
	}
}

func (status runStatusTableValue) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte(status.label))
}

func (status runStatusTableValue) String() string {
	if strings.EqualFold(status.raw, status.label) {
		return status.label
	}
	return status.label + " " + status.raw
}
