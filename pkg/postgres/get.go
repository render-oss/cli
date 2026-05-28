package postgres

import "github.com/render-oss/cli/pkg/client"

// GetResult is the shape returned to callers (and serialized to JSON/YAML)
// describing a fetched Postgres database together with its connection info.
type GetResult struct {
	Postgres       *client.PostgresDetail         `json:"postgres"`
	ConnectionInfo *client.PostgresConnectionInfo `json:"connectionInfo"`
}
