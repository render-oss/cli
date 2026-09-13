package service

import (
	"fmt"
	"os"

	envvar "github.com/render-oss/cli/pkg/client/envvar"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

func ResolveSecretFileInputs(secretFiles []string) ([]envvar.SecretFileInput, error) {
	if len(secretFiles) == 0 {
		return nil, nil
	}

	resolved := make([]envvar.SecretFileInput, 0, len(secretFiles))
	for _, secretFile := range secretFiles {
		input, err := readInput(secretFile)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, input)
	}

	return resolved, nil
}

func readInput(secretFile string) (envvar.SecretFileInput, error) {
	ref, err := servicetypes.ParseSecretFileRef(secretFile)
	if err != nil {
		return envvar.SecretFileInput{}, err
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return envvar.SecretFileInput{}, fmt.Errorf("failed to read --secret-file %q: %w", secretFile, err)
	}

	return envvar.SecretFileInput{
		Name:    ref.Name,
		Content: string(data),
	}, nil
}
