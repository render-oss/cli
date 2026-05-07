package keyvalue

import (
	"errors"
	"strings"

	types "github.com/render-oss/cli/pkg/types"
)

// KeyValueCreateInput is the raw command input parsed from Cobra flags for KV creation.
type KeyValueCreateInput struct {
	Name                string           `cli:"name"`
	Plan                Plan             `cli:"plan"`
	Region              *string          `cli:"region"`
	ProjectIDOrName     *string          `cli:"project"`
	EnvironmentIDOrName *string          `cli:"environment"`
	MaxmemoryPolicy     *MaxmemoryPolicy `cli:"memory-policy"`
	IPAllowList         []string         `cli:"ip-allow-list"`
	WorkspaceIDOrName   string           `cli:"workspace"`
}

// KeyValueCreateRequestInput is the resolved, API-ready input for building a create-KV request.
type KeyValueCreateRequestInput struct {
	Name            string
	OwnerID         string
	Plan            Plan
	Region          *string
	EnvironmentID   *string
	MaxmemoryPolicy *MaxmemoryPolicy
	IPAllowList     []string
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
	input.Plan = Plan(strings.TrimSpace(string(input.Plan)))
	input.WorkspaceIDOrName = strings.TrimSpace(input.WorkspaceIDOrName)
	input.Region = types.OptionalNonZeroString(input.Region)
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	input.MaxmemoryPolicy = types.OptionalAlias(input.MaxmemoryPolicy)
	if input.MaxmemoryPolicy != nil {
		normalized := NormalizeMemoryPolicy(*input.MaxmemoryPolicy)
		input.MaxmemoryPolicy = &normalized
	}
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
