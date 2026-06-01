package postgres

import "github.com/render-oss/cli/pkg/types"

// ResumePostgresInput is the raw command input parsed from Cobra args and flags
// for resuming a suspended Postgres database.
type ResumePostgresInput struct {
	IDOrName            string  `cli:"arg:0"`
	ProjectIDOrName     *string `cli:"project"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func NormalizeResumeInput(input ResumePostgresInput) ResumePostgresInput {
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}
