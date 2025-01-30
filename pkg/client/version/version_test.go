package version_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client/version"
)

func TestClient_NewVersionAvailable(t *testing.T) {
	t.Run("it returns the new version when a newer version is available", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"tag_name": "v9.9.9"}`))
			require.NoError(t, err)
		}))

		previousVersion := cfg.Version
		t.Cleanup(func() {
			cfg.Version = previousVersion
		})

		cfg.Version = "1.0.0"

		c := version.NewClient(s.URL)
		newVersion, err := c.NewVersionAvailable()
		require.NoError(t, err)

		assert.Equal(t, newVersion, "9.9.9")
	})

	t.Run("it returns empty when on the newest version", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"tag_name": "v1.0.0"}`))
			require.NoError(t, err)
		}))

		previousVersion := cfg.Version
		t.Cleanup(func() {
			cfg.Version = previousVersion
		})

		cfg.Version = "1.0.0"

		c := version.NewClient(s.URL)
		newVersion, err := c.NewVersionAvailable()
		require.NoError(t, err)

		assert.Zero(t, newVersion)
	})

	t.Run("it returns empty when compiled from source", func(t *testing.T) {
		c := version.NewClient("")
		newVersion, err := c.NewVersionAvailable()
		require.NoError(t, err)

		assert.Zero(t, newVersion)
	})
}
