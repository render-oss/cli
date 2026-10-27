package text_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/text"
)

func TestSandboxGroupTable_ContainsHeadersAndRow(t *testing.T) {
	envID := "evm-abc"
	groups := []*sandboxesclient.SandboxGroup{
		{
			Id:            "sbg-abc",
			Name:          "Default",
			Region:        "oregon",
			IsDefault:     true,
			EnvironmentId: &envID,
			CreatedAt:     time.Now().Add(-2 * time.Hour),
			UpdatedAt:     time.Now(),
		},
	}
	out := text.SandboxGroupTable(groups)
	for _, want := range []string{"ID", "NAME", "REGION", "DEFAULT", "ENVIRONMENT", "AGE", "sbg-abc", "Default", "oregon", "evm-abc"} {
		assert.True(t, strings.Contains(out, want), "expected %q in output:\n%s", want, out)
	}
}

func TestSandboxGroupTable_NoEnvironmentShowsDash(t *testing.T) {
	groups := []*sandboxesclient.SandboxGroup{
		{Id: "sbg-xyz", Name: "Default", Region: "oregon", IsDefault: true, CreatedAt: time.Now()},
	}
	out := text.SandboxGroupTable(groups)
	assert.True(t, strings.Contains(out, "sbg-xyz"))
	assert.True(t, strings.Contains(out, " - "), "unbound environment should render as -:\n%s", out)
}

func TestSandboxGroupDetail_ShowsFields(t *testing.T) {
	envID := "evm-abc"
	group := &sandboxesclient.SandboxGroup{
		Id:               "sbg-abc",
		Name:             "agents",
		Region:           "oregon",
		IsDefault:        false,
		ConcurrencyLimit: 10,
		EnvironmentId:    &envID,
		CreatedAt:        time.Date(2026, 7, 2, 18, 30, 0, 0, time.UTC),
	}
	out := text.SandboxGroupDetail(group)
	for _, want := range []string{
		"ID:                sbg-abc",
		"Name:              agents",
		"Region:            oregon",
		"Default:           false",
		"Concurrency limit: 10",
		"Environment:       evm-abc",
		"Created:           2026-07-02T18:30:00Z",
	} {
		assert.True(t, strings.Contains(out, want), "expected %q in output:\n%s", want, out)
	}
}

func TestSandboxGroupDetail_NoEnvironmentShowsDash(t *testing.T) {
	group := &sandboxesclient.SandboxGroup{Id: "sbg-xyz", Name: "Default", Region: "oregon", IsDefault: true, CreatedAt: time.Now()}
	out := text.SandboxGroupDetail(group)
	assert.True(t, strings.Contains(out, "Environment:       -"), "unbound environment should render as -:\n%s", out)
}
