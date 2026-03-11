package service

import (
	"fmt"
	"strings"
)

type PreviewsGeneration string

const (
	PreviewsGenerationAutomatic PreviewsGeneration = "automatic"
	PreviewsGenerationManual    PreviewsGeneration = "manual"
	PreviewsGenerationOff       PreviewsGeneration = "off"
)

var previewsGenerationValues = []PreviewsGeneration{
	PreviewsGenerationAutomatic,
	PreviewsGenerationManual,
	PreviewsGenerationOff,
}

func PreviewsGenerationValues() []string {
	values := make([]string, 0, len(previewsGenerationValues))
	for _, value := range previewsGenerationValues {
		values = append(values, string(value))
	}
	return values
}

func ParsePreviewsGeneration(value string) (PreviewsGeneration, error) {
	normalized := strings.TrimSpace(value)
	for _, pg := range previewsGenerationValues {
		if normalized == string(pg) {
			return pg, nil
		}
	}

	return "", fmt.Errorf("previews must be one of: %s", strings.Join(PreviewsGenerationValues(), ", "))
}
