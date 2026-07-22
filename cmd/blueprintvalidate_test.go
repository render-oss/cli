package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestBlueprintValidateKeepsFileCompletion(t *testing.T) {
	requireCompletionDirective(t, []string{"blueprints", "validate", ""}, cobra.ShellCompDirectiveDefault)
}
