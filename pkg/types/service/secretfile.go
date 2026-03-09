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

	name = strings.TrimSpace(name)
	localPath = strings.TrimSpace(localPath)
	if name == "" || localPath == "" {
		return SecretFileRef{}, fmt.Errorf("invalid --secret-file %q: expected NAME:LOCAL_PATH", raw)
	}

	return SecretFileRef{
		Name: name,
		Path: localPath,
	}, nil
}
