package dependencies

import (
	"sync"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/deploy"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tasks"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/render-oss/cli/pkg/user"
	"github.com/render-oss/cli/pkg/version"
	"github.com/render-oss/cli/pkg/workflow"
)

type cache[T any] struct {
	value T
	once  sync.Once
}

func (c *cache[T]) Get(fn func() T) T {
	c.once.Do(func() {
		c.value = fn()
	})
	return c.value
}

type cachedDependencies struct {
	workflowLoader      cache[*workflowviews.WorkflowLoader]
	workflowService     cache[*workflow.Service]
	workflowRepo        cache[*workflow.Repo]
	workflowVersionRepo cache[*version.Repo]
	taskRepo            cache[*tasks.Repo]
	projectRepo         cache[*project.Repo]
	environmentRepo     cache[*environment.Repo]
	serviceRepo         cache[*service.Repo]
	postgresRepo        cache[*postgres.Repo]
	keyValueRepo        cache[*keyvalue.Repo]
	userRepo            cache[*user.Repo]
	ownerRepo           cache[*owner.Repo]
	deployRepo          cache[*deploy.Repo]
	serviceService      cache[*service.Service]
	postgresService     cache[*postgres.Service]
	keyValueService     cache[*keyvalue.Service]
	resourceService     cache[*resource.Service]
	logRepo             cache[*logs.LogRepo]
	logLoader           cache[*views.LogLoader]
	resourceLoader      cache[*views.ResourceLoader]
	apiConfig           cache[*config.APIConfig]
}

type Dependencies struct {
	*Commands
	stack  *tui.StackModel
	client *client.ClientWithResponses
	cache  *cachedDependencies
}

func New(c *client.ClientWithResponses) *Dependencies {
	return &Dependencies{
		client: c,
		Commands: &Commands{
			Workflow:  &WorkflowCommands{},
			Logs:      &LogsCommands{},
			Workspace: &WorkspaceCommands{},
		},
		cache: &cachedDependencies{},
	}
}

func (d *Dependencies) Stack() *tui.StackModel {
	return d.stack
}

func (d *Dependencies) SetStack(stack *tui.StackModel) {
	d.stack = stack
}

func (d *Dependencies) TaskRepo() *tasks.Repo {
	return d.cache.taskRepo.Get(func() *tasks.Repo {
		return tasks.NewRepo(d.client)
	})
}

func (d *Dependencies) WorkflowVersionRepo() *version.Repo {
	return d.cache.workflowVersionRepo.Get(func() *version.Repo {
		return version.NewRepo(d.client)
	})
}

func (d *Dependencies) WorkflowRepo() *workflow.Repo {
	return d.cache.workflowRepo.Get(func() *workflow.Repo {
		return workflow.NewRepo(d.client)
	})
}

func (d *Dependencies) ProjectRepo() *project.Repo {
	return d.cache.projectRepo.Get(func() *project.Repo {
		return project.NewRepo(d.client)
	})
}

func (d *Dependencies) EnvironmentRepo() *environment.Repo {
	return d.cache.environmentRepo.Get(func() *environment.Repo {
		return environment.NewRepo(d.client)
	})
}

func (d *Dependencies) ServiceRepo() *service.Repo {
	return d.cache.serviceRepo.Get(func() *service.Repo {
		return service.NewRepo(d.client)
	})
}

func (d *Dependencies) PostgresRepo() *postgres.Repo {
	return d.cache.postgresRepo.Get(func() *postgres.Repo {
		return postgres.NewRepo(d.client)
	})
}

func (d *Dependencies) KeyValueRepo() *keyvalue.Repo {
	return d.cache.keyValueRepo.Get(func() *keyvalue.Repo {
		return keyvalue.NewRepo(d.client)
	})
}

func (d *Dependencies) UserRepo() *user.Repo {
	return d.cache.userRepo.Get(func() *user.Repo {
		return user.NewRepo(d.client)
	})
}

func (d *Dependencies) OwnerRepo() *owner.Repo {
	return d.cache.ownerRepo.Get(func() *owner.Repo {
		return owner.NewRepo(d.client)
	})
}

func (d *Dependencies) DeployRepo() *deploy.Repo {
	return d.cache.deployRepo.Get(func() *deploy.Repo {
		return deploy.NewRepo(d.client)
	})
}

func (d *Dependencies) ServiceService() *service.Service {
	return d.cache.serviceService.Get(func() *service.Service {
		return service.NewService(d.ServiceRepo(), d.EnvironmentRepo(), d.ProjectRepo())
	})
}

func (d *Dependencies) PostgresService() *postgres.Service {
	return d.cache.postgresService.Get(func() *postgres.Service {
		return postgres.NewService(d.PostgresRepo(), d.EnvironmentRepo(), d.ProjectRepo())
	})
}

func (d *Dependencies) KeyValueService() *keyvalue.Service {
	return d.cache.keyValueService.Get(func() *keyvalue.Service {
		return keyvalue.NewService(d.KeyValueRepo(), d.EnvironmentRepo(), d.ProjectRepo())
	})
}

func (d *Dependencies) ResourceService() *resource.Service {
	return d.cache.resourceService.Get(func() *resource.Service {
		return resource.NewResourceService(
			d.ServiceService(),
			d.PostgresService(),
			d.KeyValueService(),
			d.EnvironmentRepo(),
			d.ProjectRepo(),
			d.WorkflowService(),
		)
	})
}

func (d *Dependencies) WorkflowLoader() *workflowviews.WorkflowLoader {
	return d.cache.workflowLoader.Get(func() *workflowviews.WorkflowLoader {
		return workflowviews.NewWorkflowLoader(d.TaskRepo(), d.WorkflowService(), d.WorkflowVersionRepo(), d.WorkflowRepo())
	})
}

func (d *Dependencies) WorkflowService() *workflow.Service {
	return d.cache.workflowService.Get(func() *workflow.Service {
		return workflow.NewService(d.WorkflowRepo(), d.EnvironmentRepo(), d.ProjectRepo())
	})
}

func (d *Dependencies) APIConfig() *config.APIConfig {
	return d.cache.apiConfig.Get(func() *config.APIConfig {
		cfg, err := config.DefaultAPIConfig()
		if err != nil {
			panic(err)
		}
		return &cfg
	})
}

func (d *Dependencies) LogRepo() *logs.LogRepo {
	return d.cache.logRepo.Get(func() *logs.LogRepo {
		return logs.NewLogRepo(d.client, d.APIConfig())
	})
}

func (d *Dependencies) LogLoader() *views.LogLoader {
	return d.cache.logLoader.Get(func() *views.LogLoader {
		return views.NewLogLoader(d.LogRepo(), d.ServiceRepo(), d.KeyValueRepo(), d.PostgresRepo(), d.WorkflowRepo())
	})
}

func (d *Dependencies) ResourceLoader() *views.ResourceLoader {
	return d.cache.resourceLoader.Get(func() *views.ResourceLoader {
		return views.NewResourceLoader(d.ResourceService())
	})
}
