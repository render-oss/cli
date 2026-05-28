package postgres

import "github.com/render-oss/cli/pkg/types"

// GetPostgresInput is the raw command input parsed from Cobra args and flags
// for Postgres detail lookup.
type GetPostgresInput struct {
	IDOrName                       string  `cli:"arg:0"`
	ProjectIDOrName                *string `cli:"project"`
	EnvironmentIDOrName            *string `cli:"environment"`
	IncludeSensitiveConnectionInfo bool    `cli:"include-sensitive-connection-info"`
}

func NormalizeGetInput(input GetPostgresInput) GetPostgresInput {
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
