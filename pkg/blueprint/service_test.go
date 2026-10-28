package blueprint_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func TestCreateInputValidate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    blueprint.CreateInput
		wantErrs []string
	}{
		{
			name:  "valid input with omitted optional flags",
			input: blueprint.CreateInput{Repo: new("https://github.com/acme/app")},
		},
		{
			name:     "HTTP repository URL is rejected",
			input:    blueprint.CreateInput{Repo: new("http://github.com/acme/app.git")},
			wantErrs: []string{"--repo must be a valid https:// or ssh:// URL"},
		},
		{
			name:  "SSH repository URL",
			input: blueprint.CreateInput{Repo: new("ssh://git@github.com/acme/app.git")},
		},
		{
			name:     "relative local path",
			input:    blueprint.CreateInput{Repo: new("../app")},
			wantErrs: []string{"--repo must be a valid https:// or ssh:// URL"},
		},
		{
			name:     "absolute local path",
			input:    blueprint.CreateInput{Repo: new("/tmp/app")},
			wantErrs: []string{"--repo must be a valid https:// or ssh:// URL"},
		},
		{
			name:  "SCP-style remote",
			input: blueprint.CreateInput{Repo: new("git@github.com:acme/app.git")},
		},
		{
			name:     "unsupported repository URL scheme",
			input:    blueprint.CreateInput{Repo: new("git://github.com/acme/app.git")},
			wantErrs: []string{"--repo must be a valid https:// or ssh:// URL"},
		},
		{
			name:     "repository URL without a host",
			input:    blueprint.CreateInput{Repo: new("https:///acme/app.git")},
			wantErrs: []string{"--repo must be a valid https:// or ssh:// URL"},
		},
		{
			name:     "missing repository",
			input:    blueprint.CreateInput{},
			wantErrs: []string{"--repo is required"},
		},
		{
			name:     "explicit empty repository",
			input:    blueprint.CreateInput{Repo: new("")},
			wantErrs: []string{"--repo is required"},
		},
		{
			name:     "malformed repository URL",
			input:    blueprint.CreateInput{Repo: new("https://github.com/acme/%gh")},
			wantErrs: []string{"--repo must be a valid https:// or ssh:// URL"},
		},
		{
			name:     "invalid existing resources mode",
			input:    blueprint.CreateInput{Repo: new("https://github.com/acme/app"), ExistingResources: new(bptypes.ExistingResourceMode("delete"))},
			wantErrs: []string{"--existing-resources must be create_new or adopt"},
		},
		{
			name: "reports multiple validation errors",
			input: blueprint.CreateInput{
				ExistingResources: new(bptypes.ExistingResourceMode("delete")),
				Path:              new(""),
				Branch:            new(""),
				Name:              new(""),
			},
			wantErrs: []string{
				"--repo is required",
				"--existing-resources must be create_new or adopt",
				"--path must not be empty",
				"--branch must not be empty",
				"--name must not be empty",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate(false)
			if len(tc.wantErrs) == 0 {
				require.NoError(t, err)
				return
			}
			for _, wantErr := range tc.wantErrs {
				require.ErrorContains(t, err, wantErr)
			}
		})
	}
}

func TestServiceCreateRequiresRepository(t *testing.T) {
	repo, server, body := newBlueprintRepoFixture(t)
	svc := blueprint.NewService(repo)

	result, err := svc.Create(context.Background(), body.OwnerId, blueprint.CreateInput{})
	require.ErrorContains(t, err, "--repo is required")
	require.Nil(t, result)
	require.Empty(t, server.Requests)
}

func TestServiceCreate(t *testing.T) {
	t.Run("builds a Create Blueprint API request from input", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			input blueprint.CreateInput
			want  bptypes.CreateBlueprintRequest
		}{
			{
				name: "omitted flags resolve to creation defaults",
				input: blueprint.CreateInput{
					Repo: new("https://github.com/acme/my-app.git"),
				},
				want: bptypes.CreateBlueprintRequest{
					Name:     "my-app",
					Source:   bptypes.BlueprintSource{Repo: "https://github.com/acme/my-app.git", Path: new("render.yaml")},
					AutoSync: new(true), ExistingResources: new(bptypes.CreateNew),
				},
			},
			{
				name: "SCP-style remote is normalized before deriving the name",
				input: blueprint.CreateInput{
					Repo: new("git@github.com:acme/my-app.git"),
				},
				want: bptypes.CreateBlueprintRequest{
					Name:     "my-app",
					Source:   bptypes.BlueprintSource{Repo: "https://github.com/acme/my-app.git", Path: new("render.yaml")},
					AutoSync: new(true), ExistingResources: new(bptypes.CreateNew),
				},
			},
			{
				name: "SSH repository URL is passed through unchanged",
				input: blueprint.CreateInput{
					Repo: new("ssh://git@github.com/acme/my-app.git"),
				},
				want: bptypes.CreateBlueprintRequest{
					Name:     "my-app",
					Source:   bptypes.BlueprintSource{Repo: "ssh://git@github.com/acme/my-app.git", Path: new("render.yaml")},
					AutoSync: new(true), ExistingResources: new(bptypes.CreateNew),
				},
			},
			{
				name: "explicit default values",
				input: blueprint.CreateInput{
					Repo: new("https://github.com/acme/my-app.git"), Path: new("render.yaml"),
					AutoSync: new(true), ExistingResources: new(bptypes.CreateNew),
				},
				want: bptypes.CreateBlueprintRequest{
					Name:     "my-app",
					Source:   bptypes.BlueprintSource{Repo: "https://github.com/acme/my-app.git", Path: new("render.yaml")},
					AutoSync: new(true), ExistingResources: new(bptypes.CreateNew),
				},
			},
			{
				name: "explicit values including disabled auto-sync",
				input: blueprint.CreateInput{
					Repo: new("https://github.com/acme/my-app.git"), Name: new("production"),
					Branch: new("release"), Path: new("infra/render.yaml"),
					AutoSync: new(false), ExistingResources: new(bptypes.AdoptExisting),
				},
				want: bptypes.CreateBlueprintRequest{
					Name:     "production",
					Source:   bptypes.BlueprintSource{Repo: "https://github.com/acme/my-app.git", Branch: new("release"), Path: new("infra/render.yaml")},
					AutoSync: new(false), ExistingResources: new(bptypes.AdoptExisting),
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				repo, server, body := newBlueprintRepoFixture(t)
				svc := blueprint.NewService(repo)
				_, err := svc.Create(context.Background(), body.OwnerId, tc.input)
				require.NoError(t, err)

				require.Len(t, server.Blueprints.Instances, 1)
				require.Len(t, server.Requests, 1)
				require.Equal(t, http.MethodPost, server.Requests[0].Method)
				require.Equal(t, "/blueprints", server.Requests[0].URI)
				var recorded bptypes.CreateBlueprintRequest
				require.NoError(t, json.Unmarshal(server.Requests[0].Body, &recorded))
				tc.want.OwnerId = body.OwnerId
				require.Equal(t, tc.want, recorded)
			})
		}
	})
}
