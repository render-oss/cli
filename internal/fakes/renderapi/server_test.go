package renderapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
)

// newServerAndClient returns an empty fake server and an API client for it.
func newServerAndClient(t *testing.T) (*Server, *client.ClientWithResponses) {
	t.Helper()
	server := NewServer(t)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return server, c
}

func TestMetadataReadErrorsAreQueuedPerOperation(t *testing.T) {
	for _, resource := range []string{"projects", "environments"} {
		t.Run(resource, func(t *testing.T) {
			server := NewServer(t)
			owner := server.Owners.Add(NewOwner(client.Owner{}))
			seeded := server.CreateProject(ProjectAttrs{OwnerId: owner.Id}, EnvAttrs{Name: "production"})

			getPath := "/projects/" + seeded.Project.Id
			listPath := "/projects"
			queueGet := server.Projects.RespondWithErrorForGet
			queueList := server.Projects.RespondWithErrorForList
			if resource == "environments" {
				getPath = "/environments/" + seeded.Env("production").Id
				listPath = "/environments"
				queueGet = server.Environments.RespondWithErrorForGet
				queueList = server.Environments.RespondWithErrorForList
			}

			queueGet(http.StatusForbidden)
			queueGet(http.StatusNotFound)
			queueList(http.StatusInternalServerError)
			queueList(http.StatusUnauthorized)

			for _, request := range []struct {
				path       string
				wantStatus int
			}{
				{getPath, http.StatusForbidden},
				{listPath, http.StatusInternalServerError},
				{getPath, http.StatusNotFound},
				{getPath, http.StatusOK},
				{listPath, http.StatusUnauthorized},
				{listPath, http.StatusOK},
			} {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL()+request.path, nil)
				require.NoError(t, err)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, request.wantStatus, resp.StatusCode, "GET %s", request.path)
			}
		})
	}
}
