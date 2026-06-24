package postgres

import (
	"fmt"

	"github.com/render-oss/cli/pkg/types"
)

// UpdatePostgresInput is the raw command input parsed from Cobra flags for
// `pg update`.
//
// Target fields (IDOrName, ProjectIDOrName, EnvironmentIDOrName) identify the
// database to update and are not themselves mutated. The remaining fields are
// the changes to apply; at least one must be supplied. Pointer fields are nil
// when the flag was not provided; slice fields are empty. In both cases the
// request builder omits the field so the API leaves it unchanged.
type UpdatePostgresInput struct {
	IDOrName            string  `cli:"arg:0"`
	ProjectIDOrName     *string `cli:"project"`
	EnvironmentIDOrName *string `cli:"environment"`

	Name             *string  `cli:"name"`
	Plan             *string  `cli:"plan"`
	HighAvailability *bool    `cli:"high-availability"`
	DiskSizeGB       *int     `cli:"disk-size-gb"`
	DiskAutoscaling  *bool    `cli:"disk-autoscaling"`
	DatadogAPIKey    *string  `cli:"datadog-api-key"`
	DatadogSite      *string  `cli:"datadog-site"`
	IPAllowList      []string `cli:"ip-allow-list"`
	ClearIPAllowList bool     `cli:"clear-ip-allow-list"`
}

func (u UpdatePostgresInput) Validate(interactive bool) error {
	if u.IDOrName == "" {
		return fmt.Errorf("postgres ID or name argument is required")
	}

	hasMutation := u.Name != nil ||
		u.Plan != nil ||
		u.HighAvailability != nil ||
		u.DiskSizeGB != nil ||
		u.DiskAutoscaling != nil ||
		u.DatadogAPIKey != nil ||
		u.DatadogSite != nil ||
		len(u.IPAllowList) > 0 ||
		u.ClearIPAllowList
	if !hasMutation {
		return fmt.Errorf("at least one field must be provided for update")
	}

	if err := ValidateDiskSizeGB(u.DiskSizeGB); err != nil {
		return err
	}

	if len(u.IPAllowList) > 0 && u.ClearIPAllowList {
		return fmt.Errorf("--ip-allow-list and --clear-ip-allow-list are mutually exclusive")
	}
	for _, entry := range u.IPAllowList {
		if _, _, err := types.ParseIPAllowListEntry(entry); err != nil {
			return err
		}
	}

	return nil
}
