package cmd

import (
	"context"

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

			apiCfg, err := config.OAuthConfig()
			if err == nil {
				ctx := cmd.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				oauthClient := oauth.NewClient(apiCfg.Host)
				if revokeErr := oauthClient.RevokeToken(ctx, apiCfg.Key); revokeErr != nil {
					command.Println(cmd, "Warning: something went wrong revoking your CLI token. Your local credentials will be cleared, but you'll need to revoke your token in the Render dashboard: %s/settings#cli-tokens", config.DashboardURL())
				}
			}

			if err := config.DeleteConfig(); err != nil {
				return err
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
