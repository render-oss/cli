package text

import (
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
)

func TestIPAllowListEqual(t *testing.T) {
	office := client.CidrBlockAndDescription{CidrBlock: "203.0.113.5/32", Description: "office"}
	internal := client.CidrBlockAndDescription{CidrBlock: "10.0.0.0/8", Description: "internal"}

	t.Run("same entries, same order", func(t *testing.T) {
		a := []client.CidrBlockAndDescription{office, internal}
		b := []client.CidrBlockAndDescription{office, internal}
		assert.True(t, ipAllowListEqual(a, b))
	})

	t.Run("same entries, different order", func(t *testing.T) {
		a := []client.CidrBlockAndDescription{office, internal}
		b := []client.CidrBlockAndDescription{internal, office}
		assert.True(t, ipAllowListEqual(a, b))
	})

	t.Run("different entries", func(t *testing.T) {
		a := []client.CidrBlockAndDescription{office}
		b := []client.CidrBlockAndDescription{internal}
		assert.False(t, ipAllowListEqual(a, b))
	})

	t.Run("different lengths", func(t *testing.T) {
		a := []client.CidrBlockAndDescription{office, internal}
		b := []client.CidrBlockAndDescription{office}
		assert.False(t, ipAllowListEqual(a, b))
	})

	t.Run("does not mutate the caller's slices", func(t *testing.T) {
		a := []client.CidrBlockAndDescription{office, internal}
		b := []client.CidrBlockAndDescription{internal, office}
		ipAllowListEqual(a, b)
		assert.Equal(t, []client.CidrBlockAndDescription{office, internal}, a)
		assert.Equal(t, []client.CidrBlockAndDescription{internal, office}, b)
	})
}
