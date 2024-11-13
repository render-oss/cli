package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/client/devicegrant"
	"github.com/renderinc/render-cli/pkg/client/version"
	"github.com/renderinc/render-cli/pkg/config"
	renderstyle "github.com/renderinc/render-cli/pkg/style"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Render using the dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogin(cmd.Context())
	},
	GroupID: GroupAuth.ID,
}

func runLogin(ctx context.Context) error {
	c := devicegrant.NewClient(cfg.GetHost())
	vc := version.NewClient(cfg.RepoURL)

	dg, err := c.CreateGrant(ctx)
	if err != nil {
		return err
	}

	u, err := dashboardAuthURL(dg)
	if err != nil {
		return err
	}

	fmt.Printf("Complete the login via the dashboard. Launching browser to:\n\n\t%s\n\n", u)
	err = openBrowser(u.String())
	if err != nil {
		return err
	}
	fmt.Printf("Waiting for login to complete...\n\n")

	token, err := pollForToken(ctx, c, dg)
	if err != nil {
		return err
	}

	err = config.SetAPIConfig(cfg.GetHost(), token)
	if err != nil {
		return err
	}
	fmt.Println("Success! You are now authenticated.")

	newVersion, err := vc.NewVersionAvailable()
	if err == nil && newVersion != "" {
		fmt.Printf("\n%s\n\n", lipgloss.NewStyle().Foreground(renderstyle.ColorWarning).
			Render(fmt.Sprintf("render v%s is available. Current version is %s.\nInstallation instructions can be found at: %s", newVersion, cfg.Version, cfg.InstallationInstructionsURL)))
	}

	return nil
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
