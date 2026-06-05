package keyvalue

import "github.com/render-oss/cli/pkg/client"

// DeleteResult is the shape returned to callers (and serialized to JSON/YAML)
// describing the outcome of a delete attempt. Deleted is false when the caller
// only fetched the target (e.g. for a CLI-level preview); true after the
// resource has been removed.
type DeleteResult struct {
	KeyValue *client.KeyValueDetail `json:"keyValue"`
	Deleted  bool                   `json:"deleted"`
}
