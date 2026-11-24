package cmd

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/tui/views"
)

// shellSplit splits a copied command into argv the way a POSIX shell does.
func shellSplit(t *testing.T, s string) []string {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	out, err := exec.Command(sh, "-c", `eval "set -- $1"; printf '%s\0' "$@"`, "sh", s).Output()
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

func parseLogInput(t *testing.T, argv []string) (*views.LogInput, error) {
	t.Helper()
	cmd := NewLogsCmd(nil)
	format := command.Interactive
	cmd.SetContext(command.SetFormatInContext(context.Background(), &format))
	if err := cmd.ParseFlags(argv); err != nil {
		return nil, err
	}
	var input views.LogInput
	if err := command.ParseCommand(cmd, cmd.Flags().Args(), &input); err != nil {
		return nil, err
	}
	return &input, nil
}

// TestLogsCopyCommandRoundTrip verifies that the command shown and copied for
// a log view parses back, through the real logs flag parser, to the same input.
func TestLogsCopyCommandRoundTrip(t *testing.T) {
	tcs := []struct {
		name string
		argv []string
	}{
		{name: "phrase with space", argv: []string{"--resources", "srv-1", "--type", "app", "--text", "LEFT RIGHT"}},
		{name: "literal comma", argv: []string{"--resources", "srv-1", "--type", "app", "--text", `"LEFT,RIGHT"`}},
		{name: "relative start", argv: []string{"--resources", "srv-1", "--type", "app", "--text", "plain", "--start", "30m"}},
		{name: "plain control", argv: []string{"--resources", "srv-1", "--type", "app", "--text", "plain"}},
		{name: "two OR terms", argv: []string{"--resources", "srv-1", "--type", "app", "--text", "LEFT,RIGHT"}},
		{name: "absolute bounds with nanoseconds", argv: []string{"--resources", "srv-1", "--start", "2026-10-11T08:30:05.415654123-07:00", "--end", "2026-10-11T09:00:00Z"}},
		{name: "quotes, unicode and shell metacharacters", argv: []string{"--resources", "srv-1,srv-2", "--text", `"say ""hi""",it's,$HOME,*,日本語，テスト, lead`}},
		{name: "empty element neighbor", argv: []string{"--resources", "srv-1", "--text", `,a`}},
		{name: "enums and scalars", argv: []string{"--resources", "srv-1", "--level", "error,warning", "--type", "app,request", "--method", "GET,POST", "--direction", "forward", "--limit", "5", "--tail"}},
		{name: "repeated slice flag", argv: []string{"--resources", "srv-1", "--text", "a b", "--text", "c"}},
		{name: "path with spaces", argv: []string{"--resources", "srv-1", "--path", "/a b,/c", "--status-code", "500"}},
		{name: "lone carriage return", argv: []string{"--resources", "srv-1", "--text", "\"a\rb\""}},
		{name: "defaults only", argv: []string{}},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			original, err := parseLogInput(t, tc.argv)
			require.NoError(t, err)

			copied, err := command.CommandName(NewLogsCmd(nil), original)
			require.NoError(t, err)
			require.NotContains(t, copied, "{")
			require.True(t, strings.HasPrefix(copied, "logs"), copied)

			argv := shellSplit(t, copied)
			require.Equal(t, "logs", argv[0])
			replayed, err := parseLogInput(t, argv[1:])
			require.NoError(t, err, copied)

			requireSameTime(t, original.StartTime, replayed.StartTime)
			requireSameTime(t, original.EndTime, replayed.EndTime)
			original.StartTime, original.EndTime = nil, nil
			replayed.StartTime, replayed.EndTime = nil, nil
			require.Equal(t, original, replayed, copied)
		})
	}
}

func TestLogsCopyCommandPreservesSearchElements(t *testing.T) {
	original, err := parseLogInput(t, []string{"--resources", "srv-1", "--text", "LEFT RIGHT", "--start", "30m"})
	require.NoError(t, err)

	copied, err := command.CommandName(NewLogsCmd(nil), original)
	require.NoError(t, err)
	require.Contains(t, copied, "--text='LEFT RIGHT'")
	require.Contains(t, copied, "--start=30m")

	original, err = parseLogInput(t, []string{"--resources", "srv-1", "--text", `"LEFT,RIGHT"`})
	require.NoError(t, err)
	require.Equal(t, []string{"LEFT,RIGHT"}, original.Text)

	copied, err = command.CommandName(NewLogsCmd(nil), original)
	require.NoError(t, err)
	require.Contains(t, copied, `--text='"LEFT,RIGHT"'`)
}

func requireSameTime(t *testing.T, want, got *command.TimeOrRelative) {
	t.Helper()
	if want == nil {
		require.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	if want.Relative != nil {
		require.NotNil(t, got.Relative)
		require.Equal(t, *want.Relative, *got.Relative)
		return
	}
	require.Nil(t, got.Relative)
	require.True(t, want.T.Equal(*got.T), "want %s, got %s", want.T, got.T)
}

// TestTailLogsCopyCommandKeepsStartTime covers the automatic start time set by
// TailLogsFlow, which has no relative text.
func TestTailLogsCopyCommandKeepsStartTime(t *testing.T) {
	start := time.Date(2026, 10, 11, 8, 30, 5, 415654123, time.FixedZone("PDT", -7*60*60))
	input := views.LogInput{
		ResourceIDs: []string{"srv-1"},
		StartTime:   &command.TimeOrRelative{T: &start},
		Tail:        true,
	}

	copied, err := command.CommandName(NewLogsCmd(nil), &input)
	require.NoError(t, err)
	require.Contains(t, copied, "--start=2026-10-11T08:30:05.415654123-07:00")

	replayed, err := parseLogInput(t, shellSplit(t, copied)[1:])
	require.NoError(t, err)
	require.True(t, start.Equal(*replayed.StartTime.T))
	require.True(t, replayed.Tail)
}
