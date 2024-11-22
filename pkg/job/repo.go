package job

import (
	"context"
	"fmt"
	"time"

	"github.com/renderinc/cli/pkg/client"
	clientjob "github.com/renderinc/cli/pkg/client/jobs"
	"github.com/renderinc/cli/pkg/pointers"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

type ListJobsInput struct {
	ServiceID      string
	Status         []string
	CreatedBefore  *time.Time
	CreatedAfter   *time.Time
	StartedBefore  *time.Time
	StartedAfter   *time.Time
	FinishedBefore *time.Time
	FinishedAfter  *time.Time
}

func (r *Repo) ListJobs(ctx context.Context, input ListJobsInput, cur client.Cursor) (client.Cursor, []*clientjob.Job, error) {
	var statusFilters []client.ListJobParamsStatus
	for _, status := range input.Status {
		switch status {
		case "failed":
			statusFilters = append(statusFilters, client.Failed)
		case "pending":
			statusFilters = append(statusFilters, client.Pending)
		case "running":
			statusFilters = append(statusFilters, client.Running)
		case "succeeded":
			statusFilters = append(statusFilters, client.Succeeded)
		default:
			return "", nil, fmt.Errorf("invalid status: %s", status)
		}
	}

	pageSize := 20
	params := &client.ListJobParams{
		CreatedBefore:  input.CreatedBefore,
		CreatedAfter:   input.CreatedAfter,
		StartedBefore:  input.StartedBefore,
		StartedAfter:   input.StartedAfter,
		FinishedBefore: input.FinishedBefore,
		FinishedAfter:  input.FinishedAfter,
		Limit:          pointers.From(pageSize),
	}

	if cur != "" {
		params.Cursor = &cur
	}

	if len(statusFilters) > 0 {
		params.Status = &statusFilters
	}

	resp, err := r.client.ListJobWithResponse(ctx, input.ServiceID, params)
	if err != nil {
		return "", nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return "", nil, err
	}

	respOK := *resp.JSON200
	jobs := make([]*clientjob.Job, len(respOK))
	for i, job := range respOK {
		jobs[i] = &job.Job
	}

	if len(jobs) < pageSize {
		return "", jobs, nil
	}

	return respOK[len(respOK)-1].Cursor, jobs, nil
}

type CreateJobInput struct {
	ServiceID    string
	StartCommand string
	PlanID       string
}

func (r *Repo) CreateJob(ctx context.Context, input CreateJobInput) (*clientjob.Job, error) {
	body := client.PostJobJSONRequestBody{
		StartCommand: input.StartCommand,
		PlanId:       &input.PlanID,
	}

	resp, err := r.client.PostJobWithResponse(ctx, input.ServiceID, body)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON201, nil
}

func (r *Repo) CancelJob(ctx context.Context, serviceID, jobID string) (*clientjob.Job, error) {
	resp, err := r.client.CancelJobWithResponse(ctx, serviceID, jobID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) GetJob(ctx context.Context, serviceID, jobID string) (*clientjob.Job, error) {
	resp, err := r.client.RetrieveJobWithResponse(ctx, serviceID, jobID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}
