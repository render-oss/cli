package postgres

import (
	"strings"

	"github.com/render-oss/cli/pkg/client"
)

// buildReadReplicas translates CLI --read-replica flag values (replica names)
// into the generated REST client request shape.
func buildReadReplicas(names []string) *client.ReadReplicasInput {
	if len(names) == 0 {
		return nil
	}
	replicas := make(client.ReadReplicasInput, 0, len(names))
	for _, name := range names {
		replicas = append(replicas, client.ReadReplicaInput{Name: strings.TrimSpace(name)})
	}
	return &replicas
}
