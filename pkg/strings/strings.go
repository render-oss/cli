package strings

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

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
