package postgres

// ListInput describes optional active-workspace filters for listing Postgres
// databases.
type ListInput struct {
	ProjectIDOrName     *string
	EnvironmentIDOrName *string
}

func (i ListInput) HasFilter() bool {
	return i.ProjectIDOrName != nil || i.EnvironmentIDOrName != nil
}
