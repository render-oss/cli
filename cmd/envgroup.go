package cmd

import (
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/spf13/cobra"
)

func newEnvGroupCmd(deps *dependencies.Dependencies) *cobra.Command {
	group := &cobra.Command{Use: "env-group", Aliases: []string{"env-groups"}, Short: "Manage environment group variables", GroupID: GroupManagement.ID}
	get := &cobra.Command{
		Use: "get <environmentGroupID>", Short: "Get environment group variables", Args: cobra.ExactArgs(1), SilenceUsage: true,
		Long:    "Get all variables in an environment group by ID. Output includes secret values.",
		Example: "  render env-group get evg-abc123 --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			command.DefaultFormatNonInteractive(cmd)
			_, err := command.NonInteractive(cmd, func() ([]client.EnvVar, error) {
				return deps.EnvGroupRepo().Get(cmd.Context(), args[0])
			}, func(vars []client.EnvVar) string {
				var out strings.Builder
				for _, v := range vars {
					fmt.Fprintf(&out, "%s=%s\n", v.Key, v.Value)
				}
				return out.String()
			})
			return err
		},
	}
	set := &cobra.Command{
		Use: "set <environmentGroupID> <KEY=value>", Short: "Set an environment group variable", Args: cobra.ExactArgs(2), SilenceUsage: true,
		Long:    "Add or update one variable in an environment group by ID. Values passed on the command line may be saved in shell history.",
		Example: "  render env-group set evg-abc123 LOG_LEVEL=debug",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value, ok := strings.Cut(args[1], "=")
			if !ok || key == "" {
				return fmt.Errorf("expected KEY=value with a non-empty key")
			}
			command.DefaultFormatNonInteractive(cmd)
			type result struct {
				Key string `json:"key"`
			}
			_, err := command.NonInteractive(cmd, func() (*result, error) {
				if err := deps.EnvGroupRepo().Set(cmd.Context(), args[0], key, value); err != nil {
					return nil, err
				}
				return &result{Key: key}, nil
			}, func(out *result) string { return fmt.Sprintf("Updated %s\n", out.Key) })
			return err
		},
	}
	group.AddCommand(get, set)
	return group
}
