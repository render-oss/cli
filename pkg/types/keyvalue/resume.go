package keyvalue

import (
	types "github.com/render-oss/cli/pkg/types"
)

// KeyValueResumeInput is the raw command input parsed from Cobra args for KV resumption.
type KeyValueResumeInput struct {
	IDOrName            string  `cli:"arg:0"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func NormalizeResumeInput(input KeyValueResumeInput) KeyValueResumeInput {
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
