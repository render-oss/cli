package command_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

func TestPrintDataEmptyList(t *testing.T) {
	for _, tc := range []struct {
		format command.Output
		want   string
	}{
		{command.JSON, "[]"},
		{command.YAML, "[]\n"},
	} {
		t.Run(string(tc.format), func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetContext(command.SetFormatInContext(context.Background(), &tc.format))
			var out bytes.Buffer
			cmd.SetOut(&out)

			var data []string
			_, err := command.PrintData(cmd, data, func([]string) string { return "" })
			require.NoError(t, err)
			require.Equal(t, tc.want, out.String())
		})
	}
}
