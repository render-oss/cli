package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// clearCISignalEnvironment keeps detection tests hermetic even when the test
// process itself runs on CI (GitHub Actions sets CI and GITHUB_ACTIONS for this
// repo's own test runs). t.Setenv restores the original values after each test.
func clearCISignalEnvironment(t *testing.T) {
	t.Helper()
	for _, signal := range supportedCISignals {
		t.Setenv(signal.envVar, "")
	}
}

func TestDetectCISignals(t *testing.T) {
	testCases := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "no CI environment",
			env:  map[string]string{},
			want: []string{},
		},
		{
			name: "generic CI true",
			env:  map[string]string{"CI": "true"},
			want: []string{"CI"},
		},
		{
			name: "generic CI 1",
			env:  map[string]string{"CI": "1"},
			want: []string{"CI"},
		},
		{
			name: "boolean rule is case-insensitive and trims whitespace",
			env:  map[string]string{"CI": " True "},
			want: []string{"CI"},
		},
		{
			name: "generic CI false",
			env:  map[string]string{"CI": "false"},
			want: []string{},
		},
		{
			name: "generic CI 0",
			env:  map[string]string{"CI": "0"},
			want: []string{},
		},
		{
			name: "generic CI yes is not activating",
			env:  map[string]string{"CI": "yes"},
			want: []string{},
		},
		{
			name: "jenkins url non-empty",
			env:  map[string]string{"JENKINS_URL": "https://ci.example.com/"},
			want: []string{"JENKINS_URL"},
		},
		{
			name: "jenkins url whitespace only is not activating",
			env:  map[string]string{"JENKINS_URL": "   "},
			want: []string{},
		},
		{
			name: "jenkins build tag prefix",
			env:  map[string]string{"BUILD_TAG": "jenkins-my-job-42"},
			want: []string{"BUILD_TAG"},
		},
		{
			name: "non-jenkins build tag is not activating",
			env:  map[string]string{"BUILD_TAG": "release-42"},
			want: []string{},
		},
		{
			name: "teamcity version non-empty",
			env:  map[string]string{"TEAMCITY_VERSION": "2025.03.2"},
			want: []string{"TEAMCITY_VERSION"},
		},
		{
			name: "bitbucket build number non-empty",
			env:  map[string]string{"BITBUCKET_BUILD_NUMBER": "17"},
			want: []string{"BITBUCKET_BUILD_NUMBER"},
		},
		{
			name: "vercel without generic CI is guarded off",
			env:  map[string]string{"VERCEL": "1"},
			want: []string{},
		},
		{
			name: "vercel with generic CI",
			env:  map[string]string{"VERCEL": "1", "CI": "1"},
			want: []string{"CI", "VERCEL"},
		},
		{
			name: "render runtime without generic CI is guarded off",
			env:  map[string]string{"RENDER": "true"},
			want: []string{},
		},
		{
			name: "render build with generic CI",
			env:  map[string]string{"RENDER": "true", "CI": "true"},
			want: []string{"CI", "RENDER"},
		},
		{
			name: "multiple active signals are sorted",
			env: map[string]string{
				"TEAMCITY_VERSION": "2025.03.2",
				"GITLAB_CI":        "true",
				"CI":               "true",
			},
			want: []string{"CI", "GITLAB_CI", "TEAMCITY_VERSION"},
		},
		{
			name: "unsupported variables are never emitted and values never leak",
			env: map[string]string{
				"CI":                 "true",
				"RENDER_FAKE_SECRET": "secret-canary-value",
			},
			want: []string{"CI"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clearCISignalEnvironment(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			require.Equal(t, tc.want, DetectCISignals())
		})
	}
}

// TestDetectCISignalsBooleanMarkers covers every boolean-marker provider with
// one activating and one non-activating value, without repeating the full
// boolean-rule variations already covered on CI above.
func TestDetectCISignalsBooleanMarkers(t *testing.T) {
	booleanMarkers := []string{
		"GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "BUILDKITE",
		"TF_BUILD", "TRAVIS", "NETLIFY",
	}
	for _, name := range booleanMarkers {
		t.Run(name, func(t *testing.T) {
			clearCISignalEnvironment(t)

			t.Setenv(name, "true")
			require.Equal(t, []string{name}, DetectCISignals())

			t.Setenv(name, "false")
			require.Equal(t, []string{}, DetectCISignals())
		})
	}
}

// TestDetectCISignalsReturnsNonNilEmptySlice pins the JSON contract:
// ci_signals must serialize as [] rather than null when nothing is active.
func TestDetectCISignalsReturnsNonNilEmptySlice(t *testing.T) {
	clearCISignalEnvironment(t)

	got := DetectCISignals()
	require.NotNil(t, got)
	require.Empty(t, got)
}
