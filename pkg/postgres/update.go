package postgres

import (
	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/types"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

// UpdateResult captures both the pre- and post-update Postgres state so callers
// can show users a diff of what changed. JSON/YAML consumers also get strictly
// more information than just the new state.
type UpdateResult struct {
	Before *client.PostgresDetail `json:"before"`
	After  *ResolvedPostgres      `json:"after"`
}

// BuildUpdateRequest converts an UpdatePostgresInput into the API PATCH body.
// Only non-nil/non-empty fields are set (partial update). The IP allow-list
// IP allow-list behavior:
//
//   - input.IPAllowList non-empty → body.IpAllowList = &<entries>   (replace)
//   - input.ClearIPAllowList true → body.IpAllowList = &[]          (clear)
//   - neither                     → body.IpAllowList = nil          (leave alone)
//
// Callers are responsible for validating the input first via
// UpdatePostgresInput.Validate; BuildUpdateRequest will still surface a CIDR
// parse error for defense in depth.
func BuildUpdateRequest(input pgtypes.UpdatePostgresInput) (client.UpdatePostgresJSONRequestBody, error) {
	body := client.UpdatePostgresJSONRequestBody{}

	if input.Name != nil {
		body.Name = input.Name
	}

	if input.Plan != nil {
		p := pgclient.PostgresPlans(*input.Plan)
		body.Plan = &p
	}

	if input.DiskSizeGB != nil {
		body.DiskSizeGB = input.DiskSizeGB
	}

	if input.DiskAutoscaling != nil {
		body.EnableDiskAutoscaling = input.DiskAutoscaling
	}

	if input.HighAvailability != nil {
		body.EnableHighAvailability = input.HighAvailability
	}

	if input.DatadogAPIKey != nil {
		body.DatadogAPIKey = input.DatadogAPIKey
	}

	if input.DatadogSite != nil {
		body.DatadogSite = input.DatadogSite
	}

	if len(input.IPAllowList) > 0 {
		entries, err := types.ParseIPAllowList(input.IPAllowList)
		if err != nil {
			return client.UpdatePostgresJSONRequestBody{}, err
		}
		body.IpAllowList = &entries
	}

	if input.ClearIPAllowList {
		empty := []client.CidrBlockAndDescription{}
		body.IpAllowList = &empty
	}

	return body, nil
}
