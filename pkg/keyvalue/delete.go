package keyvalue

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
)

// DeleteResult is the shape returned to callers (and serialized to JSON/YAML)
// describing the outcome of a delete attempt. Deleted is false when the caller
// only fetched the target (e.g. for a CLI-level preview); true after the
// resource has been removed.
type DeleteResult struct {
	KeyValue *client.KeyValueDetail `json:"keyValue"`
	Deleted  bool                   `json:"deleted"`
}

// Delete removes the Key Value instance with the given ID via the Render API.
func Delete(ctx context.Context, id string) error {
	c, err := client.NewDefaultClient()
	if err != nil {
		return err
	}
	return NewRepo(c).DeleteKeyValue(ctx, id)
}
