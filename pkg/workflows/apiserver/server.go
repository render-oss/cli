package apiserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/apiserver/internal"
	"github.com/render-oss/cli/pkg/workflows/apiserver/internal/serversideevents"
	"github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/render-oss/cli/pkg/workflows/orchestrator"
	"github.com/render-oss/cli/pkg/workflows/store"
)

type ServerHandler struct {
	coordinator *orchestrator.Coordinator
	taskStore   *store.TaskStore
	logStore    *logs.LogStore
	upgrader    *websocket.Upgrader
}

func handleError(w http.ResponseWriter, err error, statusCode int) {
	w.WriteHeader(statusCode)
	errJSON, err := json.Marshal(client.Error{
		Message: pointers.From(err.Error()),
	})
	if err != nil {
		log.Println("error marshalling error", err)
		return
	}
	w.Write(errJSON)
}

func Start(handler *ServerHandler, port int) *http.Server {
	mux := chi.NewMux()

	mux.Route("/v1", func(r chi.Router) {
		r.Route("/tasks", func(r chi.Router) {
			r.Get("/", handler.ListTasks)
			r.Route("/{taskID}", func(r chi.Router) {
				r.Get("/", handler.GetTask)
			})
		})
		r.Route("/task-runs", func(r chi.Router) {
			r.Post("/", handler.RunTask)
			r.Route("/{taskRunID}", func(r chi.Router) {
				r.Get("/", handler.GetTaskRun)
			})
			r.Get("/", handler.ListTaskRuns)
			r.Route("/events", func(r chi.Router) {
				r.Get("/", handler.TaskEvents)
			})
		})
		r.Route("/logs", func(r chi.Router) {
			r.Route("/subscribe", func(r chi.Router) {
				r.Get("/", handler.SubscribeLogs)
			})
			r.Get("/", handler.GetLogs)
		})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: mux,
	}

	go func() {
		// And we serve HTTP until the world ends.
		err := server.ListenAndServe()
		if err != nil {
			log.Println("api server error listening on localhost", err)
		}
	}()

	return server
}

func NewHandler(coordinator *orchestrator.Coordinator, taskStore *store.TaskStore, logStore *logs.LogStore, upgrader *websocket.Upgrader) *ServerHandler {
	return &ServerHandler{coordinator: coordinator, taskStore: taskStore, logStore: logStore, upgrader: upgrader}
}

func (h *ServerHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, err := h.coordinator.PopulateTasks(r.Context())
	if err != nil {
		handleError(w, err, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(internal.ListTasks(h.taskStore))
}

func (h *ServerHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	taskID := chi.URLParam(r, "taskID")

	task := internal.GetTask(h.taskStore, taskID)

	if task == nil {
		handleError(w, fmt.Errorf("task not found"), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(task)
}

func (h *ServerHandler) TaskEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	queryParams := r.URL.Query()
	taskRunIDs := strings.Split(queryParams.Get("taskRunIds"), ",")

	ch, err := internal.GetTaskRunEvents(ctx, h.taskStore, taskRunIDs)
	if err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	serversideevents.ServerSideEvents(ch)(w, r)
}

func (h *ServerHandler) RunTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var input workflows.RunTask

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	inputJSON, err := json.Marshal(input.Input)
	if err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	run, err := h.coordinator.StartTask(r.Context(), input.Task, inputJSON, nil)
	if err != nil {
		if _, ok := err.(*orchestrator.TaskNotFoundError); ok {
			handleError(w, err, http.StatusNotFound)
			w.Write([]byte(err.Error()))
			return
		}

		handleError(w, err, http.StatusInternalServerError)
		return
	}

	taskRun := internal.MapTaskRun(h.taskStore, run)

	if taskRun == nil {
		handleError(w, fmt.Errorf("task run not found"), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(taskRun)
}

func (h *ServerHandler) ListTaskRuns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params client.ListTaskRunsParams

	params.TaskId = pointers.From([]string{r.URL.Query().Get("taskId")})
	if params.TaskId == nil {
		handleError(w, fmt.Errorf("taskId is required"), http.StatusBadRequest)
		w.Write([]byte("taskId is required"))
		return
	}

	taskID := (*params.TaskId)[0]
	json.NewEncoder(w).Encode(internal.ListTaskRuns(h.taskStore, taskID))
}

func (h *ServerHandler) GetTaskRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	taskRunID := chi.URLParam(r, "taskRunID")
	taskRun := internal.GetTaskRun(h.taskStore, taskRunID)

	if taskRun == nil {
		handleError(w, fmt.Errorf("task run not found"), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(taskRun)
}

func (h *ServerHandler) SubscribeLogs(w http.ResponseWriter, r *http.Request) {
	input, err := internal.ParseLogSearchQueryParams(r)
	if err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleError(w, err, http.StatusInternalServerError)
		return
	}

	readCh, writeCh := internal.WebsocketChannelWrapper(conn)

	logCh := h.logStore.LogChan(internal.MapLogSearchParams(input))
	defer close(writeCh)
	defer func() {
		_ = conn.Close()
		h.logStore.RemoveLogChan(logCh)
	}()

	internal.ForwardLogsToWebsocket(logCh, readCh, writeCh)
}

func (h *ServerHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	input, err := internal.ParseLogSearchQueryParams(r)
	if err != nil {
		handleError(w, err, http.StatusBadRequest)
		return
	}

	logs := internal.ListLogs(h.logStore, input)

	json.NewEncoder(w).Encode(logs)
}
