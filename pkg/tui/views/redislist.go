package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/redis"
	redistui "github.com/render-oss/cli/pkg/redis/tui"
	"github.com/render-oss/cli/pkg/tui"
)

type RedisList struct {
	table *tui.Table[*redis.Model]
}

func NewRedisList(ctx context.Context, selectFunc OnSelectFuncT[*redis.Model], input RedisInput, opts ...tui.TableOption[*redis.Model]) *RedisList {
	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		p, ok := rows[0].Data["resource"].(*redis.Model)
		if !ok {
			return nil
		}

		return selectFunc(ctx, p)
	}

	createRowFunc := func(p *redis.Model) btable.Row {
		return redistui.Row(p)
	}

	t := tui.NewTable(
		redistui.Columns(),
		command.LoadCmd(ctx, listRedises, input),
		createRowFunc,
		onSelect,
		opts...,
	)

	return &RedisList{
		table: t,
	}
}

type RedisInput struct {
	EnvironmentIDs []string
}

func listRedises(ctx context.Context, input RedisInput) ([]*redis.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	redisRepo := redis.NewRepo(c)

	redisService := redis.NewService(redisRepo, environmentRepo, projectRepo)

	params := &client.ListRedisParams{}
	if input.EnvironmentIDs != nil {
		params.EnvironmentId = &input.EnvironmentIDs
	}
	return redisService.ListRedis(ctx, params)
}

func (pl *RedisList) Init() tea.Cmd {
	return pl.table.Init()
}

func (pl *RedisList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return pl.table.Update(msg)
}

func (pl *RedisList) View() string {
	return pl.table.View()
}
