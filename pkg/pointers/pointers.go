package pointers

import "time"

func From[T any](x T) *T {
	return &x
}

func FromArray[T any](x []T) *[]T {
	if len(x) == 0 {
		return nil
	}

	return &x
}

func ValueOrDefault[T any](x *T, def T) T {
	if x == nil {
		return def
	}
	return *x
}

// Equal reports whether two pointers are both nil or point to equal values.
// It compares the pointed-to values, not the pointer addresses. T must be
// comparable because Equal uses == after dereferencing non-nil pointers.
func Equal[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func PointerValueIfNotEmptyString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TimeValue(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
