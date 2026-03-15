package postgres

import (
	"fmt"
	"strings"
)

// MinPostgresVersion is the oldest PostgreSQL major version the CLI accepts.
// Versions above this are passed through to the API without further validation.
const MinPostgresVersion = 11

type CreatePostgresInput struct {
	Name                string   `cli:"name"`
	Plan                string   `cli:"plan"`
	Version             int      `cli:"version"`
	Region              *string  `cli:"region"`
	DatabaseName        *string  `cli:"database-name"`
	DatabaseUser        *string  `cli:"database-user"`
	EnvironmentIDOrName *string  `cli:"environment-id"`
	HighAvailability    *bool    `cli:"high-availability"`
	DiskSizeGB          *int     `cli:"disk-size-gb"`
	DiskAutoscaling     *bool    `cli:"disk-autoscaling"`
	DatadogAPIKey       *string  `cli:"datadog-api-key"`
	DatadogSite         *string  `cli:"datadog-site"`
	ParameterOverrides  []string `cli:"parameter-override"`
	ReadReplicas        []string `cli:"read-replica"`
}

func (c CreatePostgresInput) Validate(interactive bool) error {
	if c.Name == "" {
		return fmt.Errorf("--name is required")
	}
	if c.Plan == "" {
		return fmt.Errorf("--plan is required")
	}
	if c.Version == 0 {
		return fmt.Errorf("--version is required")
	}
	if c.Version < MinPostgresVersion {
		return fmt.Errorf("invalid --version %d: must be >= %d", c.Version, MinPostgresVersion)
	}
	for _, po := range c.ParameterOverrides {
		if _, _, ok := strings.Cut(po, "="); !ok {
			return fmt.Errorf("invalid --parameter-override %q: expected KEY=VALUE format", po)
		}
	}
	return nil
}
