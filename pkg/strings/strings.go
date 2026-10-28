package strings

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ResourceLabel renders a human-friendly identifier for a Render resource by
// combining its name and ID as "name (id)". When only one piece is available,
// or when the two are equal (e.g. resolved from an env var), the duplicate is
// elided.
func ResourceLabel(name, id string) string {
	switch {
	case name != "" && id != "" && name != id:
		return fmt.Sprintf("%s (%s)", name, id)
	case name != "":
		return name
	default:
		return id
	}
}

// TitleCaseValue converts a snake case string to title case
func TitleCaseValue(s string) string {
	words := strings.Split(s, "_")
	caser := cases.Title(language.English)
	for i, word := range words {
		words[i] = caser.String(word)
	}
	return strings.Join(words, " ")
}

func StripNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", "")
}

// SanitizeTerminal neutralizes a string for display on a terminal while
// keeping its colors and text styles. It keeps SGR (Select Graphic
// Rendition) sequences, the ESC [ ... m escape codes terminals use only to
// set colors and text styles such as bold or underline,
// drops every other escape sequence (CSI, OSC, DCS, etc.) and all other C0/C1
// control bytes except tabs and newlines, and replaces invalid UTF-8 with
// U+FFFD. If any SGR sequence was kept, it appends a reset so styling can't
// leak past the string. Use it on any server- or user-supplied text (log
// messages, commit messages) before writing it to a TTY, so that content
// can't move the cursor, erase lines, inject hyperlinks, or write to the
// clipboard.
func SanitizeTerminal(s string) string {
	// Normalize to valid UTF-8 first so the sequence decoder never sees raw
	// invalid bytes: a stray 0x80-0x9F byte is a decoding error in a UTF-8
	// terminal, not a C1 control, and must not be allowed to start a
	// sequence that swallows the text after it.
	s = replaceInvalidUTF8(s)

	var b strings.Builder
	b.Grow(len(s))

	var state byte
	styled := false
	for len(s) > 0 {
		// A nil parser: DecodeSequence panics on CSI sequences with more
		// parameters than a parser's buffer holds, and we only need the
		// sequence boundaries.
		seq, _, n, newState := ansi.DecodeSequence(s, state, nil)
		// A zero-length step would spin forever and an empty seq would panic
		// on seq[0] below.
		if n == 0 || seq == "" {
			break
		}
		state = newState
		s = s[n:]

		if isSGR(seq) {
			b.WriteString(seq)
			styled = true
			continue
		}
		// Every other escape sequence starts with ESC; drop it whole.
		if seq[0] == ansi.ESC {
			continue
		}
		for _, r := range seq {
			switch {
			case r == '\t' || r == '\n':
				b.WriteRune(r)
			case r == utf8.RuneError:
				b.WriteRune(utf8.RuneError)
			case unicode.IsControl(r):
			default:
				b.WriteRune(r)
			}
		}
	}

	if styled {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

// SanitizeTerminalPlain is SanitizeTerminal without color preservation: it
// also removes SGR sequences, returning plain text. Use it where the caller
// applies its own styling (e.g. lipgloss) and embedded colors or resets
// would fight with it, such as commit messages in the deploy view.
func SanitizeTerminalPlain(s string) string {
	return ansi.Strip(SanitizeTerminal(s))
}

// replaceInvalidUTF8 replaces each byte that is not part of a valid UTF-8
// sequence with U+FFFD, the Unicode replacement character. The result is
// always valid UTF-8, so downstream escape-sequence parsing never sees raw
// invalid bytes (which ansi.Strip would otherwise silently drop).
func replaceInvalidUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// isSGR reports whether seq is an SGR (Select Graphic Rendition) sequence,
// the escape code that sets colors and text styles: exactly
// ESC [ <digits ; :> m. Lookalikes with a private prefix or intermediate
// bytes (e.g. ESC [ > 4 ; 2 m, an xterm keyboard setting) are not SGR.
func isSGR(seq string) bool {
	params, ok := strings.CutPrefix(seq, "\x1b[")
	if !ok {
		return false
	}
	params, ok = strings.CutSuffix(params, "m")
	return ok && strings.Trim(params, "0123456789;:") == ""
}
