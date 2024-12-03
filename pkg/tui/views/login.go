package views

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renderinc/cli/pkg/client/oauth"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/cfg"
	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/client/version"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/config"
	"github.com/renderinc/cli/pkg/dashboard"
	renderstyle "github.com/renderinc/cli/pkg/style"
	"github.com/renderinc/cli/pkg/tui"
)

func NonInteractiveLogin(cmd *cobra.Command) error {
	dc := oauth.NewClient(cfg.GetHost())
	vc := version.NewClient(cfg.RepoURL)

	alreadyLoggedIn := isAlreadyLoggedIn(cmd.Context())
	if alreadyLoggedIn {
		command.Println(cmd, "Success! You are authenticated.")
		return nil
	}

	err := login(cmd, dc)
	if err != nil {
		return err
	}

	command.Println(cmd, "Success! You are now authenticated.")

	newVersion, err := vc.NewVersionAvailable()
	if err == nil && newVersion != "" {
		_, _ = cmd.ErrOrStderr().Write([]byte(fmt.Sprintf("\n%s\n\n", lipgloss.NewStyle().Foreground(renderstyle.ColorWarning).
			Render(fmt.Sprintf("render v%s is available. Current version is %s.\nInstallation instructions can be found at: %s", newVersion, cfg.Version, cfg.InstallationInstructionsURL)))))
	}

	return nil
}

func login(cmd *cobra.Command, c *oauth.Client) error {
	dg, err := c.CreateGrant(cmd.Context())
	if err != nil {
		return err
	}

	u, err := dashboardAuthURL(dg)
	if err != nil {
		return err
	}

	command.Println(cmd, "Complete the login via the dashboard. Launching browser to:\n\n\t%s\n\n", u)
	err = dashboard.Open(u.String())
	if err != nil {
		return err
	}
	command.Println(cmd, "Waiting for login to complete...\n\n")

	token, err := pollForToken(cmd.Context(), c, dg)
	if err != nil {
		return err
	}

	apiCfg := configForToken(token)
	return config.SetAPIConfig(apiCfg)
}

type LoginView struct {
	ctx context.Context

	dc *oauth.Client
	vc *version.Client

	dashURL string
}

func NewLoginView(ctx context.Context) *LoginView {
	dc := oauth.NewClient(cfg.GetHost())
	vc := version.NewClient(cfg.RepoURL)

	return &LoginView{
		ctx: ctx,
		dc:  dc,
		vc:  vc,
	}
}

type loginStartedMsg struct {
	dashURL     string
	deviceGrant *oauth.DeviceGrant
}

type loginCompleteMsg struct{}

func startLogin(ctx context.Context, dc *oauth.Client) tea.Cmd {
	return func() tea.Msg {
		dg, err := dc.CreateGrant(ctx)
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		u, err := dashboardAuthURL(dg)
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		err = dashboard.Open(u.String())
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		return loginStartedMsg{
			dashURL:     u.String(),
			deviceGrant: dg,
		}
	}
}

func pollForLogin(ctx context.Context, dc *oauth.Client, msg loginStartedMsg) tea.Cmd {
	return func() tea.Msg {
		token, err := pollForToken(ctx, dc, msg.deviceGrant)
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		apiCfg := configForToken(token)
		err = config.SetAPIConfig(apiCfg)
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		return tui.DoneMsg{Message: "Success! You are authenticated."}
	}
}

func (l *LoginView) Init() tea.Cmd {
	alreadyLoggedIn := isAlreadyLoggedIn(l.ctx)
	if alreadyLoggedIn {
		return nil
	}

	return startLogin(l.ctx, l.dc)
}

func (l *LoginView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loginStartedMsg:
		l.dashURL = msg.dashURL
		return l, tea.Batch(func() tea.Msg {
			return tui.LoadingDataMsg{
				Cmd: tea.Sequence(
					pollForLogin(l.ctx, l.dc, msg),
					func() tea.Msg {
						return tui.DoneLoadingDataMsg{}
					},
				),
				LoadingMsgTmpl: fmt.Sprintf("Complete the login via the dashboard. Launching browser to:\n\n\t%s\n\n%%sWaiting for login...\n", l.dashURL),
			}
		})
	case loginCompleteMsg:
		return l, nil
	}
	return l, nil
}

func (l *LoginView) View() string {
	return fmt.Sprintf("Complete the login via the dashboard. Launching browser to:\n\n\t%s\n\nWaiting for login...\n", l.dashURL)
}

func isAlreadyLoggedIn(ctx context.Context) bool {
	if cfg.GetAPIKey() != "" {
		return true
	}

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

func dashboardAuthURL(dg *oauth.DeviceGrant) (*url.URL, error) {
	u, err := url.Parse(dg.VerificationUriComplete)
	if err != nil {
		return nil, err
	}

	err = config.SetDashboardURL(dg.VerificationUri)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func pollForToken(ctx context.Context, c *oauth.Client, dg *oauth.DeviceGrant) (*oauth.DeviceToken, error) {
	timeout := time.NewTimer(time.Duration(dg.ExpiresIn) * time.Second)
	interval := time.NewTicker(time.Duration(dg.Interval) * time.Second)

	for {
		select {
		case <-timeout.C:
			return nil, errors.New("timed out")
		case <-interval.C:
			token, err := c.GetDeviceTokenResponse(ctx, dg)
			if errors.Is(err, oauth.ErrAuthorizationPending) {
				continue
			}
			if err != nil {
				return nil, err
			}

			return token, nil
		}
	}
}

func configForToken(token *oauth.DeviceToken) config.APIConfig {
	return config.APIConfig{
		Host:         cfg.GetHost(),
		Key:          token.AccessToken,
		ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).Unix(),
		RefreshToken: token.RefreshToken,
	}
}
