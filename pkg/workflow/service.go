package workflow

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resource/util"
)

type Service struct {
	repo            *Repo
	environmentRepo *environment.Repo
	projectRepo     *project.Repo
}

func NewService(repo *Repo, environmentRepo *environment.Repo, projectRepo *project.Repo) *Service {
	return &Service{
		repo:            repo,
		environmentRepo: environmentRepo,
		projectRepo:     projectRepo,
	}
}

func (s *Service) ListWorkflows(ctx context.Context, params *client.ListWorkflowsParams) ([]*Model, error) {
	workflows, err := s.repo.ListWorkflows(ctx, params)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	envs, err := s.allEnvironments(ctx, projects)
	if err != nil {
		return nil, err
	}

	var workflowModels []*Model

	for _, workflow := range workflows {
		model, err := s.hydrateWorkflowModelWithEnvs(workflow, projects, envs)
		if err != nil {
			return nil, err
		}
		workflowModels = append(workflowModels, model)
	}

	util.SortResources(workflowModels)
	return workflowModels, nil
}

func (s *Service) GetWorkflow(ctx context.Context, id string) (*Model, error) {
	workflow, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	return s.hydrateWorkflowModel(ctx, workflow, projects)
}

func (s *Service) hydrateWorkflowModel(ctx context.Context, workflow *wfclient.Workflow, projects []*client.Project) (*Model, error) {
	model := &Model{Workflow: workflow}

	model.Project = s.projectForWorkflow(workflow, projects)

	if model.Project != nil {
		envs, err := s.allEnvironments(ctx, []*client.Project{model.Project})
		if err != nil {
			return nil, err
		}
		model.Environment = s.environmentForWorkflow(workflow, envs)
	}

	return model, nil
}

func (s *Service) hydrateWorkflowModelWithEnvs(workflow *wfclient.Workflow, projects []*client.Project, envs []*client.Environment) (*Model, error) {
	model := &Model{Workflow: workflow}

	model.Project = s.projectForWorkflow(workflow, projects)
	model.Environment = s.environmentForWorkflow(workflow, envs)

	return model, nil
}

func (s *Service) environmentForWorkflow(workflow *wfclient.Workflow, envs []*client.Environment) *client.Environment {
	if workflow.EnvironmentId == nil {
		return nil
	}

	for _, env := range envs {
		if *workflow.EnvironmentId == env.Id {
			return env
		}
	}

	return nil
}

func (s *Service) projectForWorkflow(workflow *wfclient.Workflow, projects []*client.Project) *client.Project {
	if workflow.EnvironmentId == nil {
		return nil
	}

	for _, proj := range projects {
		for _, envID := range proj.EnvironmentIds {
			if *workflow.EnvironmentId == envID {
				return proj
			}
		}
	}

	return nil
}

func (s *Service) allEnvironments(ctx context.Context, projects []*client.Project) ([]*client.Environment, error) {
	if len(projects) == 0 {
		return nil, nil
	}
	var projIDs []string
	for _, proj := range projects {
		projIDs = append(projIDs, proj.Id)
	}

	return s.environmentRepo.ListEnvironments(ctx, &client.ListEnvironmentsParams{ProjectId: projIDs})
}
