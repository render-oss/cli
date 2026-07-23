package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	commandpkg "github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

// onExecutionCompleteFunc runs side effects after a command execution finishes.
type onExecutionCompleteFunc func(result commandpkg.ExecutionResult, deps *dependencies.Dependencies, root *cobra.Command)

// onExecutionComplete is the single process-wide hook for completed executions.
var onExecutionComplete onExecutionCompleteFunc = func(result commandpkg.ExecutionResult, deps *dependencies.Dependencies, root *cobra.Command) {
	// Nil deps means setup failed (or was skipped, as on the --version fast path)
	// before a client existed, so there is no analytics sender to emit with.
	if deps == nil {
		return
	}
	deps.Analytics().Send(result, root.ErrOrStderr())
}

// executionObservation records the Cobra events needed to classify an execution as a [commandpkg.CompletionKind].
// Its fields are independent observations rather than mutually exclusive phases.
// The root version path bypasses Cobra and constructs its result directly.
type executionObservation struct {
	// flagValidationFailed reports that Cobra rejected a parsed flag.
	flagValidationFailed bool
	// helpTarget is the command whose help Cobra rendered, if any. It can differ
	// from the command ExecuteC returns: `render help services` selects the
	// builtin help command but renders help for `render services`.
	helpTarget *cobra.Command
	// helpTargetHadArgs reports that the help target retained positional
	// arguments after flag parsing, not counting arguments after the `--`
	// terminator, which the user explicitly marked as data rather than
	// commands. For a non-runnable group, this means Cobra accepted an
	// unmatched child command before rendering group help.
	helpTargetHadArgs bool
	// setup tracks whether root setup ran and how it concluded.
	setup setupState
}

// setupState describes what an execution observed of root setup, the
// persistent pre-run hook that Cobra runs after selecting a command and
// parsing its flags.
type setupState int

const (
	// setupNotStarted means Cobra never invoked root setup, so any failure
	// happened earlier, during command discovery or flag parsing.
	setupNotStarted setupState = iota
	// setupStarted means root setup is underway. Completed executions never
	// observe this state: the setup hook resolves it to setupSucceeded or
	// setupFailed on every exit.
	setupStarted
	// setupSucceeded means root setup completed and command execution began.
	setupSucceeded
	// setupFailed means root setup returned an error before the command ran.
	setupFailed
)

// executionObservationContextKey stores an [executionObservation]
type executionObservationContextKey struct{}

// prepareExecutionObservation installs a clean [executionObservation] for one
// execution. A single process may call Execute many times — many tests in this
// package and the E2E harness set up a command tree and send commands through
// it — and Cobra child commands retain contexts inherited from earlier runs,
// so an existing observation pointer is cleared in place rather than replaced.
func prepareExecutionObservation(root *cobra.Command) *executionObservation {
	ctx := root.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	observation := executionObservationFromContext(ctx)
	if observation == nil {
		observation = &executionObservation{}
		root.SetContext(context.WithValue(ctx, executionObservationContextKey{}, observation))
		return observation
	}

	clearRetainedHelpRequest(observation)
	*observation = executionObservation{}
	return observation
}

// clearRetainedHelpRequest restores Cobra's help flag after an execution that
// rendered help. Cobra retains parsed flag values when a command tree is reused,
// so leaving the flag set would make the next execution render help too. This
// only affects tests and E2E harnesses that reuse a command tree; a production
// CLI process executes its Cobra application only once.
func clearRetainedHelpRequest(observation *executionObservation) {
	if observation.helpTarget == nil {
		return
	}

	helpFlag := observation.helpTarget.Flags().Lookup("help")
	if helpFlag == nil {
		return
	}
	if err := helpFlag.Value.Set(helpFlag.DefValue); err != nil {
		panic(fmt.Sprintf("reset Cobra help flag: %v", err))
	}
}

// executionObservationFromContext returns the current [executionObservation], if active.
func executionObservationFromContext(ctx context.Context) *executionObservation {
	if ctx == nil {
		return nil
	}
	observation, _ := ctx.Value(executionObservationContextKey{}).(*executionObservation)
	return observation
}

// observationForCommand finds the [executionObservation] for a command. The
// observation lives on the root command's context. Commands Cobra executed
// inherit that context, but the builtin help command renders help by calling
// Help on a target command that was never executed and has no context of its
// own, so lookups fall back to the root.
func observationForCommand(command *cobra.Command) *executionObservation {
	if observation := executionObservationFromContext(command.Context()); observation != nil {
		return observation
	}
	return executionObservationFromContext(command.Root().Context())
}

// observeCobraValidationAndHelp wraps the root command's help and flag-error
// functions so that rendering help or rejecting a flag is recorded in the
// [executionObservation]. Cobra resolves both functions for descendant commands by walking
// up to the root, so wrapping the root once covers the whole command tree. Each
// wrapper delegates to the implementation it replaced.
func observeCobraValidationAndHelp(root *cobra.Command) {
	help := root.HelpFunc()
	root.SetHelpFunc(func(command *cobra.Command, args []string) {
		if observation := observationForCommand(command); observation != nil {
			observation.helpTarget = command
			observation.helpTargetHadArgs = positionalArgsBeforeDash(command) > 0
		}
		help(command, args)
	})
	root.SetFlagErrorFunc(func(command *cobra.Command, err error) error {
		if observation := observationForCommand(command); observation != nil {
			observation.flagValidationFailed = true
		}
		return err
	})
}

// helpExplicitlyRequested reports whether the user asked for help, as opposed to
// Cobra rendering help incidentally (for example when a non-runnable parent
// command is invoked without a subcommand). Explicit requests take two forms:
// the --help / -h flag, which Cobra records on the command it selected, and the
// builtin help command, as in `render help services`.
func helpExplicitlyRequested(command *cobra.Command, observation *executionObservation) bool {
	if observation.helpTarget == nil {
		return false
	}
	if requested, err := observation.helpTarget.Flags().GetBool("help"); err == nil && requested {
		return true
	}
	return isBuiltinHelpCommand(command)
}

// isBuiltinHelpCommand reports whether Cobra selected its builtin help command,
// as in `render help services`.
func isBuiltinHelpCommand(command *cobra.Command) bool {
	return command != nil && command.Name() == "help" && command.Parent() == command.Root()
}

// helpRenderedForUnknownSubcommand reports when Cobra accepts an unmatched
// child command before rendering help for a non-runnable group. Cobra exits
// zero, but the user still named a command that does not exist, so the
// execution classifies as a discovery error rather than success.
//
// A motivating case is `render postgres bogus`: Cobra selects the
// non-runnable group `render postgres`, leaves "bogus" behind as a positional
// argument, renders the group's help, and returns nil. With `render postgres`
// as the non-runnable group:
//
//	render postgres bogus       -> true  (unmatched child; classifies discovery_error)
//	render postgres             -> false (help rendered incidentally, no positional args)
//	render postgres -- bogus    -> false (args after -- are data, never candidate commands)
//	render postgres --help      -> false (the --help flag is not a positional arg;
//	                                      explicit help also classifies first)
//
// The reconstruction is exact rather than heuristic: Cobra renders help from
// only three places, and the other two — an explicit help request (classified
// first, see helpExplicitlyRequested) and a command whose RunE calls Help
// itself, like `render ea` (excluded by the Runnable check) — cannot reach
// this predicate. The only path that leaves a non-runnable help target with
// positional arguments is Cobra failing to match those arguments to a child
// command.
//
// For runnable groups like `ea`, an unknown sub-command results in Cobra automatically returning an error.
// So detecting this is less challenging (Cobra exits 1, and returns an error). So we are able to detect this failure mode without having
// to dig into that helper in cases like these:
//
//	render ea                   -> false (RunE rendered help, but ea is runnable)
//	render ea bogus             -> false (NoArgs rejects the child with an error before
//	                                      help renders; the error path in completionKind
//	                                      classifies it as discovery instead)
func helpRenderedForUnknownSubcommand(observation *executionObservation) bool {
	return observation.helpTarget != nil &&
		!observation.helpTarget.Runnable() &&
		observation.helpTargetHadArgs
}

// positionalArgsBeforeDash counts the positional arguments Cobra parsed for a
// command, excluding everything after the `--` terminator. With
// `render postgres` as the command:
//
//	render postgres                   -> 0
//	render postgres bogus             -> 1
//	render postgres -- bogus          -> 0
//	render postgres bogus -- a b c    -> 1
func positionalArgsBeforeDash(command *cobra.Command) int {
	flags := command.Flags()
	if dash := flags.ArgsLenAtDash(); dash >= 0 {
		return dash
	}
	return flags.NArg()
}

// completionKind classifies how an execution finished, combining the command
// and error Cobra returned with the events the [executionObservation] recorded
// along the way.
//
// A command group's configuration decides how Cobra treats an unknown child,
// and both treatments classify as [commandpkg.CompletionKindDiscoveryError]:
//
//   - A plain non-runnable group (no RunE, nil Args), like `render postgres`,
//     silently renders group help and exits zero. Cobra returns no error, so
//     the nil-error path detects it via helpRenderedForUnknownSubcommand.
//   - A group with Args: cobra.NoArgs and a RunE that renders help, like
//     `render ea`, rejects the token during arg validation and exits nonzero.
//     Cobra returns the error, and the error path recognizes a positional
//     token rejected by a command that has children.
func completionKind(command *cobra.Command, err error, observation *executionObservation) commandpkg.CompletionKind {
	if err == nil {
		if helpExplicitlyRequested(command, observation) {
			return commandpkg.CompletionKindHelp
		}
		if helpRenderedForUnknownSubcommand(observation) {
			return commandpkg.CompletionKindDiscoveryError
		}
		if isBuiltinHelpCommand(command) && observation.helpTarget == nil {
			// `render help bogus`: the builtin help command prints "Unknown help
			// topic" without rendering help, and completes without an error. The
			// user failed to name a command, so classify it as discovery even
			// though Cobra exits zero here.
			return commandpkg.CompletionKindDiscoveryError
		}
		return commandpkg.CompletionKindSuccess
	}

	var exitCoder commandpkg.ExitCoder
	if errors.As(err, &exitCoder) {
		return commandpkg.CompletionKindExplicitExit
	}
	switch observation.setup {
	case setupFailed:
		return commandpkg.CompletionKindSetupError
	case setupSucceeded:
		if command != nil {
			if command.ValidateRequiredFlags() != nil || command.ValidateFlagGroups() != nil {
				return commandpkg.CompletionKindValidationError
			}
		}
		return commandpkg.CompletionKindExecutionError
	}

	// Setup never started (setupStarted always resolves before classification
	// runs): Cobra rejected the command line during command discovery or flag
	// parsing.
	if observation.flagValidationFailed {
		return commandpkg.CompletionKindValidationError
	}
	if command != nil && command.HasSubCommands() && positionalArgsBeforeDash(command) > 0 {
		// Arg validation rejected a positional token on a command with
		// children, as in `render ea bogus` failing cobra.NoArgs. The token is
		// a failed subcommand lookup — the same user mistake as
		// `render postgres bogus` — so classify it as discovery even though
		// Cobra reports it as an argument error.
		return commandpkg.CompletionKindDiscoveryError
	}
	if command != nil && command.Args != nil {
		return commandpkg.CompletionKindValidationError
	}
	return commandpkg.CompletionKindDiscoveryError
}

// newExecutionResult constructs a result when an execution finishes, measuring
// its duration from startedAt to now. The kind is declared by the caller; use
// newClassifiedExecutionResult when it must be derived from Cobra's outcome.
func newExecutionResult(command *cobra.Command, kind commandpkg.CompletionKind, exitCode int, startedAt time.Time) commandpkg.ExecutionResult {
	return commandpkg.ExecutionResult{
		CommandPath:    commandPath(command),
		CompletionKind: kind,
		Duration:       time.Since(startedAt),
		ExitCode:       exitCode,
	}
}

// outputFormatFromExecution returns the final format stored in the command's
// context. Commands may replace interactive output with text during RunE, so
// read it after Cobra finishes rather than copying it from persistent pre-run.
// If persistent pre-run never resolved an output format, leave it absent so
// analytics can report unknown without reconstructing a value after the fact.
func outputFormatFromExecution(command *cobra.Command) *commandpkg.Output {
	if command != nil {
		if output := commandpkg.GetFormatFromContext(command.Context()); output != nil {
			return output
		}
	}
	return nil
}

// commandPath returns the space-joined path of the command Cobra resolved, the
// privacy-safe label emitted as analytics. It reports only matched command
// names, never user arguments or flag values.
func commandPath(command *cobra.Command) string {
	if command == nil {
		return ""
	}
	return command.CommandPath()
}

// newClassifiedExecutionResult derives the completion kind and exit code from
// Cobra's outcome and the observed events, then constructs the result.
func newClassifiedExecutionResult(command *cobra.Command, err error, observation *executionObservation, startedAt time.Time) commandpkg.ExecutionResult {
	kind := completionKind(command, err, observation)
	resultCommand := command
	if kind == commandpkg.CompletionKindHelp {
		// Attribute help to the command whose help was rendered. For
		// `render help services` Cobra selects the builtin help command, while
		// the help shown belongs to `render services`.
		resultCommand = observation.helpTarget
	}
	result := newExecutionResult(resultCommand, kind, exitCodeFromError(err), startedAt)
	result.OutputFormat = outputFormatFromExecution(resultCommand)
	return result
}

// runExecution runs a fully configured root command tree and returns the
// result of that execution. Execute drives it on the process-wide root; tests
// drive it on a fresh root built for a single invocation, which is why it takes
// the root rather than reaching for the package global.
func runExecution(root *cobra.Command, startedAt time.Time) commandpkg.ExecutionResult {
	observation := prepareExecutionObservation(root)
	// cobra.Command.Execute is a thin wrapper over ExecuteC; dropping to the
	// lower-level call hands back the selected + executed sub-command so the
	// result can be classified against it.
	executed, err := root.ExecuteC()
	return newClassifiedExecutionResult(executed, err, observation, startedAt)
}
