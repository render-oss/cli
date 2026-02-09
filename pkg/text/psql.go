package text

import "github.com/render-oss/cli/pkg/tui/views"

func PSQLResultText(result *views.PSQLResult) string {
	return result.Output
}
