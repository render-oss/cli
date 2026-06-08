package keyvalue

import (
	"time"

	"github.com/render-oss/cli/pkg/client"
)

// KeyValueOut is the JSON/YAML contract for a Key Value instance.
type KeyValueOut struct {
	ID              string                           `json:"id"`
	Name            string                           `json:"name"`
	Plan            client.KeyValuePlan              `json:"plan"`
	Region          client.Region                    `json:"region"`
	Status          client.DatabaseStatus            `json:"status"`
	CreatedAt       time.Time                        `json:"createdAt"`
	UpdatedAt       time.Time                        `json:"updatedAt"`
	Version         string                           `json:"version,omitempty"`
	OwnerID         string                           `json:"ownerId"`
	OwnerType       client.OwnerType                 `json:"ownerType,omitempty"`
	ProjectID       *string                          `json:"projectId"`
	ProjectName     string                           `json:"-"`
	EnvironmentID   *string                          `json:"environmentId"`
	EnvironmentName string                           `json:"-"`
	ConnectionInfo  *client.KeyValueConnectionInfo   `json:"connectionInfo,omitempty"`
	IPAllowList     []client.CidrBlockAndDescription `json:"ipAllowList"`
	MaxmemoryPolicy *string                          `json:"maxmemoryPolicy,omitempty"`
}

type DeleteOut struct {
	Data KeyValueOut   `json:"data"`
	Meta DeleteOutMeta `json:"meta"`
}

type DeleteOutMeta struct {
	Deleted bool   `json:"deleted"`
	Message string `json:"message,omitempty"`
}

type SuspendOut struct {
	Data KeyValueOut    `json:"data"`
	Meta SuspendOutMeta `json:"meta"`
}

type SuspendOutMeta struct {
	Suspended bool   `json:"suspended"`
	Message   string `json:"message,omitempty"`
}

func NewKeyValueOut(resolved *ResolvedKeyValue) KeyValueOut {
	if resolved == nil || resolved.KeyValue == nil {
		return KeyValueOut{}
	}

	kv := resolved.KeyValue
	out := KeyValueOut{
		ID:          kv.Id,
		Name:        kv.Name,
		Plan:        kv.Plan,
		Region:      kv.Region,
		Status:      kv.Status,
		CreatedAt:   kv.CreatedAt,
		UpdatedAt:   kv.UpdatedAt,
		Version:     kv.Version,
		OwnerID:     kv.Owner.Id,
		OwnerType:   kv.Owner.Type,
		IPAllowList: kv.IpAllowList,
	}
	if out.IPAllowList == nil {
		out.IPAllowList = []client.CidrBlockAndDescription{}
	}
	if kv.Options.MaxmemoryPolicy != nil {
		out.MaxmemoryPolicy = kv.Options.MaxmemoryPolicy
	}
	if kv.EnvironmentId != nil {
		out.EnvironmentID = kv.EnvironmentId
	}
	if resolved.Environment != nil {
		out.EnvironmentID = &resolved.Environment.Id
		out.EnvironmentName = resolved.Environment.Name
	}
	if resolved.Project != nil {
		out.ProjectID = &resolved.Project.Id
		out.ProjectName = resolved.Project.Name
	}
	return out
}
