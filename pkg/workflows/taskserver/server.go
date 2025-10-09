package taskserver

import (
	"context"
	"log"
	"net"
	"net/http"

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

type StartSubtaskFunc func(taskName string, input []byte) (PostRunSubtaskResponseObject, error)

func (f *TaskServerFactory) NewHandler(
	socket net.Listener,
	input GetInput200JSONResponse,
	getSubtaskResultFunc GetSubtaskResultFunc,
	startSubtaskFunc StartSubtaskFunc,
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
	}
}

func (h *ServerHandler) Start() *http.Server {
	// create a type that satisfies the `api.ServerInterface`, which contains an implementation of every operation from the generated code
	strictHandler := NewStrictHandler(h, nil)

	r := chi.NewMux()

	// get an `http.Handler` that we can use
	muxHandler := HandlerFromMux(strictHandler, r)

	server := &http.Server{
		Handler: muxHandler,
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
}

func (h *ServerHandler) PostCallback(ctx context.Context, params PostCallbackRequestObject) (PostCallbackResponseObject, error) {
	h.Channels.PostCallback <- params

	return PostCallback200Response{}, nil
}

func (h *ServerHandler) GetInput(ctx context.Context, params GetInputRequestObject) (GetInputResponseObject, error) {
	return h.Input, nil
}

func (h *ServerHandler) PostRegisterTasks(ctx context.Context, params PostRegisterTasksRequestObject) (PostRegisterTasksResponseObject, error) {
	h.Channels.PostTasks <- params

	return PostRegisterTasks200Response{}, nil
}

func (h *ServerHandler) PostGetSubtaskResult(ctx context.Context, params PostGetSubtaskResultRequestObject) (PostGetSubtaskResultResponseObject, error) {
	return h.GetSubtaskResultFunc(params.Body.TaskRunId)
}

func (h *ServerHandler) PostRunSubtask(ctx context.Context, params PostRunSubtaskRequestObject) (PostRunSubtaskResponseObject, error) {
	return h.StartSubtaskFunc(params.Body.TaskName, *params.Body.Input)
}
