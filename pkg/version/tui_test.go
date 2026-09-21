package version

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
)

func TestHeader(t *testing.T) {
	assert.Equal(t, []string{"ID", "Name", "Status", "Created"}, Header())
}

func TestRow(t *testing.T) {
	createdAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		version *wfclient.WorkflowVersion
		want    []string
	}{
		{
			name: "created status",
			version: &wfclient.WorkflowVersion{
				Id:        "wv-created",
				Name:      "version-created",
				Status:    wfclient.Created,
				CreatedAt: createdAt,
			},
			want: []string{"wv-created", "version-created", "created", createdAt.Format(time.RFC3339)},
		},
		{
			name: "building status",
			version: &wfclient.WorkflowVersion{
				Id:        "wv-building",
				Name:      "version-building",
				Status:    wfclient.Building,
				CreatedAt: createdAt,
			},
			want: []string{"wv-building", "version-building", "building", createdAt.Format(time.RFC3339)},
		},
		{
			name: "registering status",
			version: &wfclient.WorkflowVersion{
				Id:        "wv-registering",
				Name:      "version-registering",
				Status:    wfclient.Registering,
				CreatedAt: createdAt,
			},
			want: []string{"wv-registering", "version-registering", "registering", createdAt.Format(time.RFC3339)},
		},
		{
			name: "ready status",
			version: &wfclient.WorkflowVersion{
				Id:        "wv-ready",
				Name:      "version-ready",
				Status:    wfclient.Ready,
				CreatedAt: createdAt,
			},
			want: []string{"wv-ready", "version-ready", "ready", createdAt.Format(time.RFC3339)},
		},
		{
			name: "build_failed status",
			version: &wfclient.WorkflowVersion{
				Id:        "wv-build-failed",
				Name:      "version-build-failed",
				Status:    wfclient.BuildFailed,
				CreatedAt: createdAt,
			},
			want: []string{"wv-build-failed", "version-build-failed", "build_failed", createdAt.Format(time.RFC3339)},
		},
		{
			name: "registration_failed status",
			version: &wfclient.WorkflowVersion{
				Id:        "wv-registration-failed",
				Name:      "version-registration-failed",
				Status:    wfclient.RegistrationFailed,
				CreatedAt: createdAt,
			},
			want: []string{"wv-registration-failed", "version-registration-failed", "registration_failed", createdAt.Format(time.RFC3339)},
		},
		{
			name: "empty name yields empty cell",
			version: &wfclient.WorkflowVersion{
				Id:        "wv-no-name",
				Name:      "",
				Status:    wfclient.Ready,
				CreatedAt: createdAt,
			},
			want: []string{"wv-no-name", "", "ready", createdAt.Format(time.RFC3339)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := Row(tt.version)

			require.Len(t, row, len(Header()))
			assert.Equal(t, tt.want, row)
		})
	}
}
