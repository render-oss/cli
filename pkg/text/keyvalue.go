package text

import (
	"github.com/render-oss/cli/pkg/client"
)

// KeyValueDetail formats a KV instance detail for text output.
// Does NOT include an action prefix (e.g., "Created" or "Updated") — callers should prepend
// their own action prefix in the formatText closure passed to command.NonInteractive.
func KeyValueDetail(kv *client.KeyValueDetail) string {
	return FormatStringF(
		"Name: %s\nID: %s\nPlan: %s\nRegion: %s\nStatus: %s",
		kv.Name,
		kv.Id,
		string(kv.Plan),
		string(kv.Region),
		string(kv.Status),
	)
}
