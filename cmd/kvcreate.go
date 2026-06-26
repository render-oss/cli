package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Render Key Value instance",
		Args:  cobra.NoArgs,
		Long: `Create a new Render Key Value instance.

In interactive mode, a prompt guides you through each option one at a time.
In non-interactive mode (--output text/json/yaml), flags use defaults if not supplied.
Use --confirm to skip all prompts (including final confirmation) and create immediately.
Output will be human-readable; use --output json/yaml/text for machine-readable output.`,
		Example: `  # Interactive wizard (guided prompts for each option)
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

	cmd.Flags().String("name", "", "Set the Key Value instance name (generated if not provided)")
	cmd.Flags().String("workspace", "", "Set the workspace to create the Key Value in (ID or name). Defaults to the active workspace (set via 'render workspace set').")
	cmd.Flags().String("project", "", "Scope environment lookup to a project (ID or name, optional); if the project has exactly one environment it is used automatically.")
	cmd.Flags().String("environment", "", "Set the environment to create the Key Value in (ID or name, optional). Example: Production or evm-abc123def456")

	cmd.Flags().String("plan", "",
		"Set the plan to one of: free | starter | standard | pro | pro_plus. Custom enterprise plan names are also accepted.")

	regionFlag := command.NewEnumInput(types.RegionValues(), false)
	cmd.Flags().Var(regionFlag, "region", "Set the region: frankfurt | ohio | oregon | singapore | virginia")

	maxmemFlag := command.NewEnumInput(kvtypes.MemoryPolicyInputValues(), false)
	cmd.Flags().Var(maxmemFlag, "memory-policy",
		"Set the eviction policy used when the instance runs out of memory.\n"+
			"Accepts a friendly alias — cache (= allkeys_lru, for caching) or queue (= noeviction, for job queues) —\n"+
			"or any raw policy: noeviction | allkeys_lru | allkeys_lfu | allkeys_random | volatile_lru | volatile_lfu | volatile_random | volatile_ttl")

	cmd.Flags().StringArray("ip-allow-list", nil,
		"Restrict inbound traffic to specific IP ranges (format: cidr=<range>,description=<label>). Repeat the flag for multiple entries.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input kvtypes.KeyValueCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(cmd,
			func() (*keyvalue.KeyValueCreateOut, error) {
				resolved, err := deps.KeyValueService().Create(cmd.Context(), input)
				if err != nil {
					return nil, err
				}
				out := keyvalue.NewKeyValueOut(resolved)
				return &keyvalue.KeyValueCreateOut{Data: out}, nil
			},
			func(out *keyvalue.KeyValueCreateOut) string {
				return kvCreateSuccessMessage(&out.Data)
			},
		); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		if command.GetConfirmFromContext(cmd.Context()) {
			return runKVCreateAndPrint(cmd, deps, input)
		}

		kv, err := views.RunKeyValueCreate(cmd, &input)
		if err != nil {
			return err
		}
		if kv == nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Canceled.")
			return nil
		}
		out := keyvalue.NewKeyValueOut(&keyvalue.ResolvedKeyValue{KeyValue: kv})
		_, _ = fmt.Fprint(cmd.OutOrStdout(), kvCreateSuccessMessage(&out))
		return nil
	}

	return cmd
}

func runKVCreateAndPrint(cmd *cobra.Command, deps *dependencies.Dependencies, input kvtypes.KeyValueCreateInput) error {
	resolved, err := deps.KeyValueService().Create(cmd.Context(), input)
	if err != nil {
		return err
	}
	out := keyvalue.NewKeyValueOut(resolved)
	_, _ = fmt.Fprint(cmd.OutOrStdout(), kvCreateSuccessMessage(&out))
	return nil
}

func kvCreateSuccessMessage(kv *keyvalue.KeyValueOut) string {
	return fmt.Sprintf(
		"Created Render Key Value\n\n%s\n",
		text.KeyValueDetail(kv),
	)
}
