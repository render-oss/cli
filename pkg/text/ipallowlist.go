package text

import (
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/client"
)

// Shared IP allow-list rendering helpers, used by both Key Value and Postgres
// text output. The parsing counterpart lives in pkg/types/ipallowlist.go.

// ipAllowListBlock renders the allow-list as either a one-liner
// ("IP allow-list: empty (external connections blocked)") or a header line
// followed by indented entries — each "  - <cidr> (<description>)", or
// "  - <cidr>" when the description is empty.
func ipAllowListBlock(entries []client.CidrBlockAndDescription) string {
	if len(entries) == 0 {
		return "IP allow-list: empty (external connections blocked)"
	}
	var b strings.Builder
	b.WriteString("IP allow-list:")
	for _, e := range entries {
		if e.Description != "" {
			fmt.Fprintf(&b, "\n  - %s (%s)", e.CidrBlock, e.Description)
		} else {
			fmt.Fprintf(&b, "\n  - %s", e.CidrBlock)
		}
	}
	return b.String()
}

// ipAllowListLabel renders a compact count label for use in update diffs.
func ipAllowListLabel(entries []client.CidrBlockAndDescription) string {
	switch len(entries) {
	case 0:
		return "(empty)"
	case 1:
		return "1 entry"
	default:
		return fmt.Sprintf("%d entries", len(entries))
	}
}
