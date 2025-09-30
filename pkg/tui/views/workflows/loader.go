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
	var inputData []interface{}
	if err := json.Unmarshal([]byte(input.Input), &inputData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}
	return t.taskRepo.RunTask(ctx, input.TaskID, inputData)
}

func (w *WorkflowLoader) LoadVersionList(ctx context.Context, input VersionListInput, cur client.Cursor) (client.Cursor, []*wfclient.WorkflowVersion, error) {
	// TODO CAP-7491
	// https://linear.app/render-com/issue/CAP-7491/workflow-version-queries-do-not-page
	// for now we don't actually page on workflow versions listing
	// we should and will so i'm leaving this to reduce that workload

	// pageSize := 20
	params := &client.ListWorkflowVersionsParams{
		// Limit: &pageSize
	}
	// if cur != "" {
	// 	params.Cursor = &cur
	// }

	versions, err := w.workflowVersionRepo.ListVersions(ctx, input.WorkflowID, params)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list workflow versions: %w", err)
	}

	// if len(versions) < pageSize {
	// 	return "", versions, nil
	// }

	return "", versions, nil
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
			wfv, err := w.workflowVersionRepo.ListVersions(ctx, workflowID, &client.ListWorkflowVersionsParams{Limit: pointers.From(1)})
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
