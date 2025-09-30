package types

func IsNonZeroString(s *string) bool {
	return s != nil && *s != ""
}
