package style

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

const (
	blue   = "#5b74ff"
	green  = "#12c603"
	orange = "#ffb727"
	red    = "#ff0033"
	grey   = "#a2a2a2"
)

var Title = lipgloss.NewStyle().Foreground(lipgloss.Color(blue)).Bold(true)
var Label  = lipgloss.NewStyle().Foreground(lipgloss.Color("#bfd5f1"))
var Value  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
var Status = lipgloss.NewStyle().Bold(true)

const ColorOK = lipgloss.Color(green)
const ColorWarning = lipgloss.Color(orange)
const ColorError = lipgloss.Color(red)
const ColorDeprioritized = lipgloss.Color(grey)

func FormatKeyValue(key, value string) string {
	return fmt.Sprintf("%s %s", Label.Render(key+":"), value)
}
