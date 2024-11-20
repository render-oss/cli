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
	TEXT        Output = "text"
)

func (o *Output) Interactive() bool {
	return o == nil || *o == Interactive
}

func StringToOutput(s string) (Output, error) {
	switch strings.ToLower(s) {
	case "json":
		return JSON, nil
	case "yaml":
		return YAML, nil
	case "text":
		return TEXT, nil
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

func Println(cmd *cobra.Command, format string, a ...any) {
	_, err := cmd.OutOrStdout().Write([]byte(fmt.Sprintf(format, a...) + "\n"))
	if err != nil {
		panic(err)
	}
}
