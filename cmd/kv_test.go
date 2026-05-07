package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyValueAliasResolvesToKVCommand(t *testing.T) {
	short, _, err := rootCmd.Find([]string{"ea", "kv"})
	require.NoError(t, err)
	require.Same(t, kvCmd, short)

	alias, _, err := rootCmd.Find([]string{"ea", "keyvalue"})
	require.NoError(t, err)
	require.Same(t, kvCmd, alias)
}
