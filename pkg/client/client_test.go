package client_test

import (
	"net/http"
	"testing"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestErrorFromResponse(t *testing.T) {
	t.Run("status code >= 400", func(t *testing.T) {
		t.Run("when body is an error type", func(t *testing.T) {
			err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
				Body:         []byte(`{"message":"failure"}`),
				HTTPResponse: &http.Response{StatusCode: 400},
			})

			require.ErrorContains(t, err, "received response code 400: failure")
		})

		t.Run("when body is not an error type", func(t *testing.T) {
			err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
				Body:         []byte(`unauthorized`),
				HTTPResponse: &http.Response{StatusCode: 400},
			})

			require.ErrorContains(t, err, "received response code 400: unauthorized")
		})
	})

	t.Run("status code < 400", func(t *testing.T) {
		err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
			HTTPResponse: &http.Response{StatusCode: 200},
		})

		require.NoError(t, err)
	})
}
