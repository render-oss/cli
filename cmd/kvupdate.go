package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVUpdateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "update <keyValueID|keyValueName>",
		Short:        "Update a Key Value store instance",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Update an existing Key Value store instance on Render.

The positional argument is the target Key Value (ID red-... or name). At least
one mutating flag must be supplied. Use --name to rename the instance; the
positional argument always identifies the target and is never the new name.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Key Value ID instead (which works across workspaces). If a name matches more
than one instance, narrow the search with --environment <id|name>.

Environment, project, workspace, and region are immutable. A KV cannot be
moved between them; the --environment flag is for name disambiguation only.

The --ip-allow-list flag replaces the server-side list; pass it once per entry.
To remove all allow-list entries, pass --clear-ip-allow-list. The two flags are
mutually exclusive.`,
		Example: `  # Rename
  render ea kv update red-abc123def456ghi789jkl0 --name new-cache-name

  # Change plan
  render ea kv update my-cache --plan standard

  # Replace the IP allow-list (entire list, not append)
  render ea kv update my-cache \
    --ip-allow-list "cidr=203.0.113.5/32,description=office" \
    --ip-allow-list "cidr=10.0.0.0/8,description=internal"

  # Clear the IP allow-list
  render ea kv update my-cache --clear-ip-allow-list

  # Disambiguate a name that exists in multiple environments
  render ea kv update my-cache --environment production --memory-policy queue

  # JSON output
  render ea kv update red-abc123def456ghi789jkl0 --plan pro --output json`,
	}

	memoryPolicyDesc := `Controls what the instance does when it runs out of memory to store new data.
Shortcuts: cache (sets allkeys_lru, recommended for caching) | queue (sets noeviction, recommended for job queues).
Technical values: noeviction | allkeys_lru | allkeys_lfu | allkeys_random | volatile_lru | volatile_lfu | volatile_random | volatile_ttl`

	ipAllowListDesc := `Replace the IP allow-list with the supplied entries. Repeat the flag for multiple entries.
Format: cidr=<range>,description=<label>
Example: --ip-allow-list "cidr=203.0.113.5/32,description=office"`

	cmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows name lookup when the same Key Value name exists in multiple environments")

	cmd.Flags().String("name", "", "Rename the Key Value instance")
	cmd.Flags().String("plan", "",
		"Plan name. Examples: free | starter | standard | pro | pro_plus. Account-specific plan names are accepted")

	maxmemFlag := command.NewEnumInput(kvtypes.MemoryPolicyInputValues(), false)
	cmd.Flags().Var(maxmemFlag, "memory-policy", memoryPolicyDesc)

	cmd.Flags().StringArray("ip-allow-list", nil, ipAllowListDesc)
	cmd.Flags().Bool("clear-ip-allow-list", false,
		"Remove all IP allow-list entries. Mutually exclusive with --ip-allow-list")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// No interactive flow yet; collapse --output interactive onto text so
		// the standard NonInteractive path handles every format.
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueUpdateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd,
			func() (*keyvalue.KeyValueUpdateOut, error) {
				result, err := deps.KeyValueService().Update(cmd.Context(), input)
				if err != nil {
					return nil, err
				}
				out := keyvalue.NewKeyValueUpdateOut(result.Before, result.After)
				return &out, nil
			},
			func(out *keyvalue.KeyValueUpdateOut) string {
				return kvUpdateSuccessMessage(out)
			},
		)
		return err
	}

	return cmd
}

func kvUpdateSuccessMessage(out *keyvalue.KeyValueUpdateOut) string {
	details := "Full details:\n  " + strings.ReplaceAll(text.KeyValueDetail(&out.Data), "\n", "\n  ")
	diff := text.KeyValueUpdateDiff(out.Diff)
	if diff == "" {
		return fmt.Sprintf("No changes applied to Key Value\n\n%s\n", details)
	}
	return fmt.Sprintf("Updated Key Value\n\nChanges:\n%s\n\n%s\n", diff, details)
}
