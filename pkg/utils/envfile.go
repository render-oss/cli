package utils

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/joho/godotenv"
)

// LoadEnvFiles reads KEY=VALUE pairs from each path and merges them into a
// single map. Later files override values from earlier files.
//
// When explicit is false, paths that do not exist are silently skipped. This
// supports auto-loading a default file (e.g. `.env`) without forcing it to
// exist. When explicit is true, every listed path must exist and be readable.
//
// The returned `loaded` slice contains the paths that were actually read, in
// the order they were processed (so callers can show "loaded from X, Y").
func LoadEnvFiles(paths []string, explicit bool) (vars map[string]string, loaded []string, err error) {
	vars = make(map[string]string)
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, statErr := os.Stat(path); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) && !explicit {
				continue
			}
			return nil, nil, fmt.Errorf("env file %s: %w", path, statErr)
		}
		values, readErr := godotenv.Read(path)
		if readErr != nil {
			return nil, nil, fmt.Errorf("failed to load env file %s: %w", path, readErr)
		}
		for k, v := range values {
			vars[k] = v
		}
		loaded = append(loaded, path)
	}
	return vars, loaded, nil
}

// EnvMapToKVStrings converts an env-var map to a deterministically sorted
// slice of "KEY=VALUE" strings, suitable for exec.Cmd.Env.
func EnvMapToKVStrings(vars map[string]string) []string {
	if len(vars) == 0 {
		return nil
	}
	out := make([]string, 0, len(vars))
	for k, v := range vars {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(out)
	return out
}
