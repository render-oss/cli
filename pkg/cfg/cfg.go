package cfg

import (
	"fmt"
	"net/http"
	"os"
)

const RepoURL = "https://api.github.com/repos/render-oss/cli"
const InstallationInstructionsURL = "https://render.com/docs/cli#1-install-or-upgrade"

var Version = "dev"
var osInfo string

func GetHost() string {
	if host := os.Getenv("RENDER_HOST"); host != "" {
		return host
	}

	return "https://api.render.com/v1/"
}

func GetAPIKey() string {
	return os.Getenv("RENDER_API_KEY")
}

func GetRegion() string {
	return os.Getenv("RENDER_REGION")
}

// ShouldLogAnalytics reports whether the CLI should log analytics to stderr,
// enabled by RENDER_LOG_ANALYTICS=1.
// This is independent of [ShouldSendAnalytics]: both can be toggled on/off independently.
func ShouldLogAnalytics() bool {
	return os.Getenv("RENDER_LOG_ANALYTICS") == "1"
}

// ShouldSendAnalytics reports whether the CLI should send analytics events to
// the API, enabled by RENDER_TEST_ENABLE_ANALYTICS=1. While developing the telemetry system,
// sending is off by default and only fire when explicitly opted in.
func ShouldSendAnalytics() bool {
	return os.Getenv("RENDER_TEST_ENABLE_ANALYTICS") == "1"
}

// AnalyticsStrategy returns the raw configured analytics send strategy.
func AnalyticsStrategy() string {
	return os.Getenv("RENDER_CLI_ANALYTICS_STRATEGY")
}

func AddUserAgent(header http.Header) http.Header {
	header.Add("user-agent", fmt.Sprintf("render-cli/%s (%s)", Version, getOSInfoOnce()))
	return header
}

func getOSInfoOnce() string {
	if osInfo == "" {
		osInfo = getOSInfo()
	}
	return osInfo
}
