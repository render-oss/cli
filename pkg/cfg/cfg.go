package cfg

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	RepoURL                     = "https://api.github.com/repos/render-oss/cli"
	InstallationInstructionsURL = "https://render.com/docs/cli#1-install-or-upgrade"
)

var (
	Version = "dev"
	osInfo  string
)

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
// This is independent of [AnalyticsDevGateOpen]: both can be toggled on/off independently.
func ShouldLogAnalytics() bool {
	return os.Getenv("RENDER_LOG_ANALYTICS") == "1"
}

// AnalyticsDevGateOpen reports whether the internal analytics rollout gate is
// open, via RENDER_TEST_ENABLE_ANALYTICS=1. Before we roll out the analytics
// system in production, sending events and showing the user-facing notice are
// both off by default and require this explicit opt-in.
func AnalyticsDevGateOpen() bool {
	return os.Getenv("RENDER_TEST_ENABLE_ANALYTICS") == "1"
}

// DoNotTrack reports DO_NOT_TRACK is truthy
// We must respect this by disabling analytics
func DoNotTrack() bool {
	return isTruthy(os.Getenv("DO_NOT_TRACK"))
}

// AnalyticsDisabledByEnv reports whether RENDER_CLI_DISABLE_ANALYTICS is truthy
// We must respect this by disabling analytics
func AnalyticsDisabledByEnv() bool {
	return isTruthy(os.Getenv("RENDER_CLI_DISABLE_ANALYTICS"))
}

// AnalyticsStrategy returns the raw configured analytics send strategy.
func AnalyticsStrategy() string {
	return os.Getenv("RENDER_CLI_ANALYTICS_STRATEGY")
}

// IsCI reports whether the CLI is running in a CI pipeline, indicated by a
// truthy CI environment variable ("true" in any case, or "1"). This is the
// single definition of CI-ness: output resolution and the analytics send
// strategy must not diverge on it.
func IsCI() bool {
	return isTruthy(os.Getenv("CI"))
}

// isTruthy reports whether an environment value expresses an enabled boolean
// flag: "1", or "true" in any case.
func isTruthy(value string) bool {
	return value == "1" || strings.EqualFold(value, "true")
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
