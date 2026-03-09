package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestFindRegistryCredentialsByIDOrName_EmptyInput(t *testing.T) {
	t.Run("rejects empty idOrName", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected API call for empty input")
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		service := NewService(NewRepo(c))

		_, err = service.FindRegistryCredentialsByIDOrName(context.Background(), "tea-owner", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "registry credential ID or name is required")
	})

	t.Run("rejects whitespace-only idOrName", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected API call for whitespace input")
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		service := NewService(NewRepo(c))

		_, err = service.FindRegistryCredentialsByIDOrName(context.Background(), "tea-owner", "   ")
		require.Error(t, err)
		require.Contains(t, err.Error(), "registry credential ID or name is required")
	})

	t.Run("rejects empty ownerID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected API call for empty ownerID")
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		service := NewService(NewRepo(c))

		_, err = service.FindRegistryCredentialsByIDOrName(context.Background(), "", "some-cred")
		require.Error(t, err)
		require.Contains(t, err.Error(), "workspace ID is required")
	})

	t.Run("rejects whitespace-only ownerID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected API call for whitespace ownerID")
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		service := NewService(NewRepo(c))

		_, err = service.FindRegistryCredentialsByIDOrName(context.Background(), "   ", "some-cred")
		require.Error(t, err)
		require.Contains(t, err.Error(), "workspace ID is required")
	})
}

func TestFindOneRegistryCredentialByIDFromNameOrID(t *testing.T) {
	t.Run("uses direct retrieve for full registry credential IDs", func(t *testing.T) {
		query := "rgc-12345678901234567890"
		listCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/registrycredentials/" + query:
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(client.RegistryCredential{
					Id:       query,
					Name:     "prod-registry",
					Registry: client.DOCKER,
					UpdatedAt: time.Date(
						2026, time.January, 1, 0, 0, 0, 0, time.UTC,
					),
					Username: "render",
				})
			case "/registrycredentials":
				listCalled = true
				t.Fatalf("unexpected list call for ID lookup")
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		service := NewService(NewRepo(c))

		id, err := service.FindOneRegistryCredentialByIDFromNameOrID(context.Background(), "tea-owner", query)
		require.NoError(t, err)
		require.Equal(t, query, id)
		require.False(t, listCalled)
	})

	t.Run("id-like names are resolved by exact name lookup", func(t *testing.T) {
		query := "rgc-prod"
		retrieveCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/registrycredentials/" + query:
				retrieveCalled = true
				t.Fatalf("unexpected direct retrieve call for name lookup")
			case "/registrycredentials":
				require.Contains(t, r.URL.Query()["name"], query)
				require.Contains(t, r.URL.Query()["ownerId"], "tea-owner")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]client.RegistryCredential{
					{
						Id:       "rgc-00000000000000000001",
						Name:     query,
						Registry: client.DOCKER,
						UpdatedAt: time.Date(
							2026, time.January, 2, 0, 0, 0, 0, time.UTC,
						),
						Username: "render",
					},
					{
						Id:       "rgc-00000000000000000002",
						Name:     "rgc-prod-2",
						Registry: client.DOCKER,
						UpdatedAt: time.Date(
							2026, time.January, 2, 0, 0, 0, 0, time.UTC,
						),
						Username: "render",
					},
				})
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		service := NewService(NewRepo(c))

		id, err := service.FindOneRegistryCredentialByIDFromNameOrID(context.Background(), "tea-owner", query)
		require.NoError(t, err)
		require.Equal(t, "rgc-00000000000000000001", id)
		require.False(t, retrieveCalled)
	})

	t.Run("returns not found when full ID retrieve misses", func(t *testing.T) {
		query := "rgc-12345678901234567890"
		listCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/registrycredentials/" + query:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(client.Error{Message: strPtr("registry credential not found")})
			case "/registrycredentials":
				listCalled = true
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		c, err := client.NewClientWithResponses(server.URL)
		require.NoError(t, err)
		service := NewService(NewRepo(c))

		_, err = service.FindOneRegistryCredentialByIDFromNameOrID(context.Background(), "tea-owner", query)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no registry credential found")
		require.False(t, listCalled)
	})
}

func strPtr(v string) *string {
	return &v
}
