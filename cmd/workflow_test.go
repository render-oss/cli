package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestWorkflowDevKeepsFileCompletion(t *testing.T) {
	requireCompletionDirective(t, []string{"workflows", "dev", ""}, cobra.ShellCompDirectiveDefault)
}
