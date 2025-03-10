package types

import "errors"

type DeployInput struct {
	ServiceID  string  `cli:"arg:0"`
	ClearCache bool    `cli:"clear-cache"`
	CommitID   *string `cli:"commit"`
	ImageURL   *string `cli:"image"`
	Wait       bool    `cli:"wait"`
}

func (d DeployInput) String() []string {
	return []string{d.ServiceID}
}

func (d DeployInput) Validate(isInteractive bool) error {
	if isNonZeroString(d.CommitID) && isNonZeroString(d.ImageURL) {
		return errors.New("only one of commit or image may be specified")
	}

	if d.ServiceID == "" {
		if isNonZeroString(d.ImageURL) {
			return errors.New("service id must be specified when image is specified")
		}
		if isNonZeroString(d.CommitID) {
			return errors.New("service id must be specified when commit is specified")
		}
		if d.Wait {
			return errors.New("service id must be specified when wait is true")
		}
		if !isInteractive {
			return errors.New("service id must be specified when output is not interactive")
		}
	}
	return nil
}

func isNonZeroString(s *string) bool {
	return s != nil && *s != ""
}
