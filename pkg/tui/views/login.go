package views

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/render-oss/cli/pkg/client/oauth"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/client/version"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dashboard"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/user"
)

type notLoggedInMsg struct{}

func NonInteractiveLogin(cmd *cobra.Command) error {
	dc := oauth.NewClient(cfg.GetHost())
	vc := version.NewClient(cfg.RepoURL)

	alreadyLoggedIn := isAlreadyLoggedIn(cmd.Context())
	if alreadyLoggedIn {
		command.Println(cmd, "Success: CLI is already authenticated.")
		return nil
	}

	err := login(cmd, dc, dashboard.Open)
	if err != nil {
		return err
	}

	command.Println(cmd, "Login successful! CLI token saved.")

	newVersion, err := vc.NewVersionAvailable()
	if err == nil && newVersion != "" {
		_, _ = cmd.ErrOrStderr().Write([]byte(fmt.Sprintf("\n%s\n\n", lipgloss.NewStyle().Foreground(renderstyle.ColorWarning).
			Render(fmt.Sprintf("render v%s is available. Current version is %s.\nInstallation instructions: %s", newVersion, cfg.Version, cfg.InstallationInstructionsURL)))))
	}

	return nil
}

func login(cmd *cobra.Command, c *oauth.Client, openBrowser func(url string) error) error {
	dg, err := c.CreateGrant(cmd.Context())
	if err != nil {
		return err
	}

	u, err := dashboardAuthURL(dg)
	if err != nil {
		return err
	}

	command.Println(cmd, "Complete login in the Render Dashboard with code: %s\n\nOpening your browser to:\n\n\t%s\n\n", dg.UserCode, u)
	if err := openBrowser(u.String()); err != nil {
		command.Println(cmd, "Could not open your browser automatically. Open the URL above to continue.\n\n")
	}
	command.Println(cmd, "Waiting for login...\n\n")

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

	dashURL           string
	browserOpenFailed bool
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
	dashURL           string
	deviceGrant       *oauth.DeviceGrant
	browserOpenFailed bool
}

type loginCompleteMsg struct{}

func startLogin(ctx context.Context, dc *oauth.Client, openBrowser func(url string) error) tea.Cmd {
	return func() tea.Msg {
		dg, err := dc.CreateGrant(ctx)
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		u, err := dashboardAuthURL(dg)
		if err != nil {
			return tui.ErrorMsg{Err: err}
		}

		// Opening the browser is best-effort: the view shows the URL, so login
		// can still complete when no browser can be launched.
		dashURL := u.String()
		openErr := openBrowser(dashURL)

		return loginStartedMsg{
			dashURL:           dashURL,
			deviceGrant:       dg,
			browserOpenFailed: openErr != nil,
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
	return tea.Sequence(
		func() tea.Msg { return tui.LoadingDataMsg{} },
		func() tea.Msg {
			if isAlreadyLoggedIn(l.ctx) {
				return tui.DoneMsg{}
			} else {
				return notLoggedInMsg{}
			}
		},
	)
}

func (l *LoginView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case notLoggedInMsg:
		return l, tea.Sequence(func() tea.Msg {
			return tui.DoneLoadingDataMsg{}
		}, startLogin(l.ctx, l.dc, dashboard.Open))
	case loginStartedMsg:
		l.dashURL = msg.dashURL
		l.browserOpenFailed = msg.browserOpenFailed
		return l, tea.Batch(func() tea.Msg {
			return tui.LoadingDataMsg{
				Cmd: tea.Sequence(
					pollForLogin(l.ctx, l.dc, msg),
					func() tea.Msg {
						return tui.DoneLoadingDataMsg{}
					},
				),
				LoadingMsgTmpl: loginPrompt(msg.dashURL, msg.browserOpenFailed) + "%sWaiting for login...\n",
			}
		})
	case loginCompleteMsg:
		return l, nil
	}
	return l, nil
}

func (l *LoginView) View() string {
	return loginPrompt(l.dashURL, l.browserOpenFailed) + "Waiting for login...\n"
}

func loginPrompt(dashURL string, browserOpenFailed bool) string {
	if browserOpenFailed {
		return fmt.Sprintf("Complete login in the Render Dashboard. Could not open your browser automatically; open this URL to continue:\n\n\t%s\n\n", dashURL)
	}
	return fmt.Sprintf("Complete login in the Render Dashboard. Opening your browser to:\n\n\t%s\n\n", dashURL)
}

func isAlreadyLoggedIn(ctx context.Context) bool {
	if cfg.GetAPIKey() != "" {
		return true
	}

	c, err := client.NewDefaultClient()
	if err != nil {
		return false
	}

	currentUser, err := user.NewRepo(c).CurrentUser(ctx)
	return err == nil && currentUser != nil
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
