package text

import (
	"fmt"

	rstrings "github.com/render-oss/cli/pkg/strings"
)

func workspaceLine(name, id string) string {
	label := rstrings.ResourceLabel(name, id)
	if label == "" {
		return ""
	}
	return fmt.Sprintf("Workspace: %s", label)
}
