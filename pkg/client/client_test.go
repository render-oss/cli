package client_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
)

func TestErrorFromResponse(t *testing.T) {
	t.Run("status code 401", func(t *testing.T) {
		err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
			Body:         []byte("unauthorized"),
			HTTPResponse: &http.Response{StatusCode: 401},
		})

		require.ErrorIs(t, err, client.ErrUnauthorized)
	})
	t.Run("status code 403", func(t *testing.T) {
		err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
			Body:         []byte("forbidden"),
			HTTPResponse: &http.Response{StatusCode: 403},
		})

		require.ErrorIs(t, err, client.ErrForbidden)
	})
	t.Run("status code 429", func(t *testing.T) {
		// the API usually declares that errors have a `message` and `detail` field.
		// however, the rate-limit error response does not conform to this standard.
		// Mimic the API's actual rate-limit body, which uses the nonstandard
		// {"error": ...} envelope instead of the public REST {"message": ...} one.
		err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
			Body:         []byte(`{"error": "rate limit exceeded"}`),
			HTTPResponse: &http.Response{StatusCode: 429},
		})

		require.ErrorIs(t, err, client.ErrTooManyRequests)
	})

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
				Body:         []byte(`unknown error`),
				HTTPResponse: &http.Response{StatusCode: 400},
			})

			require.ErrorContains(t, err, "received response code 400: unknown error")
		})

		t.Run("when body carries an error code", func(t *testing.T) {
			err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
				Body:         []byte(`{"message":"snapshot is still being created","code":"snapshot_creating"}`),
				HTTPResponse: &http.Response{StatusCode: 409},
			})

			require.ErrorContains(t, err, "received response code 409 (snapshot_creating): snapshot is still being created")
		})

		t.Run("when body carries only an error code", func(t *testing.T) {
			err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
				Body:         []byte(`{"code":"snapshot_creating"}`),
				HTTPResponse: &http.Response{StatusCode: 409},
			})

			require.ErrorContains(t, err, "received response code 409 (snapshot_creating)")
		})

		t.Run("when body is empty", func(t *testing.T) {
			err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
				Body:         nil,
				HTTPResponse: &http.Response{StatusCode: 500},
			})

			require.EqualError(t, err, "received response code 500")
		})
	})

	t.Run("status code < 400", func(t *testing.T) {
		err := client.ErrorFromResponse(&client.ListSnapshotsResponse{
			HTTPResponse: &http.Response{StatusCode: 200},
		})

		require.NoError(t, err)
	})
}
