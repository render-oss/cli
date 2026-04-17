package service_test

import (
	"testing"

	servicetypes "github.com/render-oss/cli/v2/pkg/types/service"
	"github.com/stretchr/testify/require"
)

func TestServiceRuntimeValues(t *testing.T) {
	values := servicetypes.ServiceRuntimeValues()
	require.Equal(t, []string{
		"docker",
		"elixir",
		"go",
		"image",
		"node",
		"python",
		"ruby",
		"rust",
	}, values)
}

func TestParseServiceRuntime(t *testing.T) {
	parsed, err := servicetypes.ParseServiceRuntime("node")
	require.NoError(t, err)
	require.Equal(t, servicetypes.ServiceRuntimeNode, parsed)

	_, err = servicetypes.ParseServiceRuntime("invalid")
	require.Error(t, err)
}

func TestServiceRuntime_IsNative(t *testing.T) {
	t.Run("native runtimes return true", func(t *testing.T) {
		require.True(t, servicetypes.ServiceRuntimeNode.IsNative())
		require.True(t, servicetypes.ServiceRuntimePython.IsNative())
		require.True(t, servicetypes.ServiceRuntimeGo.IsNative())
		require.True(t, servicetypes.ServiceRuntimeRuby.IsNative())
		require.True(t, servicetypes.ServiceRuntimeRust.IsNative())
		require.True(t, servicetypes.ServiceRuntimeElixir.IsNative())
	})

	t.Run("non-native runtimes return false", func(t *testing.T) {
		require.False(t, servicetypes.ServiceRuntimeDocker.IsNative())
		require.False(t, servicetypes.ServiceRuntimeImage.IsNative())
	})

	t.Run("empty runtime returns false", func(t *testing.T) {
		require.False(t, servicetypes.ServiceRuntime("").IsNative())
	})
}
