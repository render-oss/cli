package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/blang/semver/v4"

	"github.com/renderinc/cli/pkg/cfg"
)

type Client struct {
	latestReleaseURL string
	currentVersion   *semver.Version
}

func NewClient(repoURL string) *Client {
	latestReleaseURL := strings.TrimSuffix(repoURL, "/") + "/releases/latest"
	currentVersion, err := semver.Parse(cfg.Version)
	if err != nil {
		return &Client{}
	}

	return &Client{
		latestReleaseURL: latestReleaseURL,
		currentVersion:   &currentVersion,
	}
}

type releaseResp struct {
	Tag string `json:"tag_name"`
}

func (vc *Client) NewVersionAvailable() (string, error) {
	// the user has built from source so we can't check for newer versions
	if vc.currentVersion == nil {
		return "", nil
	}

	resp, err := http.Get(vc.latestReleaseURL)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("getting latest version returned status %d", resp.StatusCode)
		}

		return "", fmt.Errorf("getting latest version returned status %d: %s", resp.StatusCode, body)
	}

	var release releaseResp
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		return "", err
	}

	newestVersion, err := semver.Parse(strings.TrimPrefix(release.Tag, "v"))
	if err != nil {
		return "", err
	}

	if newestVersion.GT(*vc.currentVersion) {
		return newestVersion.String(), nil
	}

	return "", nil
}
