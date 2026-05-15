package keyvalue

import (
	types "github.com/render-oss/cli/pkg/types"
)

// KeyValueDeleteInput is the raw command input parsed from Cobra args for KV deletion.
type KeyValueDeleteInput struct {
	IDOrName            string  `cli:"arg:0"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func NormalizeDeleteInput(input KeyValueDeleteInput) KeyValueDeleteInput {
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
