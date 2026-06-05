package keyvalue

import "github.com/render-oss/cli/pkg/client"

// GetResult is the shape returned to callers (and serialized to JSON/YAML)
// describing a fetched Key Value instance together with its connection info.
type GetResult struct {
	KeyValue       *client.KeyValueDetail         `json:"keyValue"`
	ConnectionInfo *client.KeyValueConnectionInfo `json:"connectionInfo"`
}
