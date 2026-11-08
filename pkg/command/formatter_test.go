package command_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/pointers"
)

type copyInput struct {
	Name     string                  `cli:"arg:0"`
	Second   *string                 `cli:"arg:1"`
	Slice    []string                `cli:"slice"`
	Array    []string                `cli:"array"`
	Enum     []string                `cli:"enum"`
	Single   string                  `cli:"single"`
	Optional *string                 `cli:"optional"`
	Count    *int                    `cli:"count"`
	Flag     bool                    `cli:"flag"`
	Since    *command.TimeOrRelative `cli:"since"`
}

func newCopyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "thing"}
	cmd.Flags().StringSlice("slice", nil, "")
	cmd.Flags().StringArray("array", nil, "")
	cmd.Flags().Var(command.NewEnumInput([]string{"a", "b", "c"}, true), "enum", "")
	cmd.Flags().Var(command.NewEnumInput([]string{"x", "y"}, false), "single", "")
	cmd.Flags().String("optional", "", "")
	cmd.Flags().Int("count", 0, "")
	cmd.Flags().Bool("flag", false, "")
	cmd.Flags().Var(command.NewTimeInput(), "since", "")
	return cmd
}

func parseCopyInput(t *testing.T, argv []string) copyInput {
	t.Helper()
	cmd := newCopyCmd()
	require.NoError(t, cmd.ParseFlags(argv))
	var v copyInput
	require.NoError(t, command.ParseCommand(cmd, cmd.Flags().Args(), &v))
	return v
}

func posixSplit(t *testing.T, s string) []string {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	out, err := exec.Command(sh, "-c", `eval "set -- $1"; printf '%s\0' "$@"`, "sh", s).Output()
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

func TestCommandNameRoundTrip(t *testing.T) {
	tcs := []struct {
		name string
		argv []string
	}{
		{name: "positional spaces and quotes", argv: []string{"my service", `it's "x"`}},
		{name: "string slice elements", argv: []string{"n", "--slice", `"a,b",c d,é`}},
		{name: "string array repeated", argv: []string{"n", "--array", "K=a,b", "--array", "L=c d", "--array", ""}},
		{name: "enum list", argv: []string{"n", "--enum", "a,c", "--single", "y"}},
		{name: "explicit scalars", argv: []string{"n", "--optional", "v w", "--count", "3", "--flag"}},
		{name: "relative time", argv: []string{"n", "--since", "5h"}},
		{name: "absolute time", argv: []string{"n", "--since", "2026-10-11T08:30:05.000000001Z"}},
		{name: "nil and defaults", argv: []string{"n"}},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			original := parseCopyInput(t, tc.argv)

			copied, err := command.CommandName(newCopyCmd(), &original)
			require.NoError(t, err)

			argv := posixSplit(t, copied)
			require.Equal(t, "thing", argv[0])
			replayed := parseCopyInput(t, argv[1:])

			if original.Since != nil {
				require.NotNil(t, replayed.Since)
				require.Equal(t, original.Since.Relative, replayed.Since.Relative)
				if original.Since.Relative == nil {
					require.True(t, original.Since.T.Equal(*replayed.Since.T))
				}
				original.Since, replayed.Since = nil, nil
			}
			require.Equal(t, original, replayed, copied)
		})
	}
}

func TestCommandNameFormatsFlagTypes(t *testing.T) {
	since := time.Date(2026, 10, 11, 8, 30, 5, 415654000, time.UTC)
	v := copyInput{
		Name:  "a b",
		Slice: []string{"LEFT,RIGHT"},
		Array: []string{"x", "y z"},
		Enum:  []string{"a", "b"},
		Since: &command.TimeOrRelative{T: &since},
	}

	str, err := command.CommandName(newCopyCmd(), &v)
	require.NoError(t, err)
	require.Equal(t, `thing 'a b' --slice='"LEFT,RIGHT"' --array=x --array='y z' --enum=a,b --since=2026-10-11T08:30:05.415654Z`, str)
}

func TestInputToStringQuoting(t *testing.T) {
	v := copyInput{
		Name:     "a b",
		Second:   pointers.From("c"),
		Slice:    []string{"LEFT RIGHT", "it's"},
		Optional: pointers.From(""),
		Since:    &command.TimeOrRelative{Relative: pointers.From("30m")},
	}

	str, err := command.InputToString(&v)
	require.NoError(t, err)
	require.Equal(t, `'a b' c --slice='LEFT RIGHT,it'\''s' --since=30m`, str)
}
