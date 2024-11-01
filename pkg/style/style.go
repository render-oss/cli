package style

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

const (
	blue   = "#5b74ff"
	lightBlue = "#9dc6fa"
	green  = "#12c603"
	orange = "#ffb727"
	darkOrange = "#d49822"
	lightPurple = "#b3a5ff"
	red    = "#ff0033"
	grey   = "#a2a2a2"
)

var Title = lipgloss.NewStyle().Foreground(lipgloss.Color(blue)).Bold(true)
var Label  = lipgloss.NewStyle().Foreground(lipgloss.Color("#bfd5f1"))
var Value  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
var Status = lipgloss.NewStyle().Bold(true)
var CommandTitle = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
var CommandKey = lipgloss.NewStyle().Foreground(lipgloss.Color(lightBlue))

const ColorOK = lipgloss.Color(green)
const ColorWarning = lipgloss.Color(orange)
const ColorWarningDeprioritized = lipgloss.Color(darkOrange)
const ColorError = lipgloss.Color(red)
const ColorDeprioritized = lipgloss.Color(grey)
const ColorInfo = lipgloss.Color(lightBlue)

func FormatKeyValue(key, value string) string {
	return fmt.Sprintf("%s %s", Label.Render(key+":"), value)
}
