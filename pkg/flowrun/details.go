package flowrun

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	workflows "github.com/render-oss/cli/pkg/client/workflows"
)

// Details formats a flow run for text output.
func Details(run *workflows.FlowRunDetails) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Flow run %s\n", run.Id)
	fmt.Fprintf(&b, "Status: %s\n", run.Status)
	if run.StartedAt != nil {
		fmt.Fprintf(&b, "Started at: %s\n", run.StartedAt.Format(time.RFC3339))
	}
	if run.CompletedAt != nil {
		fmt.Fprintf(&b, "Completed at: %s\n", run.CompletedAt.Format(time.RFC3339))
	}
	if input, err := json.Marshal(run.Input); err == nil {
		fmt.Fprintf(&b, "Input: %s\n", input)
	}

	switch {
	case run.ErrorDetails != nil:
		b.WriteString(ErrorDescription(run.ErrorDetails))
	case run.Error != nil:
		fmt.Fprintf(&b, "Error: %s\n", *run.Error)
	case len(run.Results) > 0:
		if results, err := json.Marshal(run.Results); err == nil {
			fmt.Fprintf(&b, "Results: %s\n", results)
		}
	}

	return b.String()
}

// ErrorDescription explains a structured flow run failure, naming the step and
// what changed for a determinism error, or the limit and value for a limit
// error. It falls back to the API's message for codes it doesn't know.
func ErrorDescription(details *workflows.FlowRunErrorDetails) string {
	switch details.Code {
	case workflows.FlowDeterminismError:
		return determinismDescription(details)
	case workflows.FlowStepLimitExceeded:
		return limitDescription("Step limit reached: the run started too many steps, or one step used up its attempts", details, formatCount)
	case workflows.FlowPayloadTooLarge:
		return limitDescription("Step payload too large: a step's input, output, or error is over the size limit in bytes", details, formatBytes)
	case workflows.FlowHistoryTooLarge:
		return limitDescription("Run history too large: the run's recorded history is over the size limit in bytes", details, formatBytes)
	default:
		return fmt.Sprintf("Error: %s\n", details.Message)
	}
}

func determinismDescription(details *workflows.FlowRunErrorDetails) string {
	var b strings.Builder

	step := "a step"
	if details.Seq != nil {
		step = fmt.Sprintf("step %d", *details.Seq)
		if details.SubKey != nil && *details.SubKey != "" {
			step = fmt.Sprintf("step %d (%s)", *details.Seq, *details.SubKey)
		}
	}

	fmt.Fprintf(&b, "Error: Determinism error at %s: %s\n", step, determinismChange(details.Kind))
	if details.FlowAttempt != nil {
		fmt.Fprintf(&b, "  Recorded by attempt: %d\n", *details.FlowAttempt+1)
	}
	if details.Expected != nil {
		fmt.Fprintf(&b, "  Expected: %s\n", *details.Expected)
	}
	if details.Actual != nil {
		fmt.Fprintf(&b, "  Actual: %s\n", *details.Actual)
	}
	return b.String()
}

func determinismChange(kind *workflows.FlowRunErrorDetailsKind) string {
	if kind == nil {
		return "the replay didn't match the run's history"
	}
	switch *kind {
	case workflows.InputHash:
		return "the step's name or input changed since the original run"
	case workflows.Type:
		return "the step no longer matches the recorded step"
	case workflows.UnknownType:
		return "the step uses a type the replay doesn't recognize"
	default:
		return "the replay didn't match the run's history"
	}
}

func limitDescription(summary string, details *workflows.FlowRunErrorDetails, format func(int64) string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Error: %s\n", summary)
	if details.Limit != nil {
		fmt.Fprintf(&b, "  Limit: %s\n", format(*details.Limit))
	}
	if details.Value != nil {
		fmt.Fprintf(&b, "  Value: %s\n", format(*details.Value))
	}
	return b.String()
}

func formatCount(n int64) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return sign + digits
}

func formatBytes(n int64) string {
	return formatCount(n) + " bytes"
}
