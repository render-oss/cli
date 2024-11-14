package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/cfg"
	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/client/devicegrant"
	"github.com/renderinc/cli/pkg/client/version"
	"github.com/renderinc/cli/pkg/config"
	renderstyle "github.com/renderinc/cli/pkg/style"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Render using the dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogin(cmd.Context(), cmd.OutOrStdout())
	},
	GroupID: GroupAuth.ID,
}

func runLogin(ctx context.Context, out io.Writer) error {
	dc := devicegrant.NewClient(cfg.GetHost())
	vc := version.NewClient(cfg.RepoURL)

	alreadyLoggedIn := isAlreadyLoggedIn(ctx)
	if alreadyLoggedIn {
		fmt.Println("Success! You are authenticated.")
		return nil
	}

	err := login(ctx, dc, out)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(out, "Success! You are now authenticated.")

	newVersion, err := vc.NewVersionAvailable()
	if err == nil && newVersion != "" {
		_, _ = fmt.Fprintf(out, "\n%s\n\n", lipgloss.NewStyle().Foreground(renderstyle.ColorWarning).
			Render(fmt.Sprintf("render v%s is available. Current version is %s.\nInstallation instructions can be found at: %s", newVersion, cfg.Version, cfg.InstallationInstructionsURL)))
	}

	return nil
}

func isAlreadyLoggedIn(ctx context.Context) bool {
	c, err := client.NewDefaultClient()
	if err != nil {
		return false
	}

	workspace, err := config.WorkspaceID()
	if err != nil {
		return false
	}

	resp, err := c.RetrieveOwner(ctx, workspace)
	return err == nil && resp.StatusCode == http.StatusOK
}

func login(ctx context.Context, c *devicegrant.Client, out io.Writer) error {
	dg, err := c.CreateGrant(ctx)
	if err != nil {
		return err
	}

	u, err := dashboardAuthURL(dg)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "Complete the login via the dashboard. Launching browser to:\n\n\t%s\n\n", u)
	err = openBrowser(u.String())
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Waiting for login to complete...\n\n")

	token, err := pollForToken(ctx, c, dg)
	if err != nil {
		return err
	}

	return config.SetAPIConfig(cfg.GetHost(), token)
}

func dashboardAuthURL(dg *devicegrant.DeviceGrant) (*url.URL, error) {
	u, err := url.Parse(dg.VerificationUri)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func pollForToken(ctx context.Context, c *devicegrant.Client, dg *devicegrant.DeviceGrant) (string, error) {
	timeout := time.NewTimer(time.Duration(dg.ExpiresIn) * time.Second)
	interval := time.NewTicker(time.Duration(dg.Interval) * time.Second)

	for {
		select {
		case <-timeout.C:
			return "", errors.New("timed out")
		case <-interval.C:
			token, err := c.GetDeviceToken(ctx, dg)
			if errors.Is(err, devicegrant.ErrAuthorizationPending) {
				continue
			}
			if err != nil {
				return "", err
			}

			return token, nil
		}
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return nil
	}
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
