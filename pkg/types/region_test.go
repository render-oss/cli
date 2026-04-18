package types_test

import (
	"testing"

	"github.com/render-oss/cli/v2/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestRegionValues(t *testing.T) {
	values := types.RegionValues()
	require.Equal(t, []string{
		"frankfurt",
		"ohio",
		"oregon",
		"singapore",
		"virginia",
	}, values)
}

func TestParseRegion(t *testing.T) {
	parsed, err := types.ParseRegion("oregon")
	require.NoError(t, err)
	require.Equal(t, types.RegionOregon, parsed)

	_, err = types.ParseRegion("invalid")
	require.Error(t, err)
}
