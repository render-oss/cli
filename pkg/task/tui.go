package task

import (
	wfclient "github.com/render-oss/cli/v2/pkg/client/workflows"
	"github.com/render-oss/cli/v2/pkg/pointers"
)

func Header() []string {
	return []string{"Name", "ID", "Created"}
}

func Row(task *wfclient.Task) []string {
	return []string{
		task.Name,
		task.Id,
		pointers.TimeValue(&task.CreatedAt),
	}
}
