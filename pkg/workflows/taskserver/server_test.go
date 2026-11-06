package taskserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
	"github.com/stretchr/testify/require"
)

func TestPostRegisterTasks(t *testing.T) {
	tests := []struct {
		name       string
		task       taskserver.Task
		wantStatus int
	}{
		{
			name:       "plain task with retry",
			task:       taskserver.Task{Name: "charge", Options: &taskserver.TaskOptions{Retry: &taskserver.RetryConfig{MaxRetries: pointers.From(3)}}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "workflow",
			task:       taskserver.Task{Name: "fulfill", Workflow: pointers.From(true)},
			wantStatus: http.StatusOK,
		},
		{
			name: "workflow with retry",
			task: taskserver.Task{
				Name:     "fulfill",
				Workflow: pointers.From(true),
				Options:  &taskserver.TaskOptions{Retry: &taskserver.RetryConfig{MaxRetries: pointers.From(3)}},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &taskserver.ServerHandler{Channels: taskserver.ServerChannels{
				PostTasks: make(chan taskserver.PostRegisterTasksRequestObject, 1),
			}}
			body, err := json.Marshal(taskserver.Tasks{Tasks: []taskserver.Task{tt.task}})
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/register-tasks", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			taskserver.Handler(taskserver.NewStrictHandler(h, nil)).ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, tt.wantStatus == http.StatusOK, len(h.Channels.PostTasks) == 1, "only valid tasks are registered")
		})
	}
}

func TestStepRoutesRequireAWorkflow(t *testing.T) {
	h := &taskserver.ServerHandler{}
	for _, path := range []string{"/step-inline/start", "/step-inline/attempt", "/step-inline/complete", "/step-sleep", "/step-wait"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{}`)))
			req.Header.Set("Content-Type", "application/json")
			taskserver.Handler(taskserver.NewStrictHandler(h, nil)).ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
