package types

type DeployInput struct {
	ServiceID  string
	ClearCache bool    `cli:"clear-cache"`
	CommitID   *string `cli:"commit"`
	ImageURL   *string `cli:"image"`
}

func (d DeployInput) String() []string {
	return []string{d.ServiceID}
}
