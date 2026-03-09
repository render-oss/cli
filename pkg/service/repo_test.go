package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestLooksLikeServiceID(t *testing.T) {
	require.True(t, looksLikeServiceID("srv-12345678901234567890"))
	require.True(t, looksLikeServiceID("crn-12345678901234567890"))
	require.False(t, looksLikeServiceID("srv-1234567890123456789!"))
	require.False(t, looksLikeServiceID("srv-short"))
	require.False(t, looksLikeServiceID("abc-12345678901234567890"))
}

func TestResolveServiceIDFromNameOrID(t *testing.T) {
	t.Run("falls back to name lookup when ID lookup returns not found", func(t *testing.T) {
		t.Setenv("RENDER_WORKSPACE", "tea-workspace")
		query := "srv-12345678901234567890"
		listCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/services/" + query:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(client.Error{Message: strPtr("service not found")})
			case "/services":
				listCalled = true
				require.Contains(t, r.URL.Query()["name"], query)
				require.Contains(t, r.URL.Query()["ownerId"], "tea-workspace")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]client.ServiceWithCursor{
					{
						Cursor: "cur-1",
						Service: client.Service{
							Id:   "srv-00000000000000000000",
							Name: query,
						},
					},
				})
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		repo := NewRepo(c)

		resolved, err := repo.ResolveServiceIDFromNameOrID(context.Background(), query)
		require.NoError(t, err)
		require.True(t, listCalled)
		require.Equal(t, "srv-00000000000000000000", resolved)
	})

	t.Run("does not fall back to name lookup when ID lookup is forbidden", func(t *testing.T) {
		t.Setenv("RENDER_WORKSPACE", "tea-workspace")
		query := "srv-12345678901234567890"
		listCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/services/" + query:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(client.Error{Message: strPtr("forbidden")})
			case "/services":
				listCalled = true
				t.Fatalf("unexpected list fallback call")
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		repo := NewRepo(c)

		_, err = repo.ResolveServiceIDFromNameOrID(context.Background(), query)
		require.ErrorIs(t, err, client.ErrForbidden)
		require.False(t, listCalled)
	})

	t.Run("requires exact name matches after list lookup", func(t *testing.T) {
		t.Setenv("RENDER_WORKSPACE", "tea-workspace")
		query := "my-service"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/services":
				require.Contains(t, r.URL.Query()["name"], query)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]client.ServiceWithCursor{
					{
						Cursor: "cur-1",
						Service: client.Service{
							Id:   "srv-11111111111111111111",
							Name: "my-service-staging",
						},
					},
				})
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		repo := NewRepo(c)

		_, err = repo.ResolveServiceIDFromNameOrID(context.Background(), query)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no service found")
	})
}

func strPtr(v string) *string {
	return &v
}
