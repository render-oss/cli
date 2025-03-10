package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/types"
)

func TestDeployInput_Validate(t *testing.T) {
	var commitID = "commitID"
	var imageURL = "imageURL"

	var testCases = map[string]struct {
		in            types.DeployInput
		isInteractive bool
		expectErr     bool
	}{
		"nothing set": {
			isInteractive: true,
			in:            types.DeployInput{},
		},
		"valid": {
			isInteractive: true,
			in: types.DeployInput{
				ServiceID: "service-id",
				CommitID:  &commitID,
			},
		},
		"commit and image": {
			isInteractive: true,
			in: types.DeployInput{
				ServiceID: "service-id",
				CommitID:  &commitID,
				ImageURL:  &imageURL,
			},
			expectErr: true,
		},
		"commit no service id": {
			isInteractive: true,
			in: types.DeployInput{
				CommitID: &commitID,
			},
			expectErr: true,
		},
		"image no service id": {
			isInteractive: true,
			in: types.DeployInput{
				ImageURL: &imageURL,
			},
			expectErr: true,
		},
		"wait no service id": {
			isInteractive: true,
			in: types.DeployInput{
				Wait: true,
			},
			expectErr: true,
		},
		"non-interactive no service id": {
			isInteractive: false,
			expectErr:     true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.Validate(tc.isInteractive)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
