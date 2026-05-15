package strings

import (
	"fmt"
	"strings"

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
