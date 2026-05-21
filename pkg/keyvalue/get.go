package keyvalue

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
)

// GetResult is the shape returned to callers (and serialized to JSON/YAML)
// describing a fetched Key Value instance together with its connection info.
type GetResult struct {
	KeyValue       *client.KeyValueDetail         `json:"keyValue"`
	ConnectionInfo *client.KeyValueConnectionInfo `json:"connectionInfo"`
}

// GetConnectionInfo fetches connection info for the given Key Value ID.
func GetConnectionInfo(ctx context.Context, id string) (*client.KeyValueConnectionInfo, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	return NewRepo(c).GetKeyValueConnectionInfo(ctx, id)
}
