package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/renderinc/render-cli/pkg/resource/util"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/redis"
	"github.com/renderinc/render-cli/pkg/service"
)

const redisResourceIDPrefix = "red-"
const postgresResourceIDPrefix = "dpg-"
const serverResourceIDPrefix = "srv-"
const cronjobResourceIDPrefix = "crn-"

type Resource interface {
	ID() string
	Name() string
	EnvironmentName() string
	ProjectName() string
	Type() string
}

type Service struct {
	serviceService  *service.Service
	postgresService *postgres.Service
	redisService    *redis.Service
	environmentRepo *environment.Repo
	projectRepo     *project.Repo
}

func NewDefaultResourceService() (*Service, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	serviceRepo := service.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	postgresRepo := postgres.NewRepo(c)
	redisRepo := redis.NewRepo(c)

	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)
	postgresService := postgres.NewService(postgresRepo, environmentRepo, projectRepo)
	redisService := redis.NewService(redisRepo, environmentRepo, projectRepo)

	return NewResourceService(
		serviceService,
		postgresService,
		redisService,
		environmentRepo,
		projectRepo,
	), nil
}

func NewResourceService(serviceService *service.Service, postgresService *postgres.Service, redisService *redis.Service, environmentRepo *environment.Repo, projectRepo *project.Repo) *Service {
	return &Service{
		serviceService:  serviceService,
		postgresService: postgresService,
		redisService:    redisService,
		environmentRepo: environmentRepo,
		projectRepo:     projectRepo,
	}
}

type ResourceParams struct {
	EnvironmentIDs  []string
	IncludePreviews bool
}

func (r ResourceParams) ToServiceParams() *client.ListServicesParams {
	params := &client.ListServicesParams{
		IncludePreviews: pointers.From(r.IncludePreviews),
	}

	if len(r.EnvironmentIDs) > 0 {
		params.EnvironmentId = pointers.From(r.EnvironmentIDs)
	}

	return params
}

func (r ResourceParams) ToPostgresParams() *client.ListPostgresParams {
	if len(r.EnvironmentIDs) == 0 {
		return &client.ListPostgresParams{}
	}

	return &client.ListPostgresParams{
		EnvironmentId: pointers.From(r.EnvironmentIDs),
	}
}

func (r ResourceParams) ToRedisParams() *client.ListRedisParams {
	if len(r.EnvironmentIDs) == 0 {
		return &client.ListRedisParams{}
	}

	return &client.ListRedisParams{
		EnvironmentId: pointers.From(r.EnvironmentIDs),
	}
}

func (rs *Service) ListResources(ctx context.Context, params ResourceParams) ([]Resource, error) {
	var services []*service.Model
	var postgresDBs []*postgres.Model
	var redisDBs []*redis.Model
	wg, _ := errgroup.WithContext(ctx)
	wg.Go(func() error {
		var err error
		services, err = rs.serviceService.ListServices(ctx, params.ToServiceParams())
		return err
	})

	wg.Go(func() error {
		var err error
		postgresDBs, err = rs.postgresService.ListPostgres(ctx, params.ToPostgresParams())
		return err
	})

	wg.Go(func() error {
		var err error
		redisDBs, err = rs.redisService.ListRedis(ctx, params.ToRedisParams())
		return err
	})

	err := wg.Wait()
	if err != nil {
		return nil, err
	}

	var resources []Resource

	for _, svc := range services {
		resources = append(resources, svc)
	}

	for _, db := range postgresDBs {
		resources = append(resources, db)
	}

	for _, db := range redisDBs {
		resources = append(resources, db)
	}

	util.SortResources(resources)

	return resources, nil
}

func (rs *Service) GetResource(ctx context.Context, id string) (Resource, error) {
	if strings.HasPrefix(id, serverResourceIDPrefix) {
		return rs.serviceService.GetService(ctx, id)
	}

	if strings.HasPrefix(id, postgresResourceIDPrefix) {
		return rs.postgresService.GetPostgres(ctx, id)
	}

	return nil, errors.New("unknown resource type")
}

func (rs *Service) RestartResource(ctx context.Context, id string) error {
	if strings.HasPrefix(id, serverResourceIDPrefix) {
		return rs.serviceService.RestartService(ctx, id)
	}

	if strings.HasPrefix(id, postgresResourceIDPrefix) {
		return rs.postgresService.RestartPostgresDatabase(ctx, id)
	}

	if strings.HasPrefix(id, cronjobResourceIDPrefix) {
		return errors.New("cron jobs cannot be restarted")
	}

	if strings.HasPrefix(id, redisResourceIDPrefix) {
		return errors.New("redises cannot be restarted")
	}

	return errors.New("unknown resource type")
}

func GetResource(ctx context.Context, id string) (Resource, error) {
	rs, err := NewDefaultResourceService()
	if err != nil {
		return nil, err
	}

	return rs.GetResource(ctx, id)
}

func BreadcrumbForResource(r Resource) string {
	if r.ProjectName() != "" && r.EnvironmentName() != "" {
		return fmt.Sprintf("%s (%s - %s)", r.Name(), r.ProjectName(), r.EnvironmentName())
	}
	return r.Name()
}
