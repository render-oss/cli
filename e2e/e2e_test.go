package e2e

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/render-oss/cli/cmd"
	"github.com/render-oss/cli/e2e/helpers"
	"github.com/stretchr/testify/require"
)

func outputHasLine(output io.Reader, line string) (string, bool) {
	totalOutput := ""
	scan := bufio.NewScanner(output)
	for scan.Scan() {
		totalOutput += scan.Text() + "\n"

		if strings.Contains(scan.Text(), line) {
			return totalOutput, true
		}
	}
	return totalOutput, false
}

func outputHasLineRegex(output io.Reader, regex *regexp.Regexp) (string, bool) {
	totalOutput := ""
	scan := bufio.NewScanner(output)
	for scan.Scan() {
		totalOutput += scan.Text() + "\n"

		if regex.MatchString(scan.Text()) {
			return totalOutput, true
		}
	}
	return totalOutput, false
}

func TestE2E(t *testing.T) {
	// We log in once at the beginning of the test suite
	// and use the same session for all tests
	f, err := os.CreateTemp("", "render-config")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	t.Setenv("RENDER_CLI_CONFIG_PATH", f.Name())

	err = helpers.Login()
	require.NoError(t, err)

	t.Run("TestServices", func(t *testing.T) {
		output := helpers.RunCommand([]string{"services", "-o=json"})

		totalOutput, foundLine := outputHasLine(output, "Test Service")
		require.True(t, foundLine, totalOutput)
	})

	t.Run("TestDeploy", func(t *testing.T) {
		output := helpers.RunCommand([]string{"deploys", "create", "srv-csr4gfi3esus73c9no5g", "-o=json", "--confirm"})
		deployRegex := regexp.MustCompile(`"id": "dep-[\w-]*"`)

		totalOutput, foundLine := outputHasLineRegex(output, deployRegex)

		require.True(t, foundLine, totalOutput)
	})

	t.Run("TestLogs", func(t *testing.T) {
		// We need to reset commands for logs so that it has access to the proper logged in client
		// Otherwise we end up reusing the non-logged in client that gets attached to the root command
		// when we try logging in above.
		cmd.RootCmd.ResetCommands()
		output := helpers.RunCommand([]string{"logs", "--resources=srv-csr4gfi3esus73c9no5g", "-o=json"})

		totalOutput, foundLine := outputHasLine(output, "this is a message")
		require.True(t, foundLine, totalOutput)
	})
}
