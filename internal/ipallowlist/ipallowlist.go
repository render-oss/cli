package ipallowlist

import (
	"cmp"
	"slices"

	"github.com/render-oss/cli/pkg/client"
)

// Equal reports whether two allow-lists contain the same entries.
// An allow-list is conceptually a set, so the comparison is order-insensitive:
// it sorts copies before comparing, never the caller's live slices.
func Equal(a, b []client.CidrBlockAndDescription) bool {
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
