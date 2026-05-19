package keyvalue_test

import (
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/pointers"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildUpdateRequest(t *testing.T) {
	t.Run("all fields together", func(t *testing.T) {
		body, err := keyvalue.BuildUpdateRequest(kvtypes.KeyValueUpdateInput{
			Name:            pointers.From("renamed"),
			Plan:            pointers.From("pro"),
			MaxmemoryPolicy: pointers.From(kvtypes.MaxmemoryPolicyNoeviction),
			IPAllowList:     []string{"cidr=10.0.0.0/8"},
		})
		require.NoError(t, err)
		require.NotNil(t, body.Name)
		assert.Equal(t, "renamed", *body.Name)
		require.NotNil(t, body.Plan)
		assert.Equal(t, client.KeyValuePlan("pro"), *body.Plan)
		require.NotNil(t, body.MaxmemoryPolicy)
		assert.Equal(t, client.Noeviction, *body.MaxmemoryPolicy)
		require.NotNil(t, body.IpAllowList)
		assert.Len(t, *body.IpAllowList, 1)
	})

	t.Run("omitted fields stay nil so JSON omits them", func(t *testing.T) {
		// Proves the if-checks short-circuit: an update that only sets Plan must
		// not send name=null/empty or memory-policy=null in the PATCH body.
		body, err := keyvalue.BuildUpdateRequest(kvtypes.KeyValueUpdateInput{
			Plan: pointers.From("starter"),
		})
		require.NoError(t, err)
		assert.Nil(t, body.Name, "name must be omitted, not sent as empty")
		assert.Nil(t, body.MaxmemoryPolicy)
		assert.Nil(t, body.IpAllowList)
		require.NotNil(t, body.Plan)
	})

	t.Run("ip-allow-list replace", func(t *testing.T) {
		body, err := keyvalue.BuildUpdateRequest(kvtypes.KeyValueUpdateInput{
			IPAllowList: []string{
				"cidr=10.0.0.0/8,description=internal",
				"cidr=203.0.113.5/32",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, body.IpAllowList)
		entries := *body.IpAllowList
		require.Len(t, entries, 2)
		assert.Equal(t, "10.0.0.0/8", entries[0].CidrBlock)
		assert.Equal(t, "internal", entries[0].Description)
		assert.Equal(t, "203.0.113.5/32", entries[1].CidrBlock)
		assert.Equal(t, "", entries[1].Description)
	})

	t.Run("ip-allow-list clear sends empty slice, not nil", func(t *testing.T) {
		body, err := keyvalue.BuildUpdateRequest(kvtypes.KeyValueUpdateInput{
			ClearIPAllowList: true,
		})
		require.NoError(t, err)
		require.NotNil(t, body.IpAllowList, "clear must serialize as []; nil would mean 'leave alone'")
		assert.Empty(t, *body.IpAllowList)
	})

	t.Run("invalid ip-allow-list entry returns error", func(t *testing.T) {
		_, err := keyvalue.BuildUpdateRequest(kvtypes.KeyValueUpdateInput{
			IPAllowList: []string{"not-a-cidr"},
		})
		require.Error(t, err)
	})
}
