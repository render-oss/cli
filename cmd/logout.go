package cmd

import (
	"context"

	huhspinner "github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client/oauth"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
)

var logoutCmd = newLogoutCmd()

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out of Render",
		RunE: func(cmd *cobra.Command, args []string) error {
			hasEnvKey := cfg.GetAPIKey() != ""
			hasOAuth, err := config.HasOAuthConfig()
			if err != nil {
				return err
			}

			if !hasEnvKey && !hasOAuth {
				command.Println(cmd, "You are not currently logged in. Run `render login` to authenticate.")
				return nil
			}

			if hasEnvKey && !hasOAuth {
				command.Println(cmd, "You are authenticated via the RENDER_API_KEY environment variable.")
				command.Println(cmd, "This command cannot remove environment variable credentials.")
				command.Println(cmd, "To revoke access, delete the API key from your Render Dashboard.")
				return nil
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			var revokeErr error
			logoutAction := func(ctx context.Context) error {
				apiCfg, err := config.OAuthConfig()
				if err == nil {
					oauthClient := oauth.NewClient(apiCfg.Host)
					revokeErr = oauthClient.RevokeToken(ctx, apiCfg.Key)
				}

				return config.DeleteConfig()
			}

			if command.IsInteractive(ctx) {
				err = huhspinner.New().
					Title("Logging out...").
					Output(cmd.ErrOrStderr()).
					ActionWithErr(logoutAction).
					Run()
			} else {
				err = logoutAction(ctx)
			}
			if err != nil {
				return err
			}

			if revokeErr != nil {
				command.Println(cmd, "Warning: something went wrong revoking your CLI token. Your local credentials have been cleared, but you'll need to revoke your token in the Render dashboard: %s/settings#cli-tokens", config.DashboardURL())
				if hasEnvKey {
					command.Println(cmd, "Note: RENDER_API_KEY is still set in your environment.")
				}
				return nil
			}

			if hasEnvKey {
				command.Println(cmd, "OAuth credentials cleared. Note: RENDER_API_KEY is still set in your environment.")
				return nil
			}

			command.Println(cmd, "Successfully logged out of Render.")
			command.Println(cmd, "Run `render login` to log back in.")
			return nil
		},
		GroupID: GroupAuth.ID,
	}
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
