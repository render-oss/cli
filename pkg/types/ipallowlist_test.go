package types_test

import (
	"testing"

	"github.com/render-oss/cli/v2/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIPAllowListEntry(t *testing.T) {
	t.Run("parses cidr with description", func(t *testing.T) {
		cidr, desc, err := types.ParseIPAllowListEntry("cidr=10.0.0.0/8,description=Internal")
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.0/8", cidr)
		assert.Equal(t, "Internal", desc)
	})

	t.Run("parses cidr without description", func(t *testing.T) {
		cidr, desc, err := types.ParseIPAllowListEntry("cidr=10.0.0.0/8")
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.0/8", cidr)
		assert.Equal(t, "", desc)
	})

	t.Run("handles IPv6 CIDR", func(t *testing.T) {
		cidr, desc, err := types.ParseIPAllowListEntry("cidr=2001:db8::/32,description=IPv6 range")
		require.NoError(t, err)
		assert.Equal(t, "2001:db8::/32", cidr)
		assert.Equal(t, "IPv6 range", desc)
	})

	t.Run("rejects malformed input missing cidr key", func(t *testing.T) {
		_, _, err := types.ParseIPAllowListEntry("malformed")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must start with cidr=")
	})

	t.Run("rejects empty cidr value", func(t *testing.T) {
		_, _, err := types.ParseIPAllowListEntry("cidr=")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cidr value is empty")
	})

	t.Run("rejects empty cidr value with description", func(t *testing.T) {
		_, _, err := types.ParseIPAllowListEntry("cidr=,description=foo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cidr value is empty")
	})

	t.Run("rejects invalid CIDR", func(t *testing.T) {
		_, _, err := types.ParseIPAllowListEntry("cidr=not-a-cidr")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid CIDR")
	})

	t.Run("rejects IP without prefix length", func(t *testing.T) {
		_, _, err := types.ParseIPAllowListEntry("cidr=10.0.0.1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid CIDR")
	})
}

func TestFormatIPAllowListEntry(t *testing.T) {
	t.Run("formats cidr with description", func(t *testing.T) {
		result := types.FormatIPAllowListEntry("10.0.0.0/8", "Internal")
		assert.Equal(t, "cidr=10.0.0.0/8,description=Internal", result)
	})

	t.Run("formats cidr without description", func(t *testing.T) {
		result := types.FormatIPAllowListEntry("10.0.0.0/8", "")
		assert.Equal(t, "cidr=10.0.0.0/8", result)
	})

	t.Run("formats IPv6 cidr with description", func(t *testing.T) {
		result := types.FormatIPAllowListEntry("2001:db8::/32", "IPv6 range")
		assert.Equal(t, "cidr=2001:db8::/32,description=IPv6 range", result)
	})

	t.Run("formats IPv6 cidr without description", func(t *testing.T) {
		result := types.FormatIPAllowListEntry("2001:db8::/32", "")
		assert.Equal(t, "cidr=2001:db8::/32", result)
	})
}
