package text_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/text"
)

func TestSandboxTable_NetworkPolicyColumns(t *testing.T) {
	https := sandboxesclient.Https

	tests := []struct {
		name        string
		policy      sandboxesclient.SandboxNetworkPolicy
		wantNetwork string
		wantAllowed string
	}{
		{
			name:        "allow-all has no allowed domains",
			policy:      sandboxesclient.SandboxNetworkPolicy{Type: sandboxesclient.AllowAll},
			wantNetwork: "allow-all",
			wantAllowed: "-",
		},
		{
			name:        "deny-all has no allowed domains",
			policy:      sandboxesclient.SandboxNetworkPolicy{Type: sandboxesclient.DenyAll},
			wantNetwork: "deny-all",
			wantAllowed: "-",
		},
		{
			name: "a single domain is shown without its protocol",
			policy: sandboxesclient.SandboxNetworkPolicy{
				Type: sandboxesclient.AllowList,
				Rules: pointers.From([]sandboxesclient.SandboxEgressRule{
					{Domain: "api.github.com", Protocol: &https},
				}),
			},
			wantNetwork: "allow-list",
			wantAllowed: "api.github.com",
		},
		{
			name: "multiple domains are shown as a count",
			policy: sandboxesclient.SandboxNetworkPolicy{
				Type: sandboxesclient.AllowList,
				Rules: pointers.From([]sandboxesclient.SandboxEgressRule{
					{Domain: "api.github.com", Protocol: &https},
					{Domain: "*.pypi.org"},
					{Domain: "registry.npmjs.org"},
				}),
			},
			wantNetwork: "allow-list",
			wantAllowed: "3 domains",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := text.SandboxTable([]*sandboxesclient.Sandbox{{
				Id:            "sbx-abc",
				Status:        sandboxesclient.SandboxStatusRunning,
				Plan:          sandboxesclient.Starter,
				Region:        "oregon",
				CreatedAt:     time.Now(),
				NetworkPolicy: tt.policy,
			}})

			assert.Equal(t, tt.wantNetwork, tableCell(t, out, "NETWORK"))
			assert.Equal(t, tt.wantAllowed, tableCell(t, out, "ALLOWED"))
		})
	}
}

// tableCell returns the value under column in a single-row table, using the
// header's position so values containing spaces stay intact.
func tableCell(t *testing.T, out, column string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 2, "expected a header and one row, got:\n%s", out)
	header, row := lines[0], lines[1]

	start := strings.Index(header, column)
	require.GreaterOrEqual(t, start, 0, "no %s column in:\n%s", column, header)

	end := len(row)
	afterName := start + len(column)
	if next := strings.IndexFunc(header[afterName:], func(r rune) bool { return r != ' ' }); next >= 0 {
		end = min(end, afterName+next)
	}
	return strings.TrimSpace(row[start:end])
}
