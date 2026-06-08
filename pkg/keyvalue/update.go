package keyvalue

import (
	"github.com/render-oss/cli/internal/ipallowlist"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

// UpdateOutcome captures both the pre-update API detail and post-update
// resolved state needed by command output.
type UpdateOutcome struct {
	Before *client.KeyValueDetail
	After  *ResolvedKeyValue
}

func NewKeyValueUpdateOut(before *client.KeyValueDetail, after *ResolvedKeyValue) KeyValueUpdateOut {
	out := KeyValueUpdateOut{
		Data: NewKeyValueOut(after),
	}
	if before == nil {
		return out
	}
	out.Diff = NewKeyValueUpdateDiff(before, &out.Data)
	return out
}

func NewKeyValueUpdateDiff(before *client.KeyValueDetail, after *KeyValueOut) KeyValueUpdateDiff {
	var diff KeyValueUpdateDiff
	if before.Name != after.Name {
		diff.Name = newKeyValueFieldDiff(before.Name, after.Name)
	}
	if before.Plan != after.Plan {
		diff.Plan = newKeyValueFieldDiff(before.Plan, after.Plan)
	}
	if !pointers.Equal(before.Options.MaxmemoryPolicy, after.MaxmemoryPolicy) {
		diff.MaxmemoryPolicy = newKeyValueFieldDiff(
			before.Options.MaxmemoryPolicy,
			after.MaxmemoryPolicy,
		)
	}
	if !ipallowlist.Equal(before.IpAllowList, after.IPAllowList) {
		diff.IPAllowList = newKeyValueFieldDiff(before.IpAllowList, after.IPAllowList)
	}
	return diff
}

func newKeyValueFieldDiff[T any](before, after T) *KeyValueFieldDiff[T] {
	return &KeyValueFieldDiff[T]{
		Before: before,
		After:  after,
	}
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
