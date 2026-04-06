package keyvalue

import (
	"errors"

	types "github.com/render-oss/cli/pkg/types"
)

// KeyValueUpdateInput is the raw command input parsed from Cobra flags for KV update.
type KeyValueUpdateInput struct {
	Name            *string          `cli:"name"`
	Plan            *string          `cli:"plan"`
	MaxmemoryPolicy *MaxmemoryPolicy `cli:"maxmemory-policy"`
	IPAllowList     []string         `cli:"ip-allow-list"`
}

// NormalizeAndValidateUpdateInput normalizes and validates CLI input for KV update.
func NormalizeAndValidateUpdateInput(input KeyValueUpdateInput) (KeyValueUpdateInput, error) {
	normalized := NormalizeUpdateInput(input)
	if err := normalized.validateNormalized(); err != nil {
		return KeyValueUpdateInput{}, err
	}
	return normalized, nil
}

func NormalizeUpdateInput(input KeyValueUpdateInput) KeyValueUpdateInput {
	input.Name = types.OptionalNonZeroString(input.Name)
	input.Plan = types.OptionalNonZeroString(input.Plan)
	input.MaxmemoryPolicy = types.OptionalAlias(input.MaxmemoryPolicy)
	return input
}

func (s KeyValueUpdateInput) validateNormalized() error {
	if s.Name == nil && s.Plan == nil && s.MaxmemoryPolicy == nil && len(s.IPAllowList) == 0 {
		return errors.New("at least one field must be provided for update")
	}
	for _, entry := range s.IPAllowList {
		if _, _, err := types.ParseIPAllowListEntry(entry); err != nil {
			return err
		}
	}
	return nil
}
