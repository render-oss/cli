package postgres

import (
	"fmt"
	"strings"
)

type UpdatePostgresInput struct {
	IDOrName           string   `cli:"arg:0"`
	Name               *string  `cli:"name"`
	Plan               *string  `cli:"plan"`
	HighAvailability   *bool    `cli:"high-availability"`
	DiskSizeGB         *int     `cli:"disk-size-gb"`
	DiskAutoscaling    *bool    `cli:"disk-autoscaling"`
	DatadogAPIKey      *string  `cli:"datadog-api-key"`
	DatadogSite        *string  `cli:"datadog-site"`
	ParameterOverrides []string `cli:"parameter-override"`
	ReadReplicas       []string `cli:"read-replica"`
}

func (u UpdatePostgresInput) Validate(interactive bool) error {
	if u.IDOrName == "" {
		return fmt.Errorf("postgres ID or name argument is required")
	}
	for _, po := range u.ParameterOverrides {
		if _, _, ok := strings.Cut(po, "="); !ok {
			return fmt.Errorf("invalid --parameter-override %q: expected KEY=VALUE format", po)
		}
	}
	return nil
}
