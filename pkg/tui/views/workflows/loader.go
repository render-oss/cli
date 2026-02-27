package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/version"
	"github.com/render-oss/cli/pkg/workflow"
	"github.com/render-oss/cli/pkg/workflowversion"
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

func unmarshalInputData(input string) (*workflows.TaskData, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("Task input is required.")
	}

	inputRaw := []byte(input)

	// Validate that the input is valid JSON
	if !json.Valid(inputRaw) {
		return nil, fmt.Errorf("The task input has invalid JSON.")
	}

	var taskData workflows.TaskData

	// Try to parse as array (TaskData0 contains positional arguments)
	var arrayInput workflows.TaskData0
	if err := json.Unmarshal(inputRaw, &arrayInput); err == nil {
		if err := taskData.FromTaskData0(arrayInput); err != nil {
			return nil, fmt.Errorf("failed to convert input to TaskData: %w", err)
		}
		return &taskData, nil
	}

	// Try to parse as object (TaskData1 contains named parameters)
	var objectInput workflows.TaskData1
	if err := json.Unmarshal(inputRaw, &objectInput); err == nil {
		if err := taskData.FromTaskData1(objectInput); err != nil {
			return nil, fmt.Errorf("failed to convert input to TaskData: %w", err)
		}
		return &taskData, nil
	}

	return nil, fmt.Errorf("Task input must be a JSON array or object.")
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
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Check immediately before waiting for the first tick
	v, err := w.workflowVersionRepo.GetVersion(ctx, workflowVersionID)
	if err != nil {
		return nil, err
	}
	if workflowversion.IsComplete(v.Status) {
		return v, nil
	}

	for {
		select {
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("timed out waiting for release to finish")
		case <-ticker.C:
			v, err := w.workflowVersionRepo.GetVersion(ctx, workflowVersionID)
			if err != nil {
				return nil, err
			}

			if workflowversion.IsComplete(v.Status) {
				return v, nil
			}
		}
	}
}

func (w *WorkflowLoader) WaitForVersionRelease(ctx context.Context, workflowID string) (*wfclient.WorkflowVersion, error) {
	timeoutTimer := time.NewTimer(versionReleaseTimeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// Check immediately before waiting for the first tick
	_, wfv, err := w.workflowVersionRepo.ListVersions(ctx, workflowID, &client.ListWorkflowVersionsParams{Limit: pointers.From(1)})
	if err != nil {
		return nil, err
	}
	if len(wfv) > 0 {
		return wfv[0], nil
	}

	for {
		select {
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("timed out waiting for version to be created")
		case <-ticker.C:
			_, wfv, err := w.workflowVersionRepo.ListVersions(ctx, workflowID, &client.ListWorkflowVersionsParams{Limit: pointers.From(1)})
			if err != nil {
				return nil, err
			}

			if len(wfv) > 0 {
				return wfv[0], nil
			}
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
	params := &client.ListTasksParams{}

	if input.WorkflowVersionID != "" {
		params.WorkflowVersionId = pointers.From([]string{input.WorkflowVersionID})
	} else if input.WorkflowID != "" {
		if input.LatestVersionOnly {
			versionID, err := w.latestVersionID(ctx, input.WorkflowID)
			if err != nil {
				return "", nil, err
			}
			params.WorkflowVersionId = pointers.From([]string{versionID})
		} else {
			params.WorkflowId = pointers.From([]string{input.WorkflowID})
		}
	}

	return w.taskRepo.ListTasks(ctx, params)
}

func (w *WorkflowLoader) latestVersionID(ctx context.Context, workflowID string) (string, error) {
	limit := 1
	params := &client.ListWorkflowVersionsParams{
		Limit:      &limit,
		WorkflowID: pointers.From([]string{workflowID}),
	}
	_, versions, err := w.workflowVersionRepo.ListVersions(ctx, workflowID, params)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for workflow %s", workflowID)
	}
	return versions[0].Id, nil
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
