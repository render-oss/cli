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

// ParseOptionalString parses a possibly-empty string-like pointer with the provided parse function.
// It returns nil when value is nil or whitespace-only.
func ParseOptionalString[S ~string, T any](value *S, parse func(string) (T, error)) (*T, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(string(*value))
	if normalized == "" {
		return nil, nil
	}

	parsed, err := parse(normalized)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func OptionalAlias[T ~string](value *T) *T {
	parsed, err := ParseOptionalString(value, func(raw string) (T, error) {
		return T(raw), nil
	})
	if err != nil {
		return nil
	}
	return parsed
}
