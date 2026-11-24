package service

import (
	"fmt"
	"strings"
)

type SecretFileRef struct {
	Name string
	Path string
}

// ParseSecretFileRef parses NAME:LOCAL_PATH from --secret-file.
func ParseSecretFileRef(raw string) (SecretFileRef, error) {
	name, localPath, ok := strings.Cut(raw, ":")
	if !ok {
		return SecretFileRef{}, fmt.Errorf("invalid --secret-file %q: expected NAME:LOCAL_PATH", raw)
	}

	// The name is formatting, so surrounding whitespace is dropped. The path
	// names a file: whitespace there is part of the name the shell passed, and
	// trimming it would read a different file that happens to exist.
	name = strings.TrimSpace(name)
	if name == "" || strings.TrimSpace(localPath) == "" {
		return SecretFileRef{}, fmt.Errorf("invalid --secret-file %q: expected NAME:LOCAL_PATH", raw)
	}

	return SecretFileRef{
		Name: name,
		Path: localPath,
	}, nil
}
