package service

import (
	"fmt"
	"os"

	"github.com/render-oss/cli/v2/pkg/client"
	servicetypes "github.com/render-oss/cli/v2/pkg/types/service"
)

func ResolveSecretFileInputs(secretFiles []string) ([]client.SecretFileInput, error) {
	if len(secretFiles) == 0 {
		return nil, nil
	}

	resolved := make([]client.SecretFileInput, 0, len(secretFiles))
	for _, secretFile := range secretFiles {
		input, err := readInput(secretFile)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, input)
	}

	return resolved, nil
}

func readInput(secretFile string) (client.SecretFileInput, error) {
	ref, err := servicetypes.ParseSecretFileRef(secretFile)
	if err != nil {
		return client.SecretFileInput{}, err
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return client.SecretFileInput{}, fmt.Errorf("failed to read --secret-file %q: %w", secretFile, err)
	}

	return client.SecretFileInput{
		Name:    ref.Name,
		Content: string(data),
	}, nil
}
