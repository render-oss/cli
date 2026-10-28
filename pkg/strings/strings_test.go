package strings

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestSanitizeTerminal(t *testing.T) {
	// SGR (Select Graphic Rendition) is the ESC [ ... m escape code that sets
	// colors and text styles; it's the only escape code SanitizeTerminal keeps.
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"keeps tabs and newlines", "a\tb\nc", "a\tb\nc"},
		{"keeps unicode", "héllo ✓ 日本", "héllo ✓ 日本"},
		{"keeps SGR color and appends reset", "\x1b[31mred\x1b[0m", "\x1b[31mred\x1b[0m\x1b[0m"},
		{"keeps truecolor SGR", "\x1b[1;38;2;255;0;0mred", "\x1b[1;38;2;255;0;0mred\x1b[0m"},
		{"keeps colon-separated SGR", "\x1b[38:2::255:0:0mred", "\x1b[38:2::255:0:0mred\x1b[0m"},
		{"keeps SGR with many params", "\x1b[" + strings.Repeat("1;", 200) + "31mx", "\x1b[" + strings.Repeat("1;", 200) + "31mx\x1b[0m"},
		{"strips private-prefix lookalike of SGR", "a\x1b[>4;2mb", "ab"},
		{"raw 8-bit C1 SGR introducer becomes U+FFFD, rest is text", "a\x9b31mb", "a\uFFFD31mb"},
		{"strips cursor move and line erase", "a\x1b[1A\x1b[2Kb", "ab"},
		{"strips OSC 8 hyperlink", "a\x1b]8;;https://attacker.example\x1b\\click\x1b]8;;\x1b\\b", "aclickb"},
		{"strips OSC 52 clipboard write", "a\x1b]52;c;QUJD\x07b", "ab"},
		{"replaces raw 8-bit C1 byte (invalid UTF-8) with U+FFFD", "a\x9bHb", "a\uFFFDHb"},
		{"strips encoded C1 CSI introducer", "a\u009bHb", "aHb"},
		{"strips bare C0 bytes", "a\rb\x07c\x08d\x00e", "abcde"},
		{"strips DEL", "a\x7fb", "ab"},
		{"strips trailing ESC", "a\x1b", "a"},
		{"replaces invalid byte with U+FFFD", "a\xffb", "a\uFFFDb"},
		{"replaces each invalid byte with U+FFFD", "a\xff\xfeb", "a\uFFFD\uFFFDb"},
		{"replaces truncated multibyte sequence", "a\xc3b", "a\uFFFDb"},
		{"replaces invalid continuation byte", "a\xe2\x28\xa1b", "a\uFFFD(\uFFFDb"},
		{"replaces invalid byte inside SGR and keeps the style", "\x1b[31m\xff\x1b[0m", "\x1b[31m\uFFFD\x1b[0m\x1b[0m"},
		{"keeps existing replacement character", "a\uFFFDb", "a\uFFFDb"},
		{"strips UTF-8 encoded C1 control", "a\u009bb", "ab"},
		{"keeps combining marks", "é", "é"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, SanitizeTerminal(tt.in))
		})
	}
}

func TestSanitizeTerminalPlain(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"strips SGR color and does not append reset", "\x1b[31mred\x1b[0m plain", "red plain"},
		{"strips truecolor SGR", "\x1b[1;38;2;255;0;0mred", "red"},
		{"still strips other escapes", "a\x1b[1A\x1b[2K\x1b]8;;https://attacker.example\x1b\\b\x1b]8;;\x1b\\", "ab"},
		{"still strips C0 bytes and replaces invalid UTF-8", "a\rb\xffc", "ab\uFFFDc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, SanitizeTerminalPlain(tt.in))
		})
	}
}

// FuzzSanitizeTerminal checks the invariants that matter for hostile input:
// the function terminates, returns valid UTF-8, and lets no escape sequence
// other than SGR through.
func FuzzSanitizeTerminal(f *testing.F) {
	for _, seed := range []string{
		"",
		"plain",
		"\x1b",
		"\x1b[",
		"\x1b[31m",
		"\x1b]8;;https://example.com\x1b\\x\x1b]8;;\x1b\\",
		"\x1b]52;c;QUJD\x07",
		"\x1b[" + strings.Repeat("1;", 200) + "m",
		"\x9b31m",
		"\xff\xfe\xc3\xe2\x28\xa1",
		"a\rb\x07c\x08d\x00e\x7f",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out := SanitizeTerminal(in)
		require.True(t, utf8.ValidString(out), "output must be valid UTF-8: %q", out)
		rest := out
		for {
			i := strings.IndexByte(rest, '\x1b')
			if i < 0 {
				break
			}
			seq, _, n, _ := ansi.DecodeSequence(rest[i:], 0, nil)
			require.True(t, isSGR(seq), "non-SGR escape survived: %q in %q", seq, out)
			rest = rest[i+n:]
		}
	})
}
