package text

import (
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/service"
	"github.com/stretchr/testify/assert"
)

func TestServiceDetail(t *testing.T) {
	envID := "env-12345678901234567890"
	projectID := "prj-12345678901234567890"
	out := service.ServiceOut{
		Service: client.Service{
			Id:            "srv-12345678901234567890",
			Name:          "my-api",
			Type:          client.WebService,
			OwnerId:       "tea-workspace",
			EnvironmentId: &envID,
			DashboardUrl:  "https://dashboard.render.com/web/srv-12345678901234567890",
			Repo:          pointers.From("https://github.com/render-examples/my-api"),
		},
		ProjectID:       &projectID,
		ProjectName:     "Website",
		EnvironmentName: "Production",
	}

	detail := ServiceDetail(&out)

	assert.Contains(t, detail, "Name: my-api")
	assert.Contains(t, detail, "ID: srv-12345678901234567890")
	assert.Contains(t, detail, "Type: web_service")
	assert.Contains(t, detail, "Owner ID: tea-workspace")
	assert.Contains(t, detail, "Project: Website (prj-12345678901234567890)")
	assert.Contains(t, detail, "Environment: Production (env-12345678901234567890)")
	assert.NotContains(t, detail, "Project ID:")
	assert.NotContains(t, detail, "Environment ID:")
	assert.Contains(t, detail, "Dashboard: https://dashboard.render.com/web/srv-12345678901234567890")
	assert.NotContains(t, detail, "Repo:")
}

func TestServiceDetailNil(t *testing.T) {
	assert.Empty(t, ServiceDetail(nil))
}
