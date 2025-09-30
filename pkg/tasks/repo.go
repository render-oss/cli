package tasks

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
)

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

type Repo struct {
	client *client.ClientWithResponses
}

func (r *Repo) RunTask(ctx context.Context, taskID string, input []interface{}) (*workflows.TaskRun, error) {
	resp, err := r.client.CreateTaskWithResponse(ctx, client.CreateTaskJSONRequestBody{
		Task:  taskID,
		Input: input,
	})

	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON202, nil
}

func (r *Repo) GetTask(ctx context.Context, taskID string) (*workflows.Task, error) {
	resp, err := r.client.GetTaskWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) ListTasks(ctx context.Context, params *client.ListTasksParams) (client.Cursor, []*workflows.Task, error) {
	resp, err := r.client.ListTasksWithResponse(ctx, params)
	if err != nil {
		return "", nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return "", nil, err
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return "", nil, nil
	}

	result := make([]*workflows.Task, 0, len(*resp.JSON200))
	for _, task := range *resp.JSON200 {
		result = append(result, &task)
	}

	// Currently cursor is not implemented for tasks
	return "", result, nil
}

func (r *Repo) ListTaskRuns(ctx context.Context, params *client.ListTaskRunsParams) (client.Cursor, []*workflows.TaskRun, error) {
	resp, err := r.client.ListTaskRunsWithResponse(ctx, params)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list task runs: %w", err)
	}

	if resp.JSON200 == nil {
		return "", nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	respOK := *resp.JSON200
	taskRuns := make([]*workflows.TaskRun, len(respOK))
	for i, tr := range respOK {
		taskRuns[i] = &tr
	}

	if len(taskRuns) < *params.Limit {
		return "", taskRuns, nil
	}

	return respOK[len(respOK)-1].Id, taskRuns, nil
}

func (r *Repo) GetTaskRunDetails(ctx context.Context, taskRunID string) (*workflows.TaskRunDetails, error) {
	resp, err := r.client.GetTaskRunWithResponse(ctx, taskRunID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	return resp.JSON200, nil
}
