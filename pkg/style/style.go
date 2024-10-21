package style

import "github.com/charmbracelet/lipgloss"

const (
	blue   = "#5b74ff"
	green  = "#12c603"
	orange = "#ffb727"
	red    = "#ff0033"
	grey   = "#a2a2a2"
)

var Title = lipgloss.NewStyle().Foreground(lipgloss.Color(blue)).Bold(true)

const ColorOK = lipgloss.Color(green)
const ColorWarning = lipgloss.Color(orange)
const ColorError = lipgloss.Color(red)
const ColorDeprioritized = lipgloss.Color(grey)
