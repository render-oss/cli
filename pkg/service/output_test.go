package service

import (
	"encoding/json"
	"testing"

	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDeleteOutFromModel(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		out := NewDeleteOutFromModel(&Model{
			Service: &client.Service{
				Id:            "srv-123",
				Name:          "my-api",
				OwnerId:       "tea-123",
				EnvironmentId: new("env-123"),
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
		})

		assert.Equal(t, "prj-123", *out.Data.ProjectID)
		assert.Equal(t, "Website", out.Data.ProjectName)
		assert.Equal(t, "production", out.Data.EnvironmentName)

		encoded, err := json.Marshal(out)
		require.NoError(t, err, "Serializes to JSON")

		var body map[string]any
		require.NoError(t, json.Unmarshal(encoded, &body), "Round-trips back to a map")

		data := testrequire.SubMap(t, body, "data")
		assert.Equal(t, "srv-123", data["id"])
		assert.Equal(t, "prj-123", data["projectId"])
		assert.Equal(t, "env-123", data["environmentId"])
		assert.NotContains(t, data, "ProjectName")
		assert.NotContains(t, data, "EnvironmentName")
	})
}
