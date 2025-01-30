package deploy

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	rstrings "github.com/render-oss/cli/pkg/strings"
	"github.com/render-oss/cli/pkg/style"
)

type ListItem struct {
	deploy *client.Deploy
}

func (i ListItem) Deploy() *client.Deploy {
	return i.deploy
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
		status = style.Status.Foreground(style.ColorOK)
	case "Inactive":
		status = style.Status.Foreground(style.ColorDeprioritized)
	case "Canceled":
		status = style.Status.Foreground(style.ColorWarning)
	case "Build Failed":
		status = style.Status.Foreground(style.ColorError)
	}

	statusLine := status.Render(statusValue)
	triggerLine := style.FormatKeyValue("Trigger", triggerValue(i.deploy.Trigger))

	timeLine := lipgloss.JoinHorizontal(lipgloss.Left,
		style.FormatKeyValue("Created", pointers.TimeValue(i.deploy.CreatedAt)),
		"   ",
		style.FormatKeyValue("Finished", pointers.TimeValue(i.deploy.FinishedAt)),
	)

	var deployInfoLine string
	if i.deploy.Image != nil {
		deployInfoLine = style.FormatKeyValue("Image", pointers.StringValue(i.deploy.Image.Ref)) + " " +
			style.FormatKeyValue("SHA", pointers.StringValue(i.deploy.Image.Sha))
	} else if i.deploy.Commit != nil {
		deployInfoLine = style.FormatKeyValue("Commit", pointers.StringValue(i.deploy.Commit.Id)) + " " +
			style.FormatKeyValue("Message", rstrings.StripNewlines(pointers.StringValue(i.deploy.Commit.Message)))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		statusLine,
		triggerLine,
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

func deployStatusValue(status *client.DeployStatus) string {
	if status == nil {
		return ""
	}

	statusStr := string(*status)
	if statusStr == "deactivated" {
		return "Inactive"
	}

	return rstrings.TitleCaseValue(statusStr)
}

func triggerValue(trigger *client.DeployTrigger) string {
	if trigger == nil {
		return ""
	}

	triggerStr := string(*trigger)
	words := strings.Split(triggerStr, "_")
	return strings.Join(words, " ")
}

func Header() []string {
	return []string{"Status", "Commit/Image", "Trigger", "Created", "Finished", "ID"}
}

func Row(deploy *client.Deploy) []string {
	var commitOrImage string
	if deploy.Image != nil {
		commitOrImage = pointers.StringValue(deploy.Image.Ref)
	} else if deploy.Commit != nil {
		commitOrImage = pointers.StringValue(deploy.Commit.Id)
	}

	return []string{
		deployStatusValue(deploy.Status),
		commitOrImage,
		triggerValue(deploy.Trigger),
		pointers.TimeValue(deploy.CreatedAt),
		pointers.TimeValue(deploy.FinishedAt),
		deploy.Id,
	}
}
