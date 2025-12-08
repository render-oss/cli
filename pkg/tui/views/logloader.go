package views

import (
	"context"
	"fmt"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/workflow"
)

type LogLoader struct {
	logRepo      *logs.LogRepo
	serviceRepo  *service.Repo
	kvRepo       *keyvalue.Repo
	postgresRepo *postgres.Repo
	workflowRepo *workflow.Repo
}

func NewLogLoader(logRepo *logs.LogRepo, serviceRepo *service.Repo, kvRepo *keyvalue.Repo, postgresRepo *postgres.Repo, workflowRepo *workflow.Repo) *LogLoader {
	return &LogLoader{logRepo: logRepo, serviceRepo: serviceRepo, kvRepo: kvRepo, postgresRepo: postgresRepo, workflowRepo: workflowRepo}
}

func (l *LogLoader) LoadLogData(ctx context.Context, in LogInput) (*tui.LogResult, error) {
	params, err := l.ToParam(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("error processing arguments: %v", err)
	}

	if in.Tail {
		logChan, err := l.logRepo.TailLogs(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("error tailing logs: %v", err)
		}
		return &tui.LogResult{Logs: &client.Logs200Response{}, LogChannel: logChan}, nil
	}

	logs, err := l.logRepo.ListLogs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error listing logs: %v", err)
	}
	return &tui.LogResult{Logs: logs, LogChannel: nil}, nil
}

func (l *LogLoader) getResourceIDsFromIDOrNames(ctx context.Context, idOrNames []string) ([]string, error) {
	resourceIds := make([]string, len(idOrNames))

	for i, idOrName := range idOrNames {
		if matchesResourceId(idOrName) {
			// This will error out if we have a name that looks like a resource ID but isn't one.
			// Ideally we'd like to catch that case and allow looking up by name for such resources.
			// However, checking if the resource ID is valid would be a performance hit, and doesn't
			// seem worth it considering how unlikely such a name is.
			resourceIds[i] = idOrName
			continue
		}

		// We have a name, not an ID. See if we can find a match

		services, err := l.serviceRepo.ListServices(ctx, &client.ListServicesParams{
			Name: &client.NameParam{idOrName},
		})
		if err != nil {
			return nil, err
		}

		if len(services) == 1 {
			resourceIds[i] = services[0].Id
			continue
		}

		kvs, err := l.kvRepo.ListKeyValue(ctx, &client.ListKeyValueParams{
			Name: &client.NameParam{idOrName},
		})
		if err != nil {
			return nil, err
		}

		if len(kvs) == 1 {
			resourceIds[i] = kvs[0].Id
			continue
		}

		postgreses, err := l.postgresRepo.ListPostgres(ctx, &client.ListPostgresParams{
			Name: &client.NameParam{idOrName},
		})
		if err != nil {
			return nil, err
		}

		if len(postgreses) == 1 {
			resourceIds[i] = postgreses[0].Id
			continue
		}

		workflows, err := l.workflowRepo.ListWorkflows(ctx, &client.ListWorkflowsParams{
			Name: &client.NameParam{idOrName},
		})
		if err != nil {
			return nil, err
		}

		if len(workflows) == 1 {
			resourceIds[i] = workflows[0].Id
			continue
		}

		return nil, fmt.Errorf("no resource found with ID or name '%s'", idOrName)
	}

	return resourceIds, nil
}

func (l *LogLoader) ToParam(ctx context.Context, in LogInput) (*client.ListLogsParams, error) {
	ownerID, err := config.WorkspaceID()
	if err != nil {
		return nil, fmt.Errorf("error getting workspace ID: %v", err)
	}

	if in.Limit == 0 {
		in.Limit = logs.DefaultLogLimit
	}

	var startTime *time.Time
	if in.StartTime != nil {
		startTime = in.StartTime.T
	}

	var endTime *time.Time
	if in.EndTime != nil {
		endTime = in.EndTime.T
	}

	resourceIDs, err := l.getResourceIDsFromIDOrNames(ctx, in.ResourceIDs)
	if err != nil {
		return nil, err
	}

	return &client.ListLogsParams{
		Resource:   resourceIDs,
		OwnerId:    ownerID,
		Instance:   pointers.FromArray(in.Instance),
		Limit:      pointers.From(in.Limit),
		StartTime:  startTime,
		EndTime:    endTime,
		Text:       pointers.FromArray(in.Text),
		Level:      pointers.FromArray(in.Level),
		Type:       pointers.FromArray(in.Type),
		Host:       pointers.FromArray(in.Host),
		StatusCode: pointers.FromArray(in.StatusCode),
		Method:     pointers.FromArray(in.Method),
		Path:       pointers.FromArray(in.Path),
		Direction:  pointers.From(mapDirection(in.Direction)),
		Task:       pointers.FromArray(in.TaskID),
		TaskRun:    pointers.FromArray(in.TaskRunID),
	}, nil
}
