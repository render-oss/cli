package renderapi

import (
	"net/http"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestGetUserQueuedError(t *testing.T) {
	server := NewServer(t)
	user := server.SetCurrentUser(NewUser(client.User{Id: "usr-d123456789abcdefghij"}))
	server.RespondToGetUsersWithError(http.StatusUnauthorized)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)

	resp, err := c.GetUserWithResponse(t.Context())
	require.NoError(t, err)
	require.ErrorIs(t, client.ErrorFromResponse(resp), client.ErrUnauthorized)

	resp, err = c.GetUserWithResponse(t.Context())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.Equal(t, &user, resp.JSON200)
}
