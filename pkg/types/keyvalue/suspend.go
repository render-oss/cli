package keyvalue

import (
	types "github.com/render-oss/cli/pkg/types"
)

// KeyValueSuspendInput is the raw command input parsed from Cobra args for KV suspension.
type KeyValueSuspendInput struct {
	IDOrName            string  `cli:"arg:0"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func NormalizeSuspendInput(input KeyValueSuspendInput) KeyValueSuspendInput {
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
