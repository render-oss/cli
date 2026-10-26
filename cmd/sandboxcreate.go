package cmd

import (
	"fmt"
	"regexp"
	"slices"
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
	Plan             string   `cli:"plan"`
	Region           string   `cli:"region"`
	Timeout          int      `cli:"timeout"`
	IdleTimeout      int      `cli:"idle-timeout"`
	NetworkPolicy    string   `cli:"network-policy"`
	AllowEgressHTTPS []string `cli:"allow-egress-https"`
	EnvVars          []string `cli:"env-var"`
	EnvFiles         []string `cli:"env-file"`
	SnapshotID       string   `cli:"snapshot-id"`
	From             string   `cli:"from"`
	Build            string   `cli:"build"`
	Wait             bool     `cli:"wait"`
}

func (i *SandboxCreateInput) Validate(_ bool) error {
	if i.From != "" && i.SnapshotID != "" {
		return fmt.Errorf("--from and --snapshot-id cannot be used together")
	}
	if i.Build != "" && (i.From != "" || i.SnapshotID != "") {
		return fmt.Errorf("--build cannot be used with --from or --snapshot-id")
	}
	if i.NetworkPolicy != "" && !sandboxclient.SandboxNetworkPolicyType(i.NetworkPolicy).Valid() {
		return fmt.Errorf("invalid network policy %q: use %s", i.NetworkPolicy, strings.Join(sandboxNetworkPolicyNames(), ", "))
	}
	switch policy := sandboxclient.SandboxNetworkPolicyType(i.resolvedNetworkPolicy()); {
	case policy == sandboxclient.AllowList && len(i.egressRules()) == 0:
		return fmt.Errorf("--network-policy=%s requires at least one --allow-egress-https", sandboxclient.AllowList)
	case policy != sandboxclient.AllowList && len(i.egressRules()) > 0:
		return fmt.Errorf("--allow-egress-https requires --network-policy=%s, got %s", sandboxclient.AllowList, policy)
	}
	// Only positive timeouts reach the request body, so without this a negative
	// value would silently get the default rather than the lifetime asked for.
	if i.Timeout < 0 {
		return fmt.Errorf("invalid timeout %d: use a positive number of seconds, or 0 for the default", i.Timeout)
	}
	if i.IdleTimeout != 0 && (i.IdleTimeout < 300 || i.IdleTimeout > 86400) {
		return fmt.Errorf("invalid idle timeout %d: use 0 to disable, or 300 to 86400 seconds", i.IdleTimeout)
	}
	if _, err := resolveSandboxEnv(nil, i.EnvVars); err != nil {
		return err
	}
	return nil
}

// resolvedNetworkPolicy implies allow-list when --allow-egress-* rules are
// given without --network-policy, since rules only apply under an allow list.
func (i *SandboxCreateInput) resolvedNetworkPolicy() string {
	if i.NetworkPolicy == "" && len(i.egressRules()) > 0 {
		return string(sandboxclient.AllowList)
	}
	return i.NetworkPolicy
}

// egressRules collects the --allow-egress-<protocol> flags into allow-list
// rules. Each supported protocol gets its own flag.
func (i *SandboxCreateInput) egressRules() []sandboxclient.SandboxEgressRule {
	var rules []sandboxclient.SandboxEgressRule
	for _, domain := range i.AllowEgressHTTPS {
		rules = append(rules, sandboxclient.SandboxEgressRule{Domain: domain, Protocol: new(sandboxclient.Https)})
	}
	return rules
}

var sandboxSnapshotIDPattern = regexp.MustCompile(`^snp-[0-9a-v]{20}$`)

func resolveSandboxSource(from string, fromSet bool, snapshotID string) (string, *string) {
	if !fromSet {
		return snapshotID, nil
	}
	if sandboxSnapshotIDPattern.MatchString(from) {
		return from, nil
	}
	return "", &from
}

var sandboxBuildIDPattern = regexp.MustCompile(`^sbb-[0-9a-v]{20}$`)

func resolveSandboxBuild(build string, buildSet bool) (string, *string) {
	if !buildSet {
		return "", nil
	}
	if sandboxBuildIDPattern.MatchString(build) {
		return build, nil
	}
	return "", &build
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

var deprecatedSandboxPlans = []sandboxclient.SandboxPlan{
	sandboxclient.Starter,
	sandboxclient.Standard,
	sandboxclient.Pro,
}

// Help text only; the API validates --plan.
func sandboxPlanNames() []string {
	var out []string
	for _, p := range sandboxclient.SandboxPlanValues() {
		if !slices.Contains(deprecatedSandboxPlans, p) {
			out = append(out, string(p))
		}
	}
	return out
}

func sandboxNetworkPolicyNames() []string {
	policies := sandboxclient.SandboxNetworkPolicyTypeValues()
	out := make([]string, len(policies))
	for i, p := range policies {
		out[i] = string(p)
	}
	return out
}

func newSandboxCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new sandbox",
		Long: `Create a new sandbox in the current workspace.

Examples:
  render ea sandboxes create
  render ea sandboxes create --plan=2c-4g --region=oregon
  render ea sandboxes create --timeout=3600
  render ea sandboxes create --idle-timeout=1800
  render ea sandboxes create --idle-timeout=0
  render ea sandboxes create --network-policy=deny-all
  render ea sandboxes create --allow-egress-https api.github.com --allow-egress-https '*.pypi.org'
  render ea sandboxes create --env-var FOO=bar --env-var BAZ=qux
  render ea sandboxes create --env-file .env.production --env-var LOG_LEVEL=debug
  render ea sandboxes create --from gold
  render ea sandboxes create --from snp-cph1rs3idesc73a2b2mg
  render ea sandboxes create --build python
  render ea sandboxes create --build sbb-cph1rs3idesc73a2b2mg
  render ea sandboxes create --from gold --wait
`,
	}

	cmd.Flags().String("plan", "", "Compute plan: "+strings.Join(sandboxPlanNames(), ", "))
	cmd.Flags().String("region", "", "Region to run the sandbox in")
	cmd.Flags().Int("timeout", 0, "Maximum sandbox lifetime in seconds, up to 2592000 (30 days), or 86400 (24 hours) on Hobby workspaces. 0 uses the default of 86400 (24 hours)")
	cmd.Flags().Int("idle-timeout", 900, "Seconds of inactivity before suspension. 0 disables automatic idle suspension")
	cmd.Flags().String("network-policy", "", "Outbound network policy: "+strings.Join(sandboxNetworkPolicyNames(), ", "))
	setFlagPlaceholder(cmd.Flags(), "network-policy", "NETWORK_POLICY")
	cmd.Flags().StringArray("allow-egress-https", nil, "Allow outbound HTTPS to a domain, e.g. api.example.com or *.example.com. Repeat for multiple domains. Implies --network-policy=allow-list")
	setFlagPlaceholder(cmd.Flags(), "allow-egress-https", "DOMAIN")
	cmd.Flags().StringArray("env-var", nil, "Set environment variables in KEY=VALUE format (can be specified multiple times). Inline values override values loaded from --env-file.")
	cmd.Flags().StringSlice("env-file", nil, "Path to an env file to load. Repeat to load multiple files (later files override earlier ones). Every listed file must exist.")
	setFlagPlaceholder(cmd.Flags(), "env-var", "KEY_VALUE")
	setFlagPlaceholder(cmd.Flags(), "env-file", "PATH")
	cmd.Flags().String("snapshot-id", "", "Start from this snapshot instead of the base image. The snapshot must be available and in the same sandbox group. A runtime snapshot requires --plan to match its plan.")
	setFlagPlaceholder(cmd.Flags(), "snapshot-id", "SNAPSHOT_ID")
	cmd.Flags().String("from", "", "Start from a snapshot ID or name")
	setFlagPlaceholder(cmd.Flags(), "from", "SNAPSHOT_ID_OR_NAME")
	cmd.MarkFlagsMutuallyExclusive("from", "snapshot-id")
	cmd.Flags().String("build", "", "Start from a build's image instead of the base image, by build ID or name. A name uses the newest succeeded build with that name in the sandbox group")
	setFlagPlaceholder(cmd.Flags(), "build", "BUILD_ID_OR_NAME")
	cmd.MarkFlagsMutuallyExclusive("build", "from")
	cmd.MarkFlagsMutuallyExclusive("build", "snapshot-id")
	cmd.Flags().Bool("wait", false, fmt.Sprintf("Wait until the sandbox is running, up to %d seconds, instead of returning while it's still creating", int(sandbox.WaitTimeout.Seconds())))

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
			// Create and --wait both report "creating"; print it once.
			var printed sandboxclient.SandboxStatus
			onEvent = func(sb *sandboxclient.Sandbox) {
				if sb.Status == printed {
					return
				}
				printed = sb.Status
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %s %s\n", sb.Id, sb.Status)
			}
		}

		env, err := resolveSandboxEnv(input.EnvFiles, input.EnvVars)
		if err != nil {
			return err
		}
		snapshotID, snapshotName := resolveSandboxSource(input.From, cmd.Flags().Changed("from"), input.SnapshotID)
		buildID, buildName := resolveSandboxBuild(input.Build, cmd.Flags().Changed("build"))
		var idleTimeout *int
		if cmd.Flags().Changed("idle-timeout") {
			idleTimeout = &input.IdleTimeout
		}

		// A failed wait still prints the sandbox, so callers get its ID.
		var waitErr error
		_, err = command.NonInteractive(cmd, func() (*sandboxclient.Sandbox, error) {
			sb, err := deps.SandboxService().Create(cmd.Context(), sandbox.CreateInput{
				Plan:          input.Plan,
				Region:        input.Region,
				Timeout:       input.Timeout,
				IdleTimeout:   idleTimeout,
				NetworkPolicy: input.resolvedNetworkPolicy(),
				EgressRules:   input.egressRules(),
				Env:           env,
				SnapshotID:    snapshotID,
				SnapshotName:  snapshotName,
				BuildID:       buildID,
				BuildName:     buildName,
			}, onEvent)
			if err != nil || !input.Wait {
				return sb, err
			}
			ctx, stop := waitContext(cmd.Context())
			defer stop()
			var waited *sandboxclient.Sandbox
			waited, waitErr = deps.SandboxService().WaitUntilRunning(ctx, sb.Id, onEvent)
			if waited != nil {
				sb = waited
			}
			return sb, nil
		}, text.SandboxDetail)
		if err != nil {
			return err
		}
		return waitErr
	}

	return cmd
}
