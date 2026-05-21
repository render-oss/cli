package keyvalue

import (
	types "github.com/render-oss/cli/pkg/types"
)

// KeyValueGetInput is the raw command input parsed from Cobra args for fetching a KV instance.
type KeyValueGetInput struct {
	IDOrName                       string  `cli:"arg:0"`
	ProjectIDOrName                *string `cli:"project"`
	EnvironmentIDOrName            *string `cli:"environment"`
	IncludeSensitiveConnectionInfo bool    `cli:"include-sensitive-connection-info"`
}

func NormalizeGetInput(input KeyValueGetInput) KeyValueGetInput {
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
