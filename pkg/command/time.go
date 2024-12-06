package command

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var RFC3339RegexString = []string{
	`\d`, `\d`, `\d`, `\d`, `-`, `\d`, `\d`, `-`, `\d`, `\d`,
	`T`, `\d`, `\d`, `:`, `\d`, `\d`, `:`, `\d`, `\d`,
	`Z`, `\d`, `\d`, `:`, `\d`, `\d`,
}

func TimeSuggestion(str string) []string {
	var suggestion string
	if i, err := strconv.Atoi(str); err == nil && i <= 60 {
		suggestion = fmt.Sprintf("%dm", i)
	} else if re, err := regexp.Compile(strings.Join(RFC3339RegexString[:len(str)], "")); err == nil && re.MatchString(str) {
		suggestion = str + time.RFC3339[len(str):]
	}

	return []string{suggestion}
}

type TimeOrRelative struct {
	T        *time.Time
	Relative *string
}

func (t *TimeOrRelative) String() string {
	if t.Relative != nil {
		return *t.Relative
	}
	return t.T.Format(time.RFC3339)
}

var relativeRegex = regexp.MustCompile(`^(\d+)([smhd])$`)

var characterToDuration = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": time.Hour * 24,
}

func parseRelativeTime(now time.Time, str string) *TimeOrRelative {
	matches := relativeRegex.FindStringSubmatch(str)
	if len(matches) != 3 {
		return nil
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil
	}
	t := now.Add(-characterToDuration[matches[2]] * time.Duration(num))

	return &TimeOrRelative{T: &t, Relative: &str}
}

func ParseTime(now time.Time, str *string) (*TimeOrRelative, error) {
	if str == nil || *str == "" {
		return nil, nil
	}

	if t := parseRelativeTime(now, *str); t != nil {
		return t, nil
	}

	absoluteTime, err := time.Parse(time.RFC3339, *str)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp, time must either be relative (1m, 5h, etc) or in RFC3339 format: %s", time.RFC3339)
	}

	return &TimeOrRelative{T: &absoluteTime}, nil
}

const (
	TimeType = "time"
)

type CobraTime struct {
	t *TimeOrRelative
}

func NewTimeInput() *CobraTime {
	return &CobraTime{}
}

func (e *CobraTime) String() string {
	if e.t == nil {
		return ""
	}

	return e.t.String()
}

func (e *CobraTime) Set(v string) error {
	t, err := ParseTime(time.Now(), &v)
	if err != nil {
		return err
	}

	e.t = t
	return nil
}

func (e *CobraTime) Type() string {
	return TimeType
}

func (e *CobraTime) Get() *TimeOrRelative {
	return e.t
}
