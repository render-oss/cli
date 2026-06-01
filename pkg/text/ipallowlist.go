package text

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/render-oss/cli/pkg/client"
)

// Shared IP allow-list rendering helpers, used by both Key Value and Postgres
// text output. The parsing counterpart lives in pkg/types/ipallowlist.go.

// ipAllowListBlock renders the allow-list as either a one-liner ("IP allow-list: (empty)")
// or a header line followed by indented entries — each "  - <cidr> (<description>)" or
// "  - <cidr>" when description is empty.
func ipAllowListBlock(entries []client.CidrBlockAndDescription) string {
	if len(entries) == 0 {
		return "IP allow-list: (empty)"
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

// ipAllowListEqual reports whether two allow-lists contain the same entries.
// An allow-list is conceptually a set, so the comparison is order-insensitive:
// it sorts copies (never the caller's slices, which are live server data)
// before comparing, so a reordered list is not reported as a change.
func ipAllowListEqual(a, b []client.CidrBlockAndDescription) bool {
	if len(a) != len(b) {
		return false
	}
	as := slices.Clone(a)
	bs := slices.Clone(b)
	byKey := func(x, y client.CidrBlockAndDescription) int {
		return cmp.Or(
			cmp.Compare(x.CidrBlock, y.CidrBlock),
			cmp.Compare(x.Description, y.Description),
		)
	}
	slices.SortFunc(as, byKey)
	slices.SortFunc(bs, byKey)
	return slices.Equal(as, bs)
}
