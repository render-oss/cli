package command

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func CommandName(cmd *cobra.Command, args []string, flags map[string]string) string {
	var flagString string
	for k, v := range flags {
		flagString += fmt.Sprintf("--%s %s ", k, v)
	}
	return fmt.Sprintf("%s %s %s", cmd.CommandPath(), strings.Join(args, " "), flagString)
}
