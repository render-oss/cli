package text

import "github.com/render-oss/cli/v2/pkg/tui/views"

func PSQLResultText(result *views.PSQLResult) string {
	return result.Output
}
