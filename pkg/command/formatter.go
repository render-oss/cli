package command

import (
	"fmt"

	"github.com/spf13/cobra"
)

type Output string

const (
	Interactive Output = "interactive"
	JSON        Output = "json"
	YAML        Output = "yaml"
)

func (o *Output) Interactive() bool {
	return o == nil || *o == Interactive
}

func StringToOutput(s string) (Output, error) {
	switch s {
	case "json":
		return JSON, nil
	case "yaml":
		return YAML, nil
	case "interactive":
		return Interactive, nil
	default:
		return "", fmt.Errorf("invalid output format: %s", s)
	}
}

func CommandName(cmd *cobra.Command, v any) (string, error) {
	inputString, err := InputToString(v)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", cmd.CommandPath(), inputString), nil
}
