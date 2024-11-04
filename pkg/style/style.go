package style

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

const (
	blue         = "#5b74ff"
	darkOrange   = "#d49822"
	verySoftBlue = "#bfd5f1"

	ColorWarningDeprioritized = lipgloss.Color(darkOrange)
)

var (
	ColorOK = lipgloss.AdaptiveColor{
		Light: green600,
		Dark:  green200,
	}
	ColorWarning = lipgloss.AdaptiveColor{
		Light: orange500,
		Dark:  orange200,
	}
	ColorError = lipgloss.AdaptiveColor{
		Light: red600,
		Dark:  red200,
	}
	ColorInfo = lipgloss.AdaptiveColor{
		Light: purple600,
		Dark:  purple200,
	}
	ColorDeprioritized = lipgloss.AdaptiveColor{
		Light: gray600,
		Dark:  gray200,
	}

	ColorHighlight = lipgloss.AdaptiveColor{
		Light: purple100,
		Dark:  purple700,
	}
)

var (
	Title        = lipgloss.NewStyle().Foreground(lipgloss.Color(blue)).Bold(true)
	Label        = lipgloss.NewStyle().Foreground(lipgloss.Color(verySoftBlue))
	Status       = lipgloss.NewStyle().Bold(true)
	CommandTitle = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	CommandKey   = lipgloss.NewStyle().Foreground(ColorInfo)
	Highlight    = lipgloss.NewStyle().Background(ColorHighlight)
)

func FormatKeyValue(key, value string) string {
	return fmt.Sprintf("%s %s", Label.Render(key+":"), value)
}
