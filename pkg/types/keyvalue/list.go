package keyvalue

import "github.com/render-oss/cli/pkg/types"

// KeyValueListInput is the raw command input parsed from Cobra flags for KV listing.
type KeyValueListInput struct {
	ProjectIDOrName     *string `cli:"project"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func NormalizeListInput(input KeyValueListInput) KeyValueListInput {
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
