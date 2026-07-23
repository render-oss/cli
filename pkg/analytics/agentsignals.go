package analytics

import (
	"os"
	"slices"
	"strings"
)

// agentSignal is one supported agent environment variable and its activation
// rule.
type agentSignal struct {
	// envVar is the raw environment variable name emitted in agent_signals.
	envVar string
	// isActive receives the raw environment value. Values are used only for
	// matching and are never retained or emitted.
	isActive func(value string) bool
}

// supportedAgentSignals is the curated allowlist of agent environment
// variables. Keep it sorted by envVar so the table and detector output are easy
// to audit. To support a new signal, add its activation rule and source
// here and cover it in agentsignals_test.go; analytics event construction needs
// no changes.
var supportedAgentSignals = []agentSignal{
	// Amp (community detector consensus; no stable vendor contract found):
	// https://github.com/databricks/databricks-sdk-go/blob/main/useragent/agent.go
	// https://github.com/algolia/cli/blob/main/pkg/telemetry/clicontext.go
	{envVar: "AMP_CURRENT_THREAD_ID", isActive: isNonEmptyAgentMarker},
	// Claude Code:
	// https://code.claude.com/docs/en/env-vars
	{envVar: "CLAUDECODE", isActive: isActiveAgentBooleanMarker},
	// Datadog PUP uses CLINE:
	// https://github.com/DataDog/pup/blob/001239b5293a4081a5135e6e71167a45e0cc4320/README.md?plain=1#L455
	{envVar: "CLINE", isActive: isActiveAgentBooleanMarker},
	// Cline:
	// https://github.com/cline/cline/blob/main/apps/vscode/src/hosts/vscode/terminal/VscodeTerminalRegistry.ts
	// https://github.com/cline/cline/blob/main/apps/vscode/src/hosts/vscode/hostbridge/workspace/executeCommandInTerminal.ts
	{envVar: "CLINE_ACTIVE", isActive: isActiveAgentBooleanMarker},
	// Codex:
	// https://github.com/openai/codex/blob/main/codex-rs/protocol/src/shell_environment.rs
	{envVar: "CODEX_THREAD_ID", isActive: isNonEmptyAgentMarker},
	// GitHub Copilot CLI:
	// https://github.com/github/copilot-cli/blob/main/changelog.md#1029---2026-04-16
	{envVar: "COPILOT_AGENT_SESSION_ID", isActive: isNonEmptyAgentMarker},
	// Cursor Agent:
	// Could not find a source for this one, but Datadog pup uses it
	// https://github.com/DataDog/pup/blob/001239b5293a4081a5135e6e71167a45e0cc4320/README.md?plain=1#L452
	{envVar: "CURSOR_AGENT", isActive: isNonEmptyAgentMarker},
	// Cursor IDE agent (maintained detector evidence; no vendor contract found):
	// https://github.com/vercel/vercel/blob/main/packages/detect-agent/src/index.ts
	// https://github.com/getsentry/cli/blob/main/src/lib/detect-agent.ts
	{envVar: "CURSOR_TRACE_ID", isActive: isNonEmptyAgentMarker},
	// Gemini CLI:
	// https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/services/shellExecutionService.ts
	{envVar: "GEMINI_CLI", isActive: isActiveAgentBooleanMarker},
	// Goose:
	// https://github.com/block/goose/blob/main/documentation/docs/guides/environment-variables.md
	{envVar: "GOOSE_TERMINAL", isActive: isActiveAgentBooleanMarker},
	// OpenCode:
	// https://github.com/anomalyco/opencode/blob/main/packages/opencode/src/effect/runtime-flags.ts
	// https://github.com/anomalyco/opencode/blob/main/packages/desktop/src/main/server.ts
	// https://github.com/anomalyco/opencode/blob/main/packages/opencode/src/cli/cmd/acp.ts
	{envVar: "OPENCODE_CLIENT", isActive: isActiveOpenCodeMarker},
	// Pi coding agent:
	// https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/environment-variables.md
	{envVar: "PI_CODING_AGENT", isActive: isActiveAgentBooleanMarker},
	// VS Code agent terminal:
	// https://code.visualstudio.com/updates/v1_121
	{envVar: "VSCODE_AGENT", isActive: isNonEmptyAgentMarker},
}

// DetectAgentSignals returns the sorted, deduplicated names of active,
// allowlisted agent environment variables. It never returns nil, so
// agent_signals serializes as [] rather than null.
func DetectAgentSignals() []string {
	signals := []string{}
	for _, signal := range supportedAgentSignals {
		if signal.isActive(os.Getenv(signal.envVar)) {
			signals = append(signals, signal.envVar)
		}
	}

	slices.Sort(signals)
	return slices.Compact(signals)
}

func isActiveAgentBooleanMarker(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "1" || strings.EqualFold(trimmed, "true")
}

func isNonEmptyAgentMarker(value string) bool {
	return value != ""
}

func isActiveOpenCodeMarker(value string) bool {
	return value == "desktop" || value == "acp"
}
