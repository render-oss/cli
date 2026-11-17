package updatenotice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/blang/semver/v4"

	"github.com/render-oss/cli/pkg/cfg"
)

// releaseLookupTimeout bounds a release lookup so a slow or unreachable GitHub
// never hangs the CLI.
const releaseLookupTimeout = 10 * time.Second

// Release is a published CLI release.
type Release struct {
	Version     semver.Version
	PublishedAt time.Time
}

// ReleaseClient looks up CLI releases on GitHub.
type ReleaseClient struct {
	latestReleaseURL string
	httpClient       *http.Client
}

// NewReleaseClient returns a client for the releases of the repository at
// repoURL, the GitHub API URL of the CLI repository.
func NewReleaseClient(repoURL string) *ReleaseClient {
	return &ReleaseClient{
		latestReleaseURL: strings.TrimSuffix(repoURL, "/") + "/releases/latest",
		httpClient:       &http.Client{Timeout: releaseLookupTimeout},
	}
}

// LatestRelease returns the latest published release.
func (c *ReleaseClient) LatestRelease(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.latestReleaseURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	// Not cfg.AddUserAgent: its lazily cached OS lookup is not safe to call
	// from the background update check while the API client also uses it.
	req.Header.Set("User-Agent", "render-cli/"+cfg.Version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Release{}, fmt.Errorf("getting latest release returned status %d: %s", resp.StatusCode, body)
	}

	var release struct {
		Tag         string    `json:"tag_name"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Release{}, err
	}

	v, err := semver.Parse(strings.TrimPrefix(release.Tag, "v"))
	if err != nil {
		return Release{}, fmt.Errorf("parsing release tag %q: %w", release.Tag, err)
	}
	return Release{Version: v, PublishedAt: release.PublishedAt}, nil
}
