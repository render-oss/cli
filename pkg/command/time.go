package command

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var relativeRegex = regexp.MustCompile(`^(\d+)([smhd])$`)

var characterToDuration = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": time.Hour * 24,
}

func parseRelativeTime(now time.Time, str string) *time.Time {
	matches := relativeRegex.FindStringSubmatch(str)
	if len(matches) != 3 {
		return nil
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil
	}
	t := now.Add(-characterToDuration[matches[2]] * time.Duration(num))

	return &t
}

func ParseTime(now time.Time, str *string) (*time.Time, error) {
	if str == nil || *str == "" {
		return nil, nil
	}

	if t := parseRelativeTime(now, *str); t != nil {
		return t, nil
	}

	t, err := time.Parse(time.RFC3339, *str)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	return &t, nil
}
