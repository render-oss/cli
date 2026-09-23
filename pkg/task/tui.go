package task

import (
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
)

func Header() []string {
	return []string{"Name", "ID", "Workflow ID", "Version ID", "Created"}
}

func Row(task *wfclient.Task) []string {
	return []string{
		task.Name,
		task.Id,
		pointers.StringValue(task.WorkflowId),
		pointers.StringValue(task.WorkflowVersionId),
		pointers.TimeValue(&task.CreatedAt),
	}
}
