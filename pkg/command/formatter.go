package command

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type Output string

const (
	Interactive Output = "interactive"
	JSON        Output = "json"
	YAML        Output = "yaml"
)

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

func CommandName(cmd *cobra.Command, args []string, flags map[string]string) string {
	var flagString string
	for k, v := range flags {
		flagString += fmt.Sprintf("--%s %s ", k, v)
	}
	return fmt.Sprintf("%s %s %s", cmd.CommandPath(), strings.Join(args, " "), flagString)
}
