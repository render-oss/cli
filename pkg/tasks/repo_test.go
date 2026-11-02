package tasks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
)

func TestRunTaskNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	repo := NewRepo(c)

	_, err = repo.RunTask(context.Background(), "My Workflow/my-task", &workflows.TaskData{})
	require.EqualError(t, err, `task "My Workflow/my-task" not found (received response code 404: not found)`)
	var notFound *NotFoundError
	require.ErrorAs(t, err, &notFound)
	require.Equal(t, "My Workflow/my-task", notFound.Task)
	require.EqualError(t, errors.Unwrap(err), "received response code 404: not found")

	// Local task names and task IDs keep the server's error.
	for _, task := range []string{"my-task", "tsk-1234"} {
		_, err = repo.RunTask(context.Background(), task, &workflows.TaskData{})
		require.EqualError(t, err, "received response code 404: not found")
		require.False(t, errors.As(err, &notFound))
	}
}
