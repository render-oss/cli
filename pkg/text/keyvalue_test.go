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

func basicKeyValue() *keyvalue.KeyValueOut {
	return &keyvalue.KeyValueOut{
		ID:     "key-abc123",
		Name:   "my-kv",
		Plan:   client.KeyValuePlanStarter,
		Region: client.Oregon,
		Status: client.DatabaseStatusAvailable,
	}
}

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

func TestKeyValueDetail_IPAllowList(t *testing.T) {
	t.Run("explains that empty allow-list blocks external connections", func(t *testing.T) {
		kv := basicKeyValue()
		assert.Contains(t, text.KeyValueDetail(kv), "IP allow-list: empty (external connections blocked)")
	})

	t.Run("renders populated entries", func(t *testing.T) {
		kv := basicKeyValue()
		kv.IPAllowList = []client.CidrBlockAndDescription{
			{CidrBlock: "10.0.0.0/8", Description: "internal"},
			{CidrBlock: "203.0.113.5/32"},
		}
		out := text.KeyValueDetail(kv)
		assert.Contains(t, out, "IP allow-list:")
		assert.Contains(t, out, "10.0.0.0/8 (internal)")
		assert.Contains(t, out, "203.0.113.5/32")
		assert.NotContains(t, out, "external connections blocked")
	})
}
