package analytics

import (
	"os"
	"slices"
	"strings"
)

// ciSignal is one supported automated-pipeline environment variable and its
// activation rule.
type ciSignal struct {
	// envVar is the raw environment variable name emitted in ci_signals.
	envVar string
	// isActive reports whether the variable's value marks an active CI/CD
	// pipeline. It receives the raw value; the value itself is never emitted.
	isActive func(value string) bool
	// requiresGenericCI guards platform markers that also exist outside
	// automated pipeline execution (Vercel and Render set their markers at
	// runtime too); they only count when the generic CI variable is active.
	requiresGenericCI bool
}

// supportedCISignals is the allow-list of CI environment variables the CLI
// reports in ci_signals, per the GROW-2745 research note. Here, CI includes any
// automated CI/CD pipeline; it does not imply that every listed platform
// offers a standalone CI product. Only names listed here can ever be emitted.
// To support a new provider, add a row here and cover it in cisignals_test.go;
// event construction needs no changes.
//
// Official sources:
//   - Generic CI / GitHub Actions: https://docs.github.com/en/actions/reference/workflows-and-actions/variables
//   - GitLab CI: https://docs.gitlab.com/ci/variables/predefined_variables/
//   - CircleCI: https://circleci.com/docs/reference/variables/
//   - Buildkite: https://buildkite.com/docs/pipelines/configure/environment-variables
//   - Jenkins: https://www.jenkins.io/doc/book/pipeline/jenkinsfile/#using-environment-variables
//   - TeamCity: https://www.jetbrains.com/help/teamcity/predefined-build-parameters.html
//   - Bitbucket Pipelines: https://support.atlassian.com/bitbucket-cloud/docs/variables-and-secrets/
//   - Azure Pipelines: https://learn.microsoft.com/en-us/azure/devops/pipelines/build/variables
//   - Travis CI: https://docs.travis-ci.com/user/environment-variables/#default-environment-variables
//   - Netlify: https://docs.netlify.com/build/configure-builds/environment-variables/
//   - Vercel: https://vercel.com/docs/environment-variables/system-environment-variables
//   - Render: https://render.com/docs/environment-variables
var supportedCISignals = []ciSignal{
	{envVar: "CI", isActive: isActiveBooleanMarker},
	{envVar: "GITHUB_ACTIONS", isActive: isActiveBooleanMarker},
	{envVar: "GITLAB_CI", isActive: isActiveBooleanMarker},
	{envVar: "CIRCLECI", isActive: isActiveBooleanMarker},
	{envVar: "BUILDKITE", isActive: isActiveBooleanMarker},
	{envVar: "JENKINS_URL", isActive: isNonEmptyMarker},
	{envVar: "BUILD_TAG", isActive: isJenkinsBuildTag},
	{envVar: "TEAMCITY_VERSION", isActive: isNonEmptyMarker},
	{envVar: "BITBUCKET_BUILD_NUMBER", isActive: isNonEmptyMarker},
	{envVar: "TF_BUILD", isActive: isActiveBooleanMarker},
	{envVar: "TRAVIS", isActive: isActiveBooleanMarker},
	{envVar: "NETLIFY", isActive: isActiveBooleanMarker},
	{envVar: "VERCEL", isActive: isActiveBooleanMarker, requiresGenericCI: true},
	{envVar: "RENDER", isActive: isActiveBooleanMarker, requiresGenericCI: true},
}

// DetectCISignals returns the sorted, deduplicated names of allow-listed CI
// environment variables that are active for this process. It never returns
// nil, so ci_signals serializes as [] rather than null.
func DetectCISignals() []string {
	genericCI := isActiveBooleanMarker(os.Getenv("CI"))
	signals := []string{}
	for _, signal := range supportedCISignals {
		if signal.requiresGenericCI && !genericCI {
			continue
		}
		if signal.isActive(os.Getenv(signal.envVar)) {
			signals = append(signals, signal.envVar)
		}
	}
	slices.Sort(signals)
	return slices.Compact(signals)
}

func isActiveBooleanMarker(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "1" || strings.EqualFold(trimmed, "true")
}

func isNonEmptyMarker(value string) bool {
	return strings.TrimSpace(value) != ""
}

func isJenkinsBuildTag(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "jenkins-")
}
