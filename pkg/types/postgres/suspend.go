package postgres

import "github.com/render-oss/cli/pkg/types"

// SuspendPostgresInput is the raw command input parsed from Cobra args and flags
// for suspending a Postgres database.
type SuspendPostgresInput struct {
	IDOrName            string  `cli:"arg:0"`
	ProjectIDOrName     *string `cli:"project"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func NormalizeSuspendInput(input SuspendPostgresInput) SuspendPostgresInput {
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
