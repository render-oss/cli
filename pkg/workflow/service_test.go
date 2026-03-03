package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/pointers"
)

// mockWorkflowRepo implements workflowRepository for testing.
type mockWorkflowRepo struct {
	listWorkflowsFn func(ctx context.Context, params *client.ListWorkflowsParams) ([]*wfclient.Workflow, error)
	getWorkflowFn   func(ctx context.Context, id string) (*wfclient.Workflow, error)
}

func (m *mockWorkflowRepo) ListWorkflows(ctx context.Context, params *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
	return m.listWorkflowsFn(ctx, params)
}

func (m *mockWorkflowRepo) GetWorkflow(ctx context.Context, id string) (*wfclient.Workflow, error) {
	return m.getWorkflowFn(ctx, id)
}

// mockProjectRepo implements projectRepository for testing.
type mockProjectRepo struct {
	listProjectsFn func(ctx context.Context) ([]*client.Project, error)
}

func (m *mockProjectRepo) ListProjects(ctx context.Context) ([]*client.Project, error) {
	return m.listProjectsFn(ctx)
}

// mockEnvironmentRepo implements environmentRepository for testing.
type mockEnvironmentRepo struct {
	listEnvironmentsFn func(ctx context.Context, params *client.ListEnvironmentsParams) ([]*client.Environment, error)
}

func (m *mockEnvironmentRepo) ListEnvironments(ctx context.Context, params *client.ListEnvironmentsParams) ([]*client.Environment, error) {
	return m.listEnvironmentsFn(ctx, params)
}

func TestListWorkflows(t *testing.T) {
	t.Run("hydrates workflows with matching project and environment", func(t *testing.T) {
		wf := &wfclient.Workflow{Id: "wf-1", Name: "my-wf", EnvironmentId: pointers.From("env-1")}
		proj := &client.Project{Id: "proj-1", Name: "my-proj", EnvironmentIds: []string{"env-1"}}
		env := &client.Environment{Id: "env-1", Name: "staging", ProjectId: "proj-1"}

		svc := &Service{
			repo: &mockWorkflowRepo{
				listWorkflowsFn: func(context.Context, *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
					return []*wfclient.Workflow{wf}, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return []*client.Project{proj}, nil
				},
			},
			environmentRepo: &mockEnvironmentRepo{
				listEnvironmentsFn: func(context.Context, *client.ListEnvironmentsParams) ([]*client.Environment, error) {
					return []*client.Environment{env}, nil
				},
			},
		}

		models, err := svc.ListWorkflows(context.Background(), &client.ListWorkflowsParams{})
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "wf-1", models[0].Workflow.Id)
		assert.Equal(t, proj, models[0].Project)
		assert.Equal(t, env, models[0].Environment)
	})

	t.Run("workflow with nil EnvironmentId gets nil project and environment", func(t *testing.T) {
		wf := &wfclient.Workflow{Id: "wf-1", Name: "my-wf", EnvironmentId: nil}
		proj := &client.Project{Id: "proj-1", Name: "my-proj", EnvironmentIds: []string{"env-1"}}

		svc := &Service{
			repo: &mockWorkflowRepo{
				listWorkflowsFn: func(context.Context, *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
					return []*wfclient.Workflow{wf}, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return []*client.Project{proj}, nil
				},
			},
			environmentRepo: &mockEnvironmentRepo{
				listEnvironmentsFn: func(context.Context, *client.ListEnvironmentsParams) ([]*client.Environment, error) {
					return nil, nil
				},
			},
		}

		models, err := svc.ListWorkflows(context.Background(), &client.ListWorkflowsParams{})
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Nil(t, models[0].Project)
		assert.Nil(t, models[0].Environment)
	})

	t.Run("results are sorted by project, environment, name", func(t *testing.T) {
		wfA := &wfclient.Workflow{Id: "wf-a", Name: "zebra", EnvironmentId: pointers.From("env-1")}
		wfB := &wfclient.Workflow{Id: "wf-b", Name: "alpha", EnvironmentId: pointers.From("env-1")}
		wfC := &wfclient.Workflow{Id: "wf-c", Name: "middle", EnvironmentId: nil}

		proj := &client.Project{Id: "proj-1", Name: "my-proj", EnvironmentIds: []string{"env-1"}}
		env := &client.Environment{Id: "env-1", Name: "staging", ProjectId: "proj-1"}

		svc := &Service{
			repo: &mockWorkflowRepo{
				listWorkflowsFn: func(context.Context, *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
					return []*wfclient.Workflow{wfC, wfA, wfB}, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return []*client.Project{proj}, nil
				},
			},
			environmentRepo: &mockEnvironmentRepo{
				listEnvironmentsFn: func(context.Context, *client.ListEnvironmentsParams) ([]*client.Environment, error) {
					return []*client.Environment{env}, nil
				},
			},
		}

		models, err := svc.ListWorkflows(context.Background(), &client.ListWorkflowsParams{})
		require.NoError(t, err)
		require.Len(t, models, 3)
		// Workflows with project/env sort before those without (empty last)
		assert.Equal(t, "alpha", models[0].Name())
		assert.Equal(t, "zebra", models[1].Name())
		assert.Equal(t, "middle", models[2].Name())
	})

	t.Run("error from workflow repo propagates", func(t *testing.T) {
		svc := &Service{
			repo: &mockWorkflowRepo{
				listWorkflowsFn: func(context.Context, *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
					return nil, errors.New("workflow repo error")
				},
			},
		}

		_, err := svc.ListWorkflows(context.Background(), &client.ListWorkflowsParams{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow repo error")
	})

	t.Run("error from project repo propagates", func(t *testing.T) {
		svc := &Service{
			repo: &mockWorkflowRepo{
				listWorkflowsFn: func(context.Context, *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
					return []*wfclient.Workflow{}, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return nil, errors.New("project repo error")
				},
			},
		}

		_, err := svc.ListWorkflows(context.Background(), &client.ListWorkflowsParams{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project repo error")
	})

	t.Run("error from environment repo propagates", func(t *testing.T) {
		proj := &client.Project{Id: "proj-1", Name: "my-proj", EnvironmentIds: []string{"env-1"}}

		svc := &Service{
			repo: &mockWorkflowRepo{
				listWorkflowsFn: func(context.Context, *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
					return []*wfclient.Workflow{}, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return []*client.Project{proj}, nil
				},
			},
			environmentRepo: &mockEnvironmentRepo{
				listEnvironmentsFn: func(context.Context, *client.ListEnvironmentsParams) ([]*client.Environment, error) {
					return nil, errors.New("env repo error")
				},
			},
		}

		_, err := svc.ListWorkflows(context.Background(), &client.ListWorkflowsParams{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "env repo error")
	})
}

func TestGetWorkflow(t *testing.T) {
	t.Run("hydrates single workflow with matching project and environment", func(t *testing.T) {
		wf := &wfclient.Workflow{Id: "wf-1", Name: "my-wf", EnvironmentId: pointers.From("env-1")}
		proj := &client.Project{Id: "proj-1", Name: "my-proj", EnvironmentIds: []string{"env-1"}}
		env := &client.Environment{Id: "env-1", Name: "staging", ProjectId: "proj-1"}

		svc := &Service{
			repo: &mockWorkflowRepo{
				getWorkflowFn: func(_ context.Context, id string) (*wfclient.Workflow, error) {
					assert.Equal(t, "wf-1", id)
					return wf, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return []*client.Project{proj}, nil
				},
			},
			environmentRepo: &mockEnvironmentRepo{
				listEnvironmentsFn: func(context.Context, *client.ListEnvironmentsParams) ([]*client.Environment, error) {
					return []*client.Environment{env}, nil
				},
			},
		}

		model, err := svc.GetWorkflow(context.Background(), "wf-1")
		require.NoError(t, err)
		assert.Equal(t, "wf-1", model.Workflow.Id)
		assert.Equal(t, proj, model.Project)
		assert.Equal(t, env, model.Environment)
	})

	t.Run("only fetches environments for the matched project", func(t *testing.T) {
		wf := &wfclient.Workflow{Id: "wf-1", Name: "my-wf", EnvironmentId: pointers.From("env-2")}
		proj1 := &client.Project{Id: "proj-1", Name: "proj-one", EnvironmentIds: []string{"env-1"}}
		proj2 := &client.Project{Id: "proj-2", Name: "proj-two", EnvironmentIds: []string{"env-2"}}

		var capturedParams *client.ListEnvironmentsParams
		svc := &Service{
			repo: &mockWorkflowRepo{
				getWorkflowFn: func(context.Context, string) (*wfclient.Workflow, error) {
					return wf, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return []*client.Project{proj1, proj2}, nil
				},
			},
			environmentRepo: &mockEnvironmentRepo{
				listEnvironmentsFn: func(_ context.Context, params *client.ListEnvironmentsParams) ([]*client.Environment, error) {
					capturedParams = params
					return []*client.Environment{{Id: "env-2", Name: "production", ProjectId: "proj-2"}}, nil
				},
			},
		}

		model, err := svc.GetWorkflow(context.Background(), "wf-1")
		require.NoError(t, err)
		assert.Equal(t, proj2, model.Project)
		require.NotNil(t, capturedParams)
		assert.Equal(t, []string{"proj-2"}, capturedParams.ProjectId)
	})

	t.Run("workflow with no matching project gets nil environment", func(t *testing.T) {
		wf := &wfclient.Workflow{Id: "wf-1", Name: "my-wf", EnvironmentId: pointers.From("env-orphan")}
		proj := &client.Project{Id: "proj-1", Name: "my-proj", EnvironmentIds: []string{"env-1"}}

		envListCalled := false
		svc := &Service{
			repo: &mockWorkflowRepo{
				getWorkflowFn: func(context.Context, string) (*wfclient.Workflow, error) {
					return wf, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return []*client.Project{proj}, nil
				},
			},
			environmentRepo: &mockEnvironmentRepo{
				listEnvironmentsFn: func(context.Context, *client.ListEnvironmentsParams) ([]*client.Environment, error) {
					envListCalled = true
					return nil, nil
				},
			},
		}

		model, err := svc.GetWorkflow(context.Background(), "wf-1")
		require.NoError(t, err)
		assert.Nil(t, model.Project)
		assert.Nil(t, model.Environment)
		assert.False(t, envListCalled, "should not fetch environments when no project matches")
	})

	t.Run("error from workflow repo propagates", func(t *testing.T) {
		svc := &Service{
			repo: &mockWorkflowRepo{
				getWorkflowFn: func(context.Context, string) (*wfclient.Workflow, error) {
					return nil, errors.New("get workflow error")
				},
			},
		}

		_, err := svc.GetWorkflow(context.Background(), "wf-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get workflow error")
	})

	t.Run("error from project repo propagates", func(t *testing.T) {
		wf := &wfclient.Workflow{Id: "wf-1", Name: "my-wf"}

		svc := &Service{
			repo: &mockWorkflowRepo{
				getWorkflowFn: func(context.Context, string) (*wfclient.Workflow, error) {
					return wf, nil
				},
			},
			projectRepo: &mockProjectRepo{
				listProjectsFn: func(context.Context) ([]*client.Project, error) {
					return nil, errors.New("project list error")
				},
			},
		}

		_, err := svc.GetWorkflow(context.Background(), "wf-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project list error")
	})
}
