package keyvalue

import (
	"errors"
	"strings"

	types "github.com/render-oss/cli/v2/pkg/types"
)

// KeyValueCreateInput is the raw command input parsed from Cobra flags for KV creation.
type KeyValueCreateInput struct {
	Name                string           `cli:"name"`
	Plan                string           `cli:"plan"`
	Region              *string          `cli:"region"`
	EnvironmentIDOrName *string          `cli:"environment-id"`
	MaxmemoryPolicy     *MaxmemoryPolicy `cli:"maxmemory-policy"`
	IPAllowList         []string         `cli:"ip-allow-list"`
}

// NormalizeAndValidateCreateInput normalizes and validates CLI input for KV creation.
func NormalizeAndValidateCreateInput(input KeyValueCreateInput) (KeyValueCreateInput, error) {
	normalized := NormalizeCreateInput(input)
	if err := normalized.validateNormalized(); err != nil {
		return KeyValueCreateInput{}, err
	}
	return normalized, nil
}

func NormalizeCreateInput(input KeyValueCreateInput) KeyValueCreateInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Plan = strings.TrimSpace(input.Plan)
	input.Region = types.OptionalNonZeroString(input.Region)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	input.MaxmemoryPolicy = types.OptionalAlias(input.MaxmemoryPolicy)
	return input
}

func (s KeyValueCreateInput) validateNormalized() error {
	if s.Name == "" {
		return errors.New("name is required")
	}
	if s.Plan == "" {
		return errors.New("plan is required")
	}
	for _, entry := range s.IPAllowList {
		if _, _, err := types.ParseIPAllowListEntry(entry); err != nil {
			return err
		}
	}
	return nil
}
