package postgres

import (
	"fmt"

	"github.com/render-oss/cli/pkg/types"
)

// CreatePostgresInput is the raw command input parsed from Cobra flags for
// `render pg create`. Every field is optional: a user can run the command
// with no flags and get a working database. Defaults are filled in
// client-side (name, plan, version, disk size) and server-side
// (region, db name, db user, etc.).
type CreatePostgresInput struct {
	Name                string   `cli:"name"`
	Plan                string   `cli:"plan"`
	Version             *int     `cli:"version"`
	Region              *string  `cli:"region"`
	WorkspaceIDOrName   string   `cli:"workspace"`
	ProjectIDOrName     *string  `cli:"project"`
	EnvironmentIDOrName *string  `cli:"environment"`
	DatabaseName        *string  `cli:"database-name"`
	DatabaseUser        *string  `cli:"database-user"`
	HighAvailability    *bool    `cli:"high-availability"`
	DiskSizeGB          *int     `cli:"disk-size-gb"`
	DiskAutoscaling     *bool    `cli:"disk-autoscaling"`
	DatadogAPIKey       *string  `cli:"datadog-api-key"`
	DatadogSite         *string  `cli:"datadog-site"`
	IPAllowList         []string `cli:"ip-allow-list"`
	ReadReplicas        []string `cli:"read-replica"`
}

func (c CreatePostgresInput) Validate(interactive bool) error {
	if err := ValidateDiskSizeGB(c.DiskSizeGB); err != nil {
		return err
	}
	for _, entry := range c.IPAllowList {
		if _, _, err := types.ParseIPAllowListEntry(entry); err != nil {
			return err
		}
	}
	return nil
}

// ValidateDiskSizeGB enforces the API rule: 1 GB or any multiple of 5 GB.
// A nil pointer is valid and means "let the server pick".
func ValidateDiskSizeGB(size *int) error {
	if size == nil {
		return nil
	}
	v := *size
	if v == 1 {
		return nil
	}
	if v >= 5 && v%5 == 0 {
		return nil
	}
	return fmt.Errorf("invalid --disk-size-gb %d: must be 1 or a multiple of 5", v)
}
