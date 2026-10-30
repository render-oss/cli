package text

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/style"
)

func BlueprintCreate(out blueprint.CreateOut) string {
	if out.Data == nil {
		return ""
	}
	result := out.Data
	var b strings.Builder
	writeRawPlan(&b, result.PlanningOutcome, result.Sync)

	bp := result.Blueprint
	fmt.Fprintf(&b, "%s (%s)\n", style.Title.Render("Blueprint "+bp.Name), bp.Id)
	fmt.Fprintf(&b, "%s %s\n", style.SubtleText.Render("Source:"), bp.Repo)
	fmt.Fprintf(&b, "%s %s\n", style.SubtleText.Render("Branch:"), bp.Branch)
	fmt.Fprintf(&b, "%s %s\n", style.SubtleText.Render("Path:"), bp.Path)
	if result.Sync != nil {
		fmt.Fprintf(&b, "%s %s\n", style.SubtleText.Render("Sync:"), result.Sync.Id)
	}
	b.WriteString("\n")
	writePlanningResult(&b, result.PlanningOutcome, result.Notices, result.Sync, out.Meta.Next,
		"The Blueprint was created, but this CLI cannot determine the next step.")
	return b.String()
}

func BlueprintPlan(out blueprint.PlanOut) string {
	if out.Data == nil {
		return ""
	}
	result := out.Data
	var b strings.Builder
	writeRawPlan(&b, result.PlanningOutcome, result.Sync)
	if result.Sync != nil {
		fmt.Fprintf(&b, "%s\n\n", style.Title.Render("Sync "+result.Sync.Id))
	}
	writePlanningResult(&b, result.PlanningOutcome, result.Notices, result.Sync, out.Meta.Next,
		"This CLI cannot determine the next step.")
	return b.String()
}

// writeRawPlan writes the sync's raw plan, when there is one.
func writeRawPlan(b *strings.Builder, outcome bptypes.PlanningOutcome, sync *bptypes.SyncWithPlan) {
	if sync == nil || sync.RawPlan == nil {
		return
	}
	fmt.Fprintln(b, style.Title.Render("Plan"))
	caption := "Planning output for this sync."
	switch outcome {
	case bptypes.PlanningOutcomePlanned:
		caption = "These are the changes that will be applied if this sync is approved."
	case bptypes.PlanningOutcomeFailed:
		caption = "Planning failed. This output may be incomplete; fix the errors below and re-plan before approval."
	}
	fmt.Fprintf(b, "%s\n\n", style.SubtleText.Render(caption))
	plan, err := json.MarshalIndent(sync.RawPlan, "", "  ")
	if err != nil {
		fmt.Fprintf(b, "Could not format the plan: %s\n\n", err)
	} else {
		fmt.Fprintf(b, "%s\n\n", plan)
	}
}

// writePlanningResult writes the planning outcome, notices, required manual
// env vars, Dashboard link, and next step. unknownOutcomeMessage explains an
// unknown PlanningOutcome value.
func writePlanningResult(b *strings.Builder, outcome bptypes.PlanningOutcome, notices []bptypes.BlueprintNotice, sync *bptypes.SyncWithPlan, next *string, unknownOutcomeMessage string) {
	//exhaustive:enforce
	switch outcome {
	case bptypes.PlanningOutcomePlanned:
		fmt.Fprintln(b, style.Status.Foreground(style.ColorInfo).Render("Planned, waiting for approval"))
	case bptypes.PlanningOutcomeNoChanges:
		fmt.Fprintf(b, "%s: the Blueprint already matches its configuration\n", style.Status.Foreground(style.ColorOK).Render("No changes"))
	case bptypes.PlanningOutcomeFailed:
		fmt.Fprintln(b, style.Status.Foreground(style.ColorError).Render("Planning failed"))
	default:
		fmt.Fprintf(b, "%s: %q\n", style.Status.Foreground(style.ColorWarning).Render("Unknown planning outcome"), outcome)
		fmt.Fprintln(b, unknownOutcomeMessage)
	}
	if len(notices) > 0 {
		b.WriteString("\n")
	}
	for _, notice := range notices {
		fmt.Fprintf(b, "  %s\n", blueprintNotice(notice))
	}
	if sync != nil {
		if len(sync.RequiredManualEnvVars) > 0 {
			fmt.Fprintf(b, "\n%s\n", style.Bold("Environment variables required at approval:"))
			for _, envVar := range sync.RequiredManualEnvVars {
				fmt.Fprintf(b, "  %s/%s: %s\n", envVar.ServiceType, envVar.ServiceName, style.Bold(envVar.Key))
			}
		}
		if sync.DashboardUrl != "" {
			fmt.Fprintf(b, "\n%s%s %s\n", style.Bold("Review in Dashboard"), style.SubtleText.Render(" (optional):"), sync.DashboardUrl)
		}
	}
	if next != nil {
		fmt.Fprintf(b, "\n%s\n  %s\n", style.Title.Render("Next:"), *next)
	}
}

func blueprintNotice(notice bptypes.BlueprintNotice) string {
	severity := string(notice.Severity) + ":"
	switch notice.Severity {
	case bptypes.BlueprintNoticeSeverityError:
		severity = style.Status.Foreground(style.ColorError).Render("Error:")
	case bptypes.BlueprintNoticeSeverityWarning:
		severity = style.Status.Foreground(style.ColorWarning).Render("Warning:")
	}
	var location []string
	if notice.Path != nil && *notice.Path != "" {
		location = append(location, *notice.Path)
	}
	if notice.Line != nil && *notice.Line > 0 {
		location = append(location, fmt.Sprintf("line %d", *notice.Line))
	}
	if notice.Column != nil && *notice.Column > 0 {
		location = append(location, fmt.Sprintf("column %d", *notice.Column))
	}
	message := fmt.Sprintf("%s %s", severity, notice.Message)
	if len(location) > 0 {
		message += style.SubtleText.Render(" (" + strings.Join(location, ", ") + ")")
	}
	return message
}

// BlueprintApprove does not distinguish a newly approved sync from one that
// was already pending or running, because the API response does not.
func BlueprintApprove(out blueprint.ApproveOut) string {
	if out.Data == nil {
		return ""
	}
	sync := out.Data
	var b strings.Builder
	fmt.Fprintln(&b, style.Title.Render("Sync "+sync.Id))
	fmt.Fprintf(&b, "%s %s\n", style.SubtleText.Render("Commit:"), sync.Commit.Id)
	fmt.Fprintf(&b, "%s %s\n", style.SubtleText.Render("State:"), sync.State)
	b.WriteString("\n")
	fmt.Fprintln(&b, style.Status.Foreground(style.ColorOK).Render("Approved"))
	if sync.DashboardUrl != "" {
		fmt.Fprintf(&b, "\n%s %s\n", style.Bold("Track in Dashboard:"), sync.DashboardUrl)
	}
	if out.Meta.Next != nil {
		fmt.Fprintf(&b, "\n%s\n  %s\n", style.Title.Render("Next:"), *out.Meta.Next)
	}
	return b.String()
}
