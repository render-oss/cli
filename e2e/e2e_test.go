package e2e

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/renderinc/cli/e2e/helpers"
	"github.com/stretchr/testify/require"
)

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
		require.NoError(t, err)

		foundLine := false

		scan := bufio.NewScanner(output)
		for scan.Scan() {
			line := scan.Text()
			if strings.Contains(line, "Test Service") {
				foundLine = true
				break
			}
		}
		require.True(t, foundLine)
	})

	t.Run("TestLogs", func(t *testing.T) {
		output := helpers.RunCommand([]string{"logs", "--resources=srv-csr4gfi3esus73c9no5g", "-o=json"})
		require.NoError(t, err)

		foundLine := false

		scan := bufio.NewScanner(output)
		for scan.Scan() {
			line := scan.Text()
			if strings.Contains(line, "this is a message") {
				foundLine = true
				break
			}
		}

		require.True(t, foundLine)
	})

	t.Run("TestDeploy", func(t *testing.T) {
		output := helpers.RunCommand([]string{"deploys", "create", "srv-csr4gfi3esus73c9no5g", "-o=json", "--confirm"})
		require.NoError(t, err)
		deployRegex := regexp.MustCompile(`"id": "dep-[\w-]*"`)

		foundLine := false

		scan := bufio.NewScanner(output)
		for scan.Scan() {
			line := scan.Text()
			if deployRegex.MatchString(line) {
				foundLine = true
				break
			}
		}

		require.True(t, foundLine)
	})
}
