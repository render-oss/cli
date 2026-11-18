package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSandboxGroupsEnvironmentSet(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want sandboxGroupsEnvironmentOutcome
	}{
		{
			name: "by name",
			args: []string{
				"sandbox-groups", "environment", "set",
				sandboxGroupsUnboundGroupID, "production",
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Requests: []sandboxGroupsEnvironmentRequest{
					{
						Method: http.MethodPost,
						URI:    environmentResourcesURI(sandboxGroupsProductionEnvID),
						Body:   resourceIDsBody(sandboxGroupsUnboundGroupID),
					},
				},
				OutputEnvironmentID: sandboxGroupsProductionEnvID,
			},
		},
		{
			name: "by ID",
			args: []string{
				"sandbox-groups", "environment", "set",
				sandboxGroupsUnboundGroupID, sandboxGroupsProductionEnvID,
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Requests: []sandboxGroupsEnvironmentRequest{
					{
						Method: http.MethodPost,
						URI:    environmentResourcesURI(sandboxGroupsProductionEnvID),
						Body:   resourceIDsBody(sandboxGroupsUnboundGroupID),
					},
				},
				OutputEnvironmentID: sandboxGroupsProductionEnvID,
			},
		},
		{
			name: "move to another environment",
			args: []string{
				"sandbox-groups", "environment", "set",
				sandboxGroupsStagingGroupID, "production",
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Requests: []sandboxGroupsEnvironmentRequest{
					{
						Method: http.MethodPost,
						URI:    environmentResourcesURI(sandboxGroupsProductionEnvID),
						Body:   resourceIDsBody(sandboxGroupsStagingGroupID),
					},
				},
				OutputEnvironmentID: sandboxGroupsProductionEnvID,
			},
		},
		{
			name: "unknown environment name",
			args: []string{
				"sandbox-groups", "environment", "set",
				sandboxGroupsUnboundGroupID, "qa",
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Err: `no environment found with name "qa"`,
			},
		},
		{
			name: "environment ID in another workspace",
			args: []string{
				"sandbox-groups", "environment", "set",
				sandboxGroupsUnboundGroupID, sandboxGroupsOtherEnvID,
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Err: `environment "` + sandboxGroupsOtherEnvID + `"` +
					` belongs to workspace "` + sandboxGroupsOtherWorkspaceID + `",` +
					` not workspace "` + sandboxGroupsActiveWorkspaceID + `"`,
			},
		},
		{
			name: "unknown group",
			args: []string{
				"sandbox-groups", "environment", "set",
				sandboxGroupsUnknownGroupID, "production",
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Err: "received response code 404: sandbox group not found",
				Requests: []sandboxGroupsEnvironmentRequest{
					{
						Method: http.MethodPost,
						URI:    environmentResourcesURI(sandboxGroupsProductionEnvID),
						Body:   resourceIDsBody(sandboxGroupsUnknownGroupID),
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			server := newSandboxGroupsEnvironmentServer(t)

			// act
			result, err := executeSandboxGroupsCommand(t, server, tc.args...)

			// assert
			assert.Equal(t, tc.want, sandboxGroupsEnvironmentOutcomeOf(t, server, result, err))
		})
	}
}

func TestSandboxGroupsEnvironmentSet_ProtectedEnvironment(t *testing.T) {
	// arrange
	server := newSandboxGroupsEnvironmentServer(t)
	server.Environments.RespondWithErrorForResources(http.StatusForbidden)

	// act
	result, err := executeSandboxGroupsCommand(t, server,
		"sandbox-groups", "environment", "set",
		sandboxGroupsUnboundGroupID, "production",
		"--output", "json",
	)

	// assert
	assert.Equal(t, sandboxGroupsEnvironmentOutcome{
		Err: "you are not allowed to take this action",
		Requests: []sandboxGroupsEnvironmentRequest{
			{
				Method: http.MethodPost,
				URI:    environmentResourcesURI(sandboxGroupsProductionEnvID),
				Body:   resourceIDsBody(sandboxGroupsUnboundGroupID),
			},
		},
	}, sandboxGroupsEnvironmentOutcomeOf(t, server, result, err))
}
