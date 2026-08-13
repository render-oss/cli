package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/types"
	"github.com/render-oss/cli/pkg/utils"
)

type SandboxCreateInput struct {
	Plan          string `cli:"plan"`
	Region        string `cli:"region"`
	Timeout       int      `cli:"timeout"`
	NetworkPolicy string   `cli:"network-policy"`
	EnvVars       []string `cli:"env-var"`
	EnvFiles      []string `cli:"env-file"`
}

func (i *SandboxCreateInput) Validate(_ bool) error {
	if i.Plan != "" && !sandboxclient.SandboxPlan(i.Plan).Valid() {
		return fmt.Errorf("invalid plan %q: use %s", i.Plan, strings.Join(sandboxPlanNames(), ", "))
	}
	if i.NetworkPolicy != "" && !sandboxclient.SandboxNetworkPolicyDefault(i.NetworkPolicy).Valid() {
		return fmt.Errorf("invalid network policy %q: use %s", i.NetworkPolicy, strings.Join(sandboxNetworkPolicyNames(), ", "))
	}
	if _, err := resolveSandboxEnv(nil, i.EnvVars); err != nil {
		return err
	}
	return nil
}

func resolveSandboxEnv(files, pairs []string) (map[string]string, error) {
	fileVars, _, err := utils.LoadEnvFiles(files, true)
	if err != nil {
		return nil, err
	}
	raw := append(utils.EnvMapToKVStrings(fileVars), pairs...)
	if len(raw) == 0 {
		return nil, nil
	}
	env := make(map[string]string, len(raw))
	for _, pair := range raw {
		ev, err := types.ParseEnvVar(pair)
		if err != nil {
			return nil, err
		}
		env[ev.Key] = ev.Value
	}
	return env, nil
}

// sandboxPlanNames returns the valid sandbox plan names, sourced from the
// generated SandboxPlanValues() so help and error text stay in sync with the
// schema. SandboxPlan has no "custom" sentinel, so no filtering is needed.
func sandboxPlanNames() []string {
	plans := sandboxclient.SandboxPlanValues()
	out := make([]string, len(plans))
	for i, p := range plans {
		out[i] = string(p)
	}
	return out
}

func sandboxNetworkPolicyNames() []string {
	return []string{
		string(sandboxclient.AllowAll),
		string(sandboxclient.DenyAll),
	}
}

func newSandboxCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new sandbox",
		Long: `Create a new sandbox in the current workspace.

Examples:
  render ea sandboxes create
  render ea sandboxes create --plan=standard --region=oregon
  render ea sandboxes create --timeout=3600
  render ea sandboxes create --network-policy=deny-all
  render ea sandboxes create --env-var FOO=bar --env-var BAZ=qux
  render ea sandboxes create --env-file .env.production --env-var LOG_LEVEL=debug
`,
	}

	cmd.Flags().String("plan", "", "Compute plan: "+strings.Join(sandboxPlanNames(), ", "))
	cmd.Flags().String("region", "", "Region to run the sandbox in")
	cmd.Flags().Int("timeout", 0, "Maximum sandbox lifetime in seconds")
	cmd.Flags().String("network-policy", "", "Outbound network policy: "+strings.Join(sandboxNetworkPolicyNames(), ", "))
	setFlagPlaceholder(cmd.Flags(), "network-policy", "NETWORK_POLICY")
	cmd.Flags().StringArray("env-var", nil, "Set environment variables in KEY=VALUE format (can be specified multiple times). Inline values override values loaded from --env-file.")
	cmd.Flags().StringSlice("env-file", nil, "Path to an env file to load. Repeat to load multiple files (later files override earlier ones). Every listed file must exist.")
	setFlagPlaceholder(cmd.Flags(), "env-var", "KEY_VALUE")
	setFlagPlaceholder(cmd.Flags(), "env-file", "PATH")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		// Stream status updates to stderr only for text output, so JSON/YAML
		// consumers get a single clean payload on stdout.
		var onEvent func(*sandboxclient.Sandbox)
		if format := command.GetFormatFromContext(cmd.Context()); format != nil && *format == command.TEXT {
			onEvent = func(sb *sandboxclient.Sandbox) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %s %s\n", sb.Id, sb.Status)
			}
		}

		env, err := resolveSandboxEnv(input.EnvFiles, input.EnvVars)
		if err != nil {
			return err
		}

		_, err = command.NonInteractive(cmd, func() (*sandboxclient.Sandbox, error) {
			return deps.SandboxService().Create(cmd.Context(), sandbox.CreateInput{
				Plan:          input.Plan,
				Region:        input.Region,
				Timeout:       input.Timeout,
				NetworkPolicy: input.NetworkPolicy,
				Env:           env,
			}, onEvent)
		}, text.SandboxDetail)
		return err
	}

	return cmd
}
