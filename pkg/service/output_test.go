package service

import (
	"testing"

	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
)

func TestNewDeleteOutFromModel(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		out := NewDeleteOutFromModel(serviceOutputTestModel())

		assert.Equal(t, "prj-123", *out.Data.ProjectID)
		assert.Equal(t, "Website", out.Data.ProjectName)
		assert.Equal(t, "production", out.Data.EnvironmentName)

		body := testrequire.AsJSONMap(t, out)
		data := testrequire.SubMap(t, body, "data")
		assert.Equal(t, "srv-123", data["id"])
		assert.Equal(t, "prj-123", data["projectId"])
		assert.Equal(t, "env-123", data["environmentId"])
		assert.NotContains(t, data, "ProjectName")
		assert.NotContains(t, data, "EnvironmentName")
	})
}

func TestNewUpdateOutFromModel(t *testing.T) {
	out := NewUpdateOutFromModel(serviceOutputTestModel())

	assert.Equal(t, "srv-123", out.Data.Id)
	assert.Equal(t, "prj-123", *out.Data.ProjectID)
	assert.Equal(t, "Website", out.Data.ProjectName)
	assert.Equal(t, "production", out.Data.EnvironmentName)

	body := testrequire.AsJSONMap(t, out)
	data := testrequire.SubMap(t, body, "data")
	assert.Equal(t, "srv-123", data["id"])
	assert.Equal(t, "prj-123", data["projectId"])
	assert.Equal(t, "env-123", data["environmentId"])
	assert.NotContains(t, body, "meta")
}

func TestNewUpdateOutFromModel_IncludesNullEnvironmentID(t *testing.T) {
	out := NewUpdateOutFromModel(&Model{
		Service: &client.Service{
			Id:      "srv-123",
			Name:    "my-api",
			OwnerId: "tea-123",
			Type:    client.WebService,
		},
	})

	body := testrequire.AsJSONMap(t, out)
	data := testrequire.SubMap(t, body, "data")
	assert.Contains(t, data, "environmentId")
	assert.Nil(t, data["environmentId"])
}

func serviceOutputTestModel() *Model {
	envID := "env-123"

	return &Model{
		Service: &client.Service{
			Id:            "srv-123",
			Name:          "my-api",
			OwnerId:       "tea-123",
			EnvironmentId: &envID,
			Type:          client.WebService,
		},
		Project: &client.Project{
			Id:   "prj-123",
			Name: "Website",
		},
		Environment: &client.Environment{
			Id:   "env-123",
			Name: "production",
		},
	}
}
