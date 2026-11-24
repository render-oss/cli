package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
)

const restartTestPostgresID = "dpg-00000000000000000000"

// newRestartTestRepo serves POST /postgres/{id}/restart with the given status
// and returns a Repo pointed at it plus a counter of restart requests.
func newRestartTestRepo(t *testing.T, status int, body *client.Error) (*Repo, *int) {
	t.Helper()

	restartCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/postgres/"+restartTestPostgresID+"/restart", r.URL.Path)
		restartCalls++
		if body == nil {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(server.Close)

	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	return NewRepo(c), &restartCalls
}

func TestRestartPostgresDatabase_AcceptsSuccessStatuses(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusAccepted} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			repo, calls := newRestartTestRepo(t, status, nil)

			err := repo.RestartPostgresDatabase(context.Background(), restartTestPostgresID)

			require.NoError(t, err)
			assert.Equal(t, 1, *calls)
		})
	}
}

func TestRestartPostgresDatabase_ReturnsAPIErrors(t *testing.T) {
	for _, tc := range []struct {
		status    int
		code      string
		message   string
		wantErr   string
		wantErrIs error
	}{
		{status: http.StatusBadRequest, code: "BAD_REQUEST", message: "bad request", wantErr: "received response code 400 (BAD_REQUEST): bad request"},
		{status: http.StatusNotFound, code: "NOT_FOUND", message: "not found", wantErr: "received response code 404 (NOT_FOUND): not found"},
		{status: http.StatusConflict, code: "CONFLICT", message: "restart already in progress", wantErr: "received response code 409 (CONFLICT): restart already in progress"},
		{status: http.StatusTooManyRequests, code: "TOO_MANY_REQUESTS", message: "slow down", wantErrIs: client.ErrTooManyRequests},
		{status: http.StatusInternalServerError, code: "INTERNAL", message: "internal error", wantErr: "received response code 500 (INTERNAL): internal error"},
		{status: http.StatusServiceUnavailable, code: "UNAVAILABLE", message: "unavailable", wantErr: "received response code 503 (UNAVAILABLE): unavailable"},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			repo, calls := newRestartTestRepo(t, tc.status, &client.Error{
				Code:    pointers.From(tc.code),
				Message: pointers.From(tc.message),
			})

			err := repo.RestartPostgresDatabase(context.Background(), restartTestPostgresID)

			require.Error(t, err)
			if tc.wantErrIs != nil {
				require.ErrorIs(t, err, tc.wantErrIs)
			} else {
				require.EqualError(t, err, tc.wantErr)
			}
			assert.Equal(t, 1, *calls)
		})
	}
}

func TestRestartPostgresDatabase_ReturnsTransportErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	c, err := client.NewClientWithResponses(url)
	require.NoError(t, err)

	err = NewRepo(c).RestartPostgresDatabase(context.Background(), restartTestPostgresID)

	require.Error(t, err)
}
