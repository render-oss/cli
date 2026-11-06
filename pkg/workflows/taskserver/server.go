package taskserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	TaskUpdateSignalName = "task-update"
)

type TokenStore struct {
	WorkflowID string
	RunID      string
}

type temporalSignalClient interface {
	SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error
}

type TaskServerFactory struct{}

func NewTaskServerFactory() *TaskServerFactory {
	return &TaskServerFactory{}
}

type GetSubtaskResultFunc func(taskRunID string) (PostGetSubtaskResultResponseObject, error)

type StartSubtaskFunc func(req RunSubtaskRequest) (PostRunSubtaskResponseObject, error)

// DurableBackend serves the step routes of a workflow run's attempt.
type DurableBackend interface {
	StepInlineStart(ctx context.Context, req StepStartRequest) (PostStepInlineStartResponseObject, error)
	StepInlineAttempt(ctx context.Context, req StepAttemptRequest) (PostStepInlineAttemptResponseObject, error)
	StepInlineComplete(ctx context.Context, req StepCompleteRequest) (PostStepInlineCompleteResponseObject, error)
	StepWait(ctx context.Context, req WaitRequest) (PostStepWaitResponseObject, error)
}

// NewHandler builds the socket handler for one process. durable is nil for
// plain task runs and registration; the step routes then answer 400.
func (f *TaskServerFactory) NewHandler(
	socket net.Listener,
	input GetInput200JSONResponse,
	getSubtaskResultFunc GetSubtaskResultFunc,
	startSubtaskFunc StartSubtaskFunc,
	durable DurableBackend,
) *ServerHandler {
	channels := ServerChannels{
		PostCallback: make(chan PostCallbackRequestObject),
		PostTasks:    make(chan PostRegisterTasksRequestObject),
	}

	return &ServerHandler{
		Socket:               socket,
		Input:                input,
		Channels:             channels,
		GetSubtaskResultFunc: getSubtaskResultFunc,
		StartSubtaskFunc:     startSubtaskFunc,
		Durable:              durable,
	}
}

func (h *ServerHandler) Start() *http.Server {
	// create a type that satisfies the `api.ServerInterface`, which contains an implementation of every operation from the generated code
	strictHandler := NewStrictHandler(h, nil)

	r := chi.NewMux()

	// get an `http.Handler` that we can use
	muxHandler := HandlerFromMux(strictHandler, r)

	server := &http.Server{
		Handler:     muxHandler,
		ReadTimeout: 5 * time.Second, // Prevent Slowloris DoS attacks
	}

	go func() {
		// And we serve HTTP until the world ends.
		err := server.Serve(h.Socket)
		if err != nil {
			log.Println("task server error", err)
		}
		h.Socket.Close()
	}()

	return server
}

type ServerChannels struct {
	PostCallback chan PostCallbackRequestObject
	PostTasks    chan PostRegisterTasksRequestObject
}

type ServerHandler struct {
	Socket               net.Listener
	GetSubtaskResultFunc GetSubtaskResultFunc
	StartSubtaskFunc     StartSubtaskFunc
	Input                GetInput200JSONResponse
	Channels             ServerChannels
	Durable              DurableBackend
}

func (h *ServerHandler) PostCallback(ctx context.Context, params PostCallbackRequestObject) (PostCallbackResponseObject, error) {
	h.Channels.PostCallback <- params

	return PostCallback200Response{}, nil
}

func (h *ServerHandler) GetInput(ctx context.Context, params GetInputRequestObject) (GetInputResponseObject, error) {
	return h.Input, nil
}

func (h *ServerHandler) PostRegisterTasks(ctx context.Context, params PostRegisterTasksRequestObject) (PostRegisterTasksResponseObject, error) {
	if err := validateTasks(params.Body.Tasks); err != nil {
		log.Println(err)
		return PostRegisterTasks400Response{}, nil
	}

	h.Channels.PostTasks <- params

	return PostRegisterTasks200Response{}, nil
}

func (h *ServerHandler) PostGetSubtaskResult(ctx context.Context, params PostGetSubtaskResultRequestObject) (PostGetSubtaskResultResponseObject, error) {
	return h.GetSubtaskResultFunc(params.Body.TaskRunId)
}

func (h *ServerHandler) PostRunSubtask(ctx context.Context, params PostRunSubtaskRequestObject) (PostRunSubtaskResponseObject, error) {
	return h.StartSubtaskFunc(*params.Body)
}

func (h *ServerHandler) PostStepInlineStart(ctx context.Context, params PostStepInlineStartRequestObject) (PostStepInlineStartResponseObject, error) {
	if h.Durable == nil {
		return PostStepInlineStart400Response{}, nil
	}
	return h.Durable.StepInlineStart(ctx, *params.Body)
}

func (h *ServerHandler) PostStepInlineAttempt(ctx context.Context, params PostStepInlineAttemptRequestObject) (PostStepInlineAttemptResponseObject, error) {
	if h.Durable == nil {
		return PostStepInlineAttempt400Response{}, nil
	}
	return h.Durable.StepInlineAttempt(ctx, *params.Body)
}

func (h *ServerHandler) PostStepInlineComplete(ctx context.Context, params PostStepInlineCompleteRequestObject) (PostStepInlineCompleteResponseObject, error) {
	if h.Durable == nil {
		return PostStepInlineComplete400Response{}, nil
	}
	return h.Durable.StepInlineComplete(ctx, *params.Body)
}

// PostStepSleep answers 400 until local dev supports ctx.sleep.
func (h *ServerHandler) PostStepSleep(ctx context.Context, params PostStepSleepRequestObject) (PostStepSleepResponseObject, error) {
	return PostStepSleep400Response{}, nil
}

func (h *ServerHandler) PostStepWait(ctx context.Context, params PostStepWaitRequestObject) (PostStepWaitResponseObject, error) {
	if h.Durable == nil {
		return PostStepWait400Response{}, nil
	}
	return h.Durable.StepWait(ctx, *params.Body)
}

func validateTasks(tasks []Task) error {
	for _, task := range tasks {
		if task.Workflow != nil && *task.Workflow && task.Options != nil && task.Options.Retry != nil {
			return fmt.Errorf("task %s: retry policy is not allowed on a workflow", task.Name)
		}
	}
	return nil
}
