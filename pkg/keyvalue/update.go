package keyvalue

import (
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

// UpdateResult captures both the pre- and post-update KV state so callers can
// show users a diff of what changed. JSON/YAML consumers also get strictly
// more information than just the new state.
type UpdateResult struct {
	Before *client.KeyValueDetail `json:"before"`
	After  *client.KeyValueDetail `json:"after"`
}

// BuildUpdateRequest converts a normalized KeyValueUpdateInput into the API
// PATCH body. The IP allow-list field uses a tri-state to express the user's
// intent:
//
//   - input.IPAllowList non-empty → body.IpAllowList = &<entries>   (replace)
//   - input.ClearIPAllowList true → body.IpAllowList = &[]          (clear)
//   - neither                     → body.IpAllowList = nil          (leave alone)
//
// Callers are responsible for validating the input first via
// kvtypes.NormalizeAndValidateUpdateInput; BuildUpdateRequest will still
// surface a CIDR parse error for defense in depth but does not re-check the
// "at least one field" or mutex rules.
func BuildUpdateRequest(input kvtypes.KeyValueUpdateInput) (client.UpdateKeyValueJSONRequestBody, error) {
	body := client.UpdateKeyValueJSONRequestBody{}

	if input.Name != nil {
		body.Name = input.Name
	}

	if input.Plan != nil {
		p := client.KeyValuePlan(*input.Plan)
		body.Plan = &p
	}

	if input.MaxmemoryPolicy != nil {
		p := client.MaxmemoryPolicy(*input.MaxmemoryPolicy)
		body.MaxmemoryPolicy = &p
	}

	if len(input.IPAllowList) > 0 {
		entries, err := types.ParseIPAllowList(input.IPAllowList)
		if err != nil {
			return client.UpdateKeyValueJSONRequestBody{}, err
		}
		body.IpAllowList = &entries
	}

	if input.ClearIPAllowList {
		empty := []client.CidrBlockAndDescription{}
		body.IpAllowList = &empty
	}

	return body, nil
}
