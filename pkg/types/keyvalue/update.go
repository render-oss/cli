package keyvalue

import (
	"errors"

	types "github.com/render-oss/cli/pkg/types"
)

// KeyValueUpdateInput is the raw command input parsed from Cobra flags for KV update.
//
// Target fields (IDOrName, EnvironmentIDOrName) identify the KV to update and
// are not themselves mutated. The remaining fields are the changes to apply;
// at least one must be supplied.
type KeyValueUpdateInput struct {
	IDOrName            string  `cli:"arg:0"`
	EnvironmentIDOrName *string `cli:"environment"`

	Name             *string          `cli:"name"`
	Plan             *string          `cli:"plan"`
	MaxmemoryPolicy  *MaxmemoryPolicy `cli:"memory-policy"`
	IPAllowList      []string         `cli:"ip-allow-list"`
	ClearIPAllowList bool             `cli:"clear-ip-allow-list"`
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
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	input.Name = types.OptionalNonZeroString(input.Name)
	input.Plan = types.OptionalNonZeroString(input.Plan)
	input.MaxmemoryPolicy = types.OptionalAlias(input.MaxmemoryPolicy)
	if input.MaxmemoryPolicy != nil {
		normalized := NormalizeMemoryPolicy(*input.MaxmemoryPolicy)
		input.MaxmemoryPolicy = &normalized
	}
	return input
}

func (s KeyValueUpdateInput) validateNormalized() error {
	hasMutation := s.Name != nil ||
		s.Plan != nil ||
		s.MaxmemoryPolicy != nil ||
		len(s.IPAllowList) > 0 ||
		s.ClearIPAllowList
	if !hasMutation {
		return errors.New("at least one field must be provided for update")
	}
	if len(s.IPAllowList) > 0 && s.ClearIPAllowList {
		return errors.New("--ip-allow-list and --clear-ip-allow-list are mutually exclusive")
	}
	for _, entry := range s.IPAllowList {
		if _, _, err := types.ParseIPAllowListEntry(entry); err != nil {
			return err
		}
	}
	return nil
}
