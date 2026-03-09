package types

import (
	"fmt"
	"strings"
)

type EnvVar struct {
	Key   string
	Value string
}

// ParseEnvVar parses KEY=VALUE.
func ParseEnvVar(raw string) (EnvVar, error) {
	key, value, hasEquals := strings.Cut(raw, "=")
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if !hasEquals || key == "" {
		return EnvVar{}, fmt.Errorf("invalid --env-var %q: expected KEY=VALUE", raw)
	}

	return EnvVar{
		Key:   key,
		Value: value,
	}, nil
}
