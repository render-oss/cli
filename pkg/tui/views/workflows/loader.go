package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/version"
	"github.com/render-oss/cli/pkg/workflow"
)

type WorkflowLoaderDeps interface {
	TaskRepo() *tasks.Repo
	WorkflowService() *workflow.Service
	WorkflowVersionRepo() *version.Repo
	WorkflowRepo() *workflow.Repo
}

type WorkflowLoader struct {
	taskRepo            *tasks.Repo
	workflowService     *workflow.Service
	workflowVersionRepo *version.Repo
	workflowRepo        *workflow.Repo
}

func NewWorkflowLoader(taskRepo *tasks.Repo, workflowService *workflow.Service, workflowVersionRepo *version.Repo, workflowRepo *workflow.Repo) *WorkflowLoader {
	return &WorkflowLoader{
		taskRepo:            taskRepo,
		workflowService:     workflowService,
		workflowVersionRepo: workflowVersionRepo,
		workflowRepo:        workflowRepo,
	}
}

func (t *WorkflowLoader) CreateTaskRun(ctx context.Context, input TaskRunInput) (*workflows.TaskRun, error) {
	inputData, err := unmarshalInputData(input.Input)
	if err != nil {
		return nil, err
	}
	return t.taskRepo.RunTask(ctx, input.TaskID, inputData)
}

func unmarshalInputData(input string) ([]interface{}, error) {
	var parsedInputs []interface{}

	if len(input) == 0 {
		return nil, fmt.Errorf("Task input is required.")
	}

	inputRaw := []byte(input)

	if err := json.Unmarshal(inputRaw, &parsedInputs); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshaltypeErr *json.UnmarshalTypeError
		if errors.As(err, &syntaxErr) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("The task input has invalid JSON.")
		} else if errors.As(err, &unmarshaltypeErr) {
			actualType := friendlyTypeName(unmarshaltypeErr.Value)
			return nil, fmt.Errorf("Expected type array for task input, received type %s.", actualType)
		} else {
			return nil, err
		}
	}

	return parsedInputs, nil
}

// friendlyTypeName converts type names from Go's json.UnmarshalTypeError to user-friendly names
func friendlyTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)

	switch typeName {
	case "[]interface {}":
		return "array"
	case "bool":
		return "boolean"
	default:
		return typeName
	}
}

func (w *WorkflowLoader) LoadVersionList(ctx context.Context, input VersionListInput, cur client.Cursor) (client.Cursor, []*wfclient.WorkflowVersion, error) {
	pageSize := 20
	params := &client.ListWorkflowVersionsParams{
		Limit:      &pageSize,
		WorkflowID: pointers.From([]string{input.WorkflowID}),
	}
	if cur != "" {
		params.Cursor = &cur
	}

	return w.workflowVersionRepo.ListVersions(ctx, input.WorkflowID, params)
}

func (w *WorkflowLoader) ReleaseVersion(ctx context.Context, input VersionReleaseInput) (*wfclient.WorkflowVersion, error) {
	if input.CommitID != nil && *input.CommitID == "" {
		input.CommitID = nil
	}

	err := w.workflowVersionRepo.TriggerRelease(ctx, input.WorkflowID, version.TriggerReleaseInput{
		CommitId: input.CommitID,
	})
	if err != nil {
		return nil, err
	}

	wfv, err := w.WaitForVersionRelease(ctx, input.WorkflowID)
	if err != nil {
		return nil, err
	}

	return wfv, nil
}

func (w *WorkflowLoader) WaitForVersion(ctx context.Context, workflowID, workflowVersionID string) (*wfclient.WorkflowVersion, error) {
	timeoutTimer := time.NewTimer(versionTimeout)

	for {
		select {
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("timed out waiting for release to finish")
		default:
			v, err := w.workflowVersionRepo.GetVersion(ctx, workflowVersionID)
			if err != nil {
				return nil, err
			}

			return v, nil

			// TODO CAP-7490
			// https://linear.app/render-com/issue/CAP-7490/flesh-out-workflow-version-information-at-least-restgql-if-not-present
			// if workflowversion.IsComplete(v.Status) {
			// 	return v, nil
			// }

			// if v.Status == nil || *v.Status == client.VersionStatusCreated {
			// 	time.Sleep(10 * time.Second)
			// } else {
			// 	// if the release has started, poll more frequently
			// 	time.Sleep(5 * time.Second)
			// }
		}
	}
}

func (w *WorkflowLoader) WaitForVersionRelease(ctx context.Context, workflowID string) (*wfclient.WorkflowVersion, error) {
	timeoutTimer := time.NewTimer(versionReleaseTimeout)

	for {
		select {
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("timed out waiting for version to be created")
		default:
			// TODO CAP-7490
			// https://linear.app/render-com/issue/CAP-7490/flesh-out-workflow-version-information-at-least-restgql-if-not-present
			// hacky "get latest version" straight up does not work without statuses/visibility
			_, wfv, err := w.workflowVersionRepo.ListVersions(ctx, workflowID, &client.ListWorkflowVersionsParams{Limit: pointers.From(1)})
			if err != nil {
				return nil, err
			}

			if len(wfv) > 0 {
				return wfv[0], nil
			}

			time.Sleep(time.Second)
		}
	}
}

func (w *WorkflowLoader) VersionReleaseConfirm(ctx context.Context, input VersionReleaseInput) func() (string, error) {
	return func() (string, error) {
		workflowRepo := w.workflowRepo
		wf, err := workflowRepo.GetWorkflow(ctx, input.WorkflowID)
		if err != nil {
			return "", fmt.Errorf("failed to get workflow: %w", err)
		}

		return fmt.Sprintf("Are you sure you want to release %s?", wf.Name), nil
	}
}

func (w *WorkflowLoader) ListWorkflows(ctx context.Context, in WorkflowInput) ([]*workflow.Model, error) {
	workflowService := w.workflowService

	listInput := &client.ListWorkflowsParams{
		Limit: pointers.From(100),
	}

	if len(in.EnvironmentIDs) > 0 {
		listInput.EnvironmentId = &in.EnvironmentIDs
	}

	return workflowService.ListWorkflows(ctx, listInput)
}

func (w *WorkflowLoader) LoadTaskList(ctx context.Context, input TaskListInput, cur client.Cursor) (client.Cursor, []*wfclient.Task, error) {
	params := &client.ListTasksParams{
		WorkflowVersionId: pointers.From([]string{input.WorkflowVersionID}),
	}

	return w.taskRepo.ListTasks(ctx, params)
}

func (w *WorkflowLoader) GetTask(ctx context.Context, id string) (*wfclient.Task, error) {
	return w.taskRepo.GetTask(ctx, id)
}

func (w *WorkflowLoader) LoadTaskRunList(ctx context.Context, input TaskRunListInput, cur client.Cursor) (client.Cursor, []*wfclient.TaskRun, error) {
	pageSize := 20
	params := &client.ListTaskRunsParams{Limit: &pageSize, TaskId: pointers.From([]string{input.TaskID})}
	if cur != "" {
		params.Cursor = &cur
	}

	return w.taskRepo.ListTaskRuns(ctx, params)
}

func (w *WorkflowLoader) LoadTaskRunDetails(ctx context.Context, input *TaskRunDetailsInput) (*workflows.TaskRunDetails, error) {
	return w.taskRepo.GetTaskRunDetails(ctx, input.TaskRunID)
}
