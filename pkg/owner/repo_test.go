package owner_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/owner"
)

func TestRepo_ListOwners(t *testing.T) {
	t.Run("returns list of owners", func(t *testing.T) {
		owners := []client.OwnerWithCursor{
			{Owner: &client.Owner{Id: "tea-abc123", Name: "Team Alpha", Email: "alpha@example.com"}},
			{Owner: &client.Owner{Id: "usr-def456", Name: "User Beta", Email: "beta@example.com"}},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/owners", r.URL.Path)
			assert.Equal(t, "100", r.URL.Query().Get("limit"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(owners)
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		repo := owner.NewRepo(c)
		result, err := repo.ListOwners(context.Background(), owner.ListInput{})

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "tea-abc123", result[0].Id)
		assert.Equal(t, "Team Alpha", result[0].Name)
		assert.Equal(t, "usr-def456", result[1].Id)
		assert.Equal(t, "User Beta", result[1].Name)
	})

	t.Run("filters by name", func(t *testing.T) {
		owners := []client.OwnerWithCursor{
			{Owner: &client.Owner{Id: "tea-abc123", Name: "Team Alpha", Email: "alpha@example.com"}},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/owners", r.URL.Path)
			assert.Contains(t, r.URL.Query()["name"], "Team Alpha")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(owners)
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		repo := owner.NewRepo(c)
		result, err := repo.ListOwners(context.Background(), owner.ListInput{Name: "Team Alpha"})

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Team Alpha", result[0].Name)
	})

	t.Run("returns empty list when no owners", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]client.OwnerWithCursor{})
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		repo := owner.NewRepo(c)
		result, err := repo.ListOwners(context.Background(), owner.ListInput{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestRepo_RetrieveOwner(t *testing.T) {
	t.Run("returns owner by id", func(t *testing.T) {
		expectedOwner := &client.Owner{
			Id:    "tea-abc123",
			Name:  "Team Alpha",
			Email: "alpha@example.com",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/owners/tea-abc123", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(expectedOwner)
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		repo := owner.NewRepo(c)
		result, err := repo.RetrieveOwner(context.Background(), "tea-abc123")

		require.NoError(t, err)
		assert.Equal(t, "tea-abc123", result.Id)
		assert.Equal(t, "Team Alpha", result.Name)
		assert.Equal(t, "alpha@example.com", result.Email)
	})

	t.Run("returns error when owner not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		repo := owner.NewRepo(c)
		_, err = repo.RetrieveOwner(context.Background(), "nonexistent")

		require.Error(t, err)
	})
}
