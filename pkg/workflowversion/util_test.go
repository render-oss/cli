package workflowversion_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	wfclient "github.com/render-oss/cli/v2/pkg/client/workflows"
	"github.com/render-oss/cli/v2/pkg/workflowversion"
)

func TestIsComplete(t *testing.T) {
	tests := map[wfclient.WorkflowVersionStatus]bool{
		wfclient.BuildFailed:        true,
		wfclient.Building:           false,
		wfclient.Created:            false,
		wfclient.Ready:              true,
		wfclient.Registering:        false,
		wfclient.RegistrationFailed: true,
	}

	for status, expected := range tests {
		t.Run(string(status), func(t *testing.T) {
			actual := workflowversion.IsComplete(status)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestIsSuccessful(t *testing.T) {
	tests := map[wfclient.WorkflowVersionStatus]bool{
		wfclient.BuildFailed:        false,
		wfclient.Building:           false,
		wfclient.Created:            false,
		wfclient.Ready:              true,
		wfclient.Registering:        false,
		wfclient.RegistrationFailed: false,
	}

	for status, expected := range tests {
		t.Run(string(status), func(t *testing.T) {
			actual := workflowversion.IsSuccessful(status)
			assert.Equal(t, expected, actual)
		})
	}
}
