package deploy

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/style"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#bfd5f1"))
	valueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	statusStyle = lipgloss.NewStyle().Bold(true)
)

type ListItem struct {
	deploy *client.Deploy
}

func NewListItem(d *client.Deploy) ListItem {
	return ListItem{deploy: d}
}

func (i ListItem) Title() string {
	return style.Title.Render(i.deploy.Id)
}

func (i ListItem) Description() string {
	statusValue := deployStatusValue(i.deploy.Status)
	var status lipgloss.Style
	switch statusValue {
	case "Live":
		status = statusStyle.Foreground(style.ColorOK)
	case "Inactive":
		status = statusStyle.Foreground(style.ColorDeprioritized)
	case "Canceled":
		status = statusStyle.Foreground(style.ColorWarning)
	case "Build Failed":
		status = statusStyle.Foreground(style.ColorError)
	}

	statusLine := status.Render(statusValue)
	triggerLine := fmt.Sprintf("Triggered by %s", triggerValue(i.deploy.Trigger))

	timeLine := lipgloss.JoinHorizontal(lipgloss.Left,
		formatKeyValue("Created", timeValue(i.deploy.CreatedAt)),
		"   ",
		formatKeyValue("Finished", timeValue(i.deploy.FinishedAt)),
	)

	var deployInfoLine string
	if i.deploy.Image != nil {
		deployInfoLine = formatKeyValue("Image", stringValue(i.deploy.Image.Ref)) + " " +
			formatKeyValue("SHA", stringValue(i.deploy.Image.Sha))
	} else if i.deploy.Commit != nil {
		deployInfoLine = formatKeyValue("Commit", stringValue(i.deploy.Commit.Id)) + " " +
			formatKeyValue("Message", stripNewlines(stringValue(i.deploy.Commit.Message)))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Left, statusLine, "  |  ", triggerLine),
		timeLine,
		deployInfoLine,
	)
}

func (i ListItem) FilterValue() string {
	return i.deploy.Id
}

func (i ListItem) Height() int {
	return 5
}

func formatKeyValue(key, value string) string {
	return fmt.Sprintf("%s %s", labelStyle.Render(key+":"), valueStyle.Render(value))
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func timeValue(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func deployStatusValue(status *client.DeployStatus) string {
	if status == nil {
		return ""
	}

	statusStr := string(*status)
	if statusStr == "deactivated" {
		return "Inactive"
	}

	words := strings.Split(statusStr, "_")
	caser := cases.Title(language.English)
	for i, word := range words {
		words[i] = caser.String(word)
	}
	return strings.Join(words, " ")
}

func triggerValue(trigger *client.DeployTrigger) string {
	if trigger == nil {
		return ""
	}

	triggerStr := string(*trigger)
	words := strings.Split(triggerStr, "_")
	return strings.Join(words, " ")
}

func stripNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", "")
}
