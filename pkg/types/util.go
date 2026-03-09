package types

import "strings"

func IsNonZeroString(s *string) bool {
	return s != nil && *s != ""
}

func OptionalNonZeroString(value *string) *string {
	if !IsNonZeroString(value) {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func TrimmedNonEmpty(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

// ParseOptional parses a possibly-empty string pointer with the provided parse function.
// It returns nil when value is nil or whitespace-only.
func ParseOptional[T any](value *string, parse func(string) (T, error)) (*T, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	parsed, err := parse(*value)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
