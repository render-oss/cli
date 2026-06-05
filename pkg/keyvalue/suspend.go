package keyvalue

import "github.com/render-oss/cli/pkg/client"

// SuspendResult is the shape returned to callers (and serialized to JSON/YAML)
// describing the outcome of a suspend attempt. Suspended is false when the
// caller only fetched the target for a dry-run preview; true after the
// instance has been suspended.
type SuspendResult struct {
	KeyValue  *client.KeyValueDetail `json:"keyValue"`
	Suspended bool                   `json:"suspended"`
}
