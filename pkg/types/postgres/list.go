package postgres

import "github.com/render-oss/cli/pkg/types"

// ListPostgresInput is the raw command input parsed from Cobra flags for
// Postgres listing.
type ListPostgresInput struct {
	ProjectIDOrName     *string `cli:"project"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func NormalizeListInput(input ListPostgresInput) ListPostgresInput {
	input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
	input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)
	return input
}

func (i ListPostgresInput) HasFilter() bool {
	return i.ProjectIDOrName != nil || i.EnvironmentIDOrName != nil
}
