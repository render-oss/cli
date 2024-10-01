package types

type DeployInput struct {
	ServiceID  string
	ClearCache *bool
	CommitID   *string
	ImageURL   *string
}

func (d DeployInput) String() []string {
	return []string{d.ServiceID}
}
