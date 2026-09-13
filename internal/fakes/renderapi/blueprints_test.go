package renderapi

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func TestBlueprintValidationResponsesAreQueuedBeforeDefault(t *testing.T) {
	server := NewServer(t)
	server.Blueprints.RespondWithValidation(bptypes.ValidateBlueprintResponse{Valid: true})
	server.Blueprints.RespondWithRawValidation("text/plain", []byte("unexpected response"))

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)

	first, err := c.ValidateBlueprintWithBodyWithResponse(context.Background(), "multipart/form-data", bytes.NewReader(nil))
	require.NoError(t, err)
	require.NotNil(t, first.JSON200)
	require.True(t, first.JSON200.Valid)

	second, err := c.ValidateBlueprintWithBodyWithResponse(context.Background(), "multipart/form-data", bytes.NewReader(nil))
	require.NoError(t, err)
	require.Nil(t, second.JSON200)
	require.NotNil(t, second.HTTPResponse)
	require.Equal(t, http.StatusOK, second.StatusCode())
	require.Equal(t, "text/plain", second.ContentType())
	require.Equal(t, []byte("unexpected response"), second.Body)

	third, err := c.ValidateBlueprintWithBodyWithResponse(context.Background(), "multipart/form-data", bytes.NewReader(nil))
	require.NoError(t, err)
	require.NotNil(t, third.JSON200)
	require.True(t, third.JSON200.Valid)
}
