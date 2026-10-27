package flowrun

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

func (r *Repo) ListFlows(ctx context.Context, params *client.ListFlowsParams) (client.Cursor, []*workflows.Flow, error) {
	resp, err := r.client.ListFlowsWithResponse(ctx, params)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list flows: %w", err)
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return "", nil, err
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return "", nil, nil
	}

	result := make([]*workflows.Flow, 0, len(*resp.JSON200))
	for _, f := range *resp.JSON200 {
		flow := f.Flow
		result = append(result, &flow)
	}
	return nextCursor(params.Limit, (*resp.JSON200)[len(*resp.JSON200)-1].Cursor, len(result)), result, nil
}

func (r *Repo) GetFlow(ctx context.Context, flowID string) (*workflows.Flow, error) {
	resp, err := r.client.GetFlowWithResponse(ctx, flowID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}
	return resp.JSON200, nil
}

func (r *Repo) ListFlowRuns(ctx context.Context, params *client.ListFlowRunsParams) (client.Cursor, []*workflows.FlowRun, error) {
	resp, err := r.client.ListFlowRunsWithResponse(ctx, params)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list flow runs: %w", err)
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return "", nil, err
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return "", nil, nil
	}

	result := make([]*workflows.FlowRun, 0, len(*resp.JSON200))
	for _, fr := range *resp.JSON200 {
		run := fr.FlowRun
		result = append(result, &run)
	}
	return nextCursor(params.Limit, (*resp.JSON200)[len(*resp.JSON200)-1].Cursor, len(result)), result, nil
}

func (r *Repo) StartFlowRun(ctx context.Context, body workflows.StartFlowRun) (*workflows.FlowRun, error) {
	resp, err := r.client.StartFlowRunWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON202 == nil {
		return nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}
	return resp.JSON202, nil
}

func (r *Repo) GetFlowRun(ctx context.Context, flowRunID string) (*workflows.FlowRunDetails, error) {
	resp, err := r.client.GetFlowRunWithResponse(ctx, flowRunID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	return resp.JSON200, nil
}

func (r *Repo) CancelFlowRun(ctx context.Context, flowRunID string) error {
	resp, err := r.client.CancelFlowRunWithResponse(ctx, flowRunID)
	if err != nil {
		return err
	}
	return client.ErrorFromResponse(resp)
}

func (r *Repo) ListFlowRunSteps(ctx context.Context, flowRunID string, params *client.ListFlowRunStepsParams) (client.Cursor, []*workflows.FlowRunStep, error) {
	resp, err := r.client.ListFlowRunStepsWithResponse(ctx, flowRunID, params)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list flow run steps: %w", err)
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return "", nil, err
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return "", nil, nil
	}

	result := make([]*workflows.FlowRunStep, 0, len(*resp.JSON200))
	for _, s := range *resp.JSON200 {
		step := s.Step
		result = append(result, &step)
	}
	return nextCursor(params.Limit, (*resp.JSON200)[len(*resp.JSON200)-1].Cursor, len(result)), result, nil
}

// nextCursor returns the server's cursor only when the page is full, matching
// the --limit the caller asked for. An empty cursor signals the final page so
// callers do not treat a short page as "more coming".
func nextCursor(limit *int, cursor client.Cursor, pageLen int) client.Cursor {
	if limit != nil && pageLen < *limit {
		return ""
	}
	return cursor
}

func (r *Repo) GetFlowRunStep(ctx context.Context, flowRunID string, stepSeq int) (*workflows.FlowRunStepDetails, error) {
	resp, err := r.client.GetFlowRunStepWithResponse(ctx, flowRunID, stepSeq)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}
	return resp.JSON200, nil
}
