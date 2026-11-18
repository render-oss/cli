package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSandboxGroupsEnvironmentRemove(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want sandboxGroupsEnvironmentOutcome
	}{
		{
			name: "group in an environment",
			args: []string{
				"sandbox-groups", "environment", "remove",
				sandboxGroupsStagingGroupID,
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Requests: []sandboxGroupsEnvironmentRequest{
					{
						Method: http.MethodDelete,
						URI: environmentResourcesURI(sandboxGroupsStagingEnvID) +
							"?resourceIds=" + sandboxGroupsStagingGroupID,
					},
				},
			},
		},
		{
			name: "group without an environment changes nothing",
			args: []string{
				"sandbox-groups", "environment", "remove",
				sandboxGroupsUnboundGroupID,
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{},
		},
		{
			name: "unknown group",
			args: []string{
				"sandbox-groups", "environment", "remove",
				sandboxGroupsUnknownGroupID,
				"--output", "json",
			},
			want: sandboxGroupsEnvironmentOutcome{
				Err: "received response code 404: sandbox group not found",
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

func TestSandboxGroupsEnvironmentRemove_ProtectedEnvironment(t *testing.T) {
	// arrange
	server := newSandboxGroupsEnvironmentServer(t)
	server.Environments.RespondWithErrorForResources(http.StatusForbidden)

	// act
	result, err := executeSandboxGroupsCommand(t, server,
		"sandbox-groups", "environment", "remove",
		sandboxGroupsStagingGroupID,
		"--output", "json",
	)

	// assert
	assert.Equal(t, sandboxGroupsEnvironmentOutcome{
		Err: "you are not allowed to take this action",
		Requests: []sandboxGroupsEnvironmentRequest{
			{
				Method: http.MethodDelete,
				URI: environmentResourcesURI(sandboxGroupsStagingEnvID) +
					"?resourceIds=" + sandboxGroupsStagingGroupID,
			},
		},
	}, sandboxGroupsEnvironmentOutcomeOf(t, server, result, err))
}
