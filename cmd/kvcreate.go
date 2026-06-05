package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVCreateCmd(_ *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Key Value store instance",
		Args:  cobra.NoArgs,
		Long: `Create a new Key Value store instance on Render.

In interactive mode, a prompt guides you through each option one at a time.
In non-interactive mode (--output text/json/yaml), flags use defaults if not supplied.
Use --confirm to skip all prompts (including final confirmation) and create immediately.
Output will be human-readable; use --output json/yaml/text for machine-readable output.

Examples:
  # Interactive wizard (guided prompts for each option)
  render ea kv create

  # Specify all options; wizard still asks for confirmation before creating
  render ea kv create --name my-cache --plan starter --region oregon

  # Skip all prompts and create immediately (no confirmation)
  render ea kv create --name my-cache --plan free --confirm

  # Machine-readable output (non-interactive, no prompts)
  render ea kv create --name my-cache --plan starter --output json

  # With IP allow-listing (repeat the flag for multiple entries)
  render ea kv create --name my-cache \
    --ip-allow-list "cidr=203.0.113.5/32,description=office" \
    --ip-allow-list "cidr=10.0.0.0/8,description=internal"`,
	}

	cmd.Flags().String("name", "", "Key Value instance name (generated if not provided)")
	cmd.Flags().String("workspace", "", "Workspace ID or name. Defaults to the active workspace (set via 'render workspace set').")
	cmd.Flags().String("project", "", "Project ID or name (optional). Scopes environment lookup; if the project has exactly one environment it is used automatically.")
	cmd.Flags().String("environment", "", "Environment ID or name (optional). Example: Production or evm-abc123def456")

	cmd.Flags().String("plan", "",
		"Plan name. Examples: free | starter | standard | pro | pro_plus. Account-specific plan names are accepted.")

	regionFlag := command.NewEnumInput(types.RegionValues(), false)
	cmd.Flags().Var(regionFlag, "region", "Region: frankfurt | ohio | oregon | singapore | virginia")

	maxmemFlag := command.NewEnumInput(kvtypes.MemoryPolicyInputValues(), false)
	cmd.Flags().Var(maxmemFlag, "memory-policy",
		"Controls what the instance does when it runs out of memory to store new data.\n"+
			"Shortcuts: cache (sets allkeys_lru, recommended for caching) | queue (sets noeviction, recommended for job queues).\n"+
			"Technical values: noeviction | allkeys_lru | allkeys_lfu | allkeys_random | volatile_lru | volatile_lfu | volatile_random | volatile_ttl")

	cmd.Flags().StringArray("ip-allow-list", nil,
		"Restrict inbound traffic to specific IP ranges. Repeat the flag for multiple entries.\n"+
			"Format: cidr=<range>,description=<label>\n"+
			"Example: --ip-allow-list \"cidr=203.0.113.5/32,description=office\"")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input kvtypes.KeyValueCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(cmd,
			func() (*client.KeyValueDetail, error) {
				return keyvalue.Create(cmd.Context(), input)
			},
			func(kv *client.KeyValueDetail) string {
				return kvCreateSuccessMessage(kv)
			},
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		if command.GetConfirmFromContext(cmd.Context()) {
			return runKVCreateAndPrint(cmd, input)
		}

		kv, err := views.RunKeyValueCreate(cmd, &input)
		if err != nil {
			return err
		}
		if kv == nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Canceled.")
			return nil
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), kvCreateSuccessMessage(kv))
		return nil
	}

	return cmd
}

func runKVCreateAndPrint(cmd *cobra.Command, input kvtypes.KeyValueCreateInput) error {
	kv, err := keyvalue.Create(cmd.Context(), input)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), kvCreateSuccessMessage(kv))
	return nil
}

func kvCreateSuccessMessage(kv *client.KeyValueDetail) string {
	return fmt.Sprintf(
		"Created Key Value store\n\n%s\n",
		text.KeyValueDetail(kv),
	)
}
