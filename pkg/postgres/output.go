package postgres

import (
	"reflect"

	"github.com/render-oss/cli/internal/ipallowlist"
	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/pointers"
)

// PostgresListItemOut is the JSON/YAML contract for a Postgres list item.
type PostgresListItemOut struct {
	client.Postgres
	ProjectID       *string `json:"projectId"`
	ProjectName     string  `json:"-"`
	EnvironmentName string  `json:"-"`
}

// PostgresOut is the JSON/YAML contract for a Postgres detail.
type PostgresOut struct {
	client.PostgresDetail
	ProjectID       *string                        `json:"projectId"`
	ProjectName     string                         `json:"-"`
	EnvironmentName string                         `json:"-"`
	ConnectionInfo  *client.PostgresConnectionInfo `json:"connectionInfo,omitempty"`
}

type PostgresListOut struct {
	Data []PostgresListItemOut `json:"data"`
}

type GetOut struct {
	Data PostgresOut `json:"data"`
}

type CreateOut = GetOut

type DeleteOut struct {
	Data PostgresOut   `json:"data"`
	Meta DeleteOutMeta `json:"meta"`
}

type DeleteOutMeta struct {
	Deleted bool   `json:"deleted"`
	Message string `json:"message,omitempty"`
}

type ResumeOut = GetOut

type SuspendOut struct {
	Data PostgresOut    `json:"data"`
	Meta SuspendOutMeta `json:"meta"`
}

type SuspendOutMeta struct {
	Suspended bool   `json:"suspended"`
	Message   string `json:"message,omitempty"`
}

type PostgresUpdateOut struct {
	Data PostgresOut        `json:"data"`
	Diff PostgresUpdateDiff `json:"diff"`
}

type PostgresUpdateDiff struct {
	Name                    *PostgresFieldDiff[string]                             `json:"name,omitempty"`
	Plan                    *PostgresFieldDiff[pgclient.PostgresPlans]             `json:"plan,omitempty"`
	DiskSizeGB              *PostgresFieldDiff[*int]                               `json:"diskSizeGB,omitempty"`
	DiskAutoscalingEnabled  *PostgresFieldDiff[bool]                               `json:"diskAutoscalingEnabled,omitempty"`
	HighAvailabilityEnabled *PostgresFieldDiff[bool]                               `json:"highAvailabilityEnabled,omitempty"`
	IPAllowList             *PostgresFieldDiff[[]client.CidrBlockAndDescription]   `json:"ipAllowList,omitempty"`
	ParameterOverrides      *PostgresFieldDiff[*client.PostgresParameterOverrides] `json:"parameterOverrides,omitempty"`
}

type PostgresFieldDiff[T any] struct {
	Before T `json:"before"`
	After  T `json:"after"`
}

func NewPostgresGetOut(resolved *ResolvedPostgres) GetOut {
	return GetOut{Data: newPostgresOut(resolved)}
}

func NewPostgresCreateOut(resolved *ResolvedPostgres) CreateOut {
	return CreateOut{Data: newPostgresOut(resolved)}
}

func NewPostgresResumeOut(resolved *ResolvedPostgres) ResumeOut {
	return ResumeOut{Data: newPostgresOut(resolved)}
}

func NewPostgresDeleteOut(resolved *ResolvedPostgres) DeleteOut {
	return DeleteOut{Data: newPostgresOut(resolved)}
}

func NewPostgresSuspendOut(resolved *ResolvedPostgres) SuspendOut {
	return SuspendOut{Data: newPostgresOut(resolved)}
}

func newPostgresOut(resolved *ResolvedPostgres) PostgresOut {
	if resolved == nil || resolved.Postgres == nil {
		return PostgresOut{}
	}

	out := PostgresOut{
		PostgresDetail: *resolved.Postgres,
	}
	finalizePostgresOut(&out, resolved.Project, resolved.Environment)
	return out
}

func NewPostgresListOut(models []*Model) PostgresListOut {
	data := make([]PostgresListItemOut, 0, len(models))
	for _, model := range models {
		data = append(data, newPostgresListItemOutFromModel(model))
	}
	return PostgresListOut{Data: data}
}

func newPostgresListItemOutFromModel(model *Model) PostgresListItemOut {
	if model == nil || model.Postgres == nil {
		return PostgresListItemOut{}
	}
	return newPostgresListItemOutFromPostgres(model.Postgres, model.Project, model.Environment)
}

func NewPostgresUpdateOut(before *client.PostgresDetail, after *ResolvedPostgres) PostgresUpdateOut {
	out := PostgresUpdateOut{
		Data: newPostgresOut(after),
	}
	if before == nil {
		return out
	}
	out.Diff = newPostgresUpdateDiff(before, &out.Data)
	return out
}

func newPostgresUpdateDiff(before *client.PostgresDetail, after *PostgresOut) PostgresUpdateDiff {
	var diff PostgresUpdateDiff
	if before == nil || after == nil {
		return diff
	}

	if before.Name != after.Name {
		diff.Name = newPostgresFieldDiff(before.Name, after.Name)
	}
	if before.Plan != after.Plan {
		diff.Plan = newPostgresFieldDiff(before.Plan, after.Plan)
	}
	if !pointers.Equal(before.DiskSizeGB, after.DiskSizeGB) {
		diff.DiskSizeGB = newPostgresFieldDiff(before.DiskSizeGB, after.DiskSizeGB)
	}
	if before.DiskAutoscalingEnabled != after.DiskAutoscalingEnabled {
		diff.DiskAutoscalingEnabled = newPostgresFieldDiff(before.DiskAutoscalingEnabled, after.DiskAutoscalingEnabled)
	}
	if before.HighAvailabilityEnabled != after.HighAvailabilityEnabled {
		diff.HighAvailabilityEnabled = newPostgresFieldDiff(before.HighAvailabilityEnabled, after.HighAvailabilityEnabled)
	}
	if !ipallowlist.Equal(before.IpAllowList, after.IpAllowList) {
		diff.IPAllowList = newPostgresFieldDiff(before.IpAllowList, after.IpAllowList)
	}
	beforeOverrides := normalizePostgresParameterOverrides(before.ParameterOverrides)
	afterOverrides := normalizePostgresParameterOverrides(after.ParameterOverrides)
	if !reflect.DeepEqual(beforeOverrides, afterOverrides) {
		diff.ParameterOverrides = newPostgresFieldDiff(
			beforeOverrides,
			afterOverrides,
		)
	}
	return diff
}

func newPostgresListItemOutFromPostgres(
	pg *client.Postgres,
	project *client.Project,
	env *client.Environment,
) PostgresListItemOut {
	if pg == nil {
		return PostgresListItemOut{}
	}

	out := PostgresListItemOut{
		Postgres: *pg,
	}
	finalizePostgresListItemOut(&out, project, env)
	return out
}

func finalizePostgresOut(out *PostgresOut, project *client.Project, env *client.Environment) {
	if out.IpAllowList == nil {
		out.IpAllowList = []client.CidrBlockAndDescription{}
	}
	if out.ReadReplicas == nil {
		out.ReadReplicas = client.ReadReplicas{}
	}
	if out.ParameterOverrides != nil && len(*out.ParameterOverrides) == 0 {
		out.ParameterOverrides = nil
	}
	if env != nil {
		out.EnvironmentId = &env.Id
		out.EnvironmentName = env.Name
	}
	if project != nil {
		out.ProjectID = &project.Id
		out.ProjectName = project.Name
	}
}

func finalizePostgresListItemOut(out *PostgresListItemOut, project *client.Project, env *client.Environment) {
	if out.IpAllowList == nil {
		out.IpAllowList = []client.CidrBlockAndDescription{}
	}
	if out.ReadReplicas == nil {
		out.ReadReplicas = client.ReadReplicas{}
	}
	if env != nil {
		out.EnvironmentId = &env.Id
		out.EnvironmentName = env.Name
	}
	if project != nil {
		out.ProjectID = &project.Id
		out.ProjectName = project.Name
	}
}

func newPostgresFieldDiff[T any](before, after T) *PostgresFieldDiff[T] {
	return &PostgresFieldDiff[T]{
		Before: before,
		After:  after,
	}
}

func normalizePostgresParameterOverrides(overrides *client.PostgresParameterOverrides) *client.PostgresParameterOverrides {
	if overrides == nil || len(*overrides) == 0 {
		return nil
	}
	return overrides
}
