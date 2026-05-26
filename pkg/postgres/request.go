package postgres

import (
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/client"
)

// buildParameterOverrides translates CLI --parameter-override flag values
// (KEY=VALUE strings, such as max_connections=100) into the generated REST
// client request shape.
func buildParameterOverrides(raw []string) (*client.PostgresParameterOverrides, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	overrides := make(client.PostgresParameterOverrides, len(raw))
	for _, entry := range raw {
		key, value, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("invalid --parameter-override %q: expected KEY=VALUE format", entry)
		}
		overrides[key] = value
	}
	return &overrides, nil
}

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
