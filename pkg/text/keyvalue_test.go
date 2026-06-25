package text_test

import (
	"testing"

	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/text"
	"github.com/stretchr/testify/assert"
)

func TestKeyValueDetail_OmitsProjectAndEnvironmentWhenUnset(t *testing.T) {
	kv := keyvalue.KeyValueOut{
		ID:            "red-abc123",
		Name:          "my-cache",
		OwnerID:       "tea-workspace",
		WorkspaceName: "My Workspace",
		Region:        client.Oregon,
		Status:        client.DatabaseStatusAvailable,
	}

	out := text.KeyValueDetail(&kv)

	assert.NotContains(t, out, "Project:")
	assert.NotContains(t, out, "Environment:")
}

func TestKeyValueDetail_HappyPath(t *testing.T) {
	projectID := "prj-project"
	envID := "evm-production"
	memoryPolicy := "allkeys-lru"
	kv := keyvalue.KeyValueOut{
		ID:              "red-abc123",
		Name:            "my-cache",
		OwnerID:         "tea-workspace",
		WorkspaceName:   "My Workspace",
		ProjectID:       pointers.From(projectID),
		ProjectName:     "My Project",
		EnvironmentID:   pointers.From(envID),
		EnvironmentName: "production",
		Plan:            client.KeyValuePlanStarter,
		Region:          client.Oregon,
		Status:          client.DatabaseStatusAvailable,
		MaxmemoryPolicy: pointers.From(memoryPolicy),
	}

	out := text.KeyValueDetail(&kv)

	testassert.ContainsInOrder(t, out,
		"Name: my-cache",
		"ID: red-abc123",
		"Workspace: My Workspace (tea-workspace)",
		"Project: My Project (prj-project)",
		"Environment: production (evm-production)",
		"Plan: starter",
		"Region: oregon",
		"Status: available",
		"Memory policy: allkeys-lru",
	)
}
