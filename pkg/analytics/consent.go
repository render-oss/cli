package analytics

import "github.com/render-oss/cli/pkg/cfg"

// Consent reports whether analytics events may be sent and, when they may not,
// which opt-out mechanism the user expressed.
type Consent struct {
	// Granted reports whether consent to send analytics events is granted.
	// Consent is presumed unless the user denies it through an opt-out
	// mechanism — it is not a record of an affirmative opt-in.
	Granted bool
	// OptOutReason identifies which opt-out mechanism denied consent;
	// [OptOutReasonNone] when consent is granted.
	OptOutReason OptOutReason
}

// ResolveConsent reports whether the CLI may send analytics events, checking
// the opt-out mechanisms in precedence order and stopping at the first one
// that applies. Consent is granted by default; the first opt-out mechanism
// that applies denies it.
func ResolveConsent() Consent {
	if cfg.DoNotTrack() {
		return Consent{Granted: false, OptOutReason: OptOutReasonDoNotTrack}
	}
	if cfg.AnalyticsDisabledByEnv() {
		return Consent{Granted: false, OptOutReason: OptOutReasonDisableAnalyticsEnv}
	}
	return Consent{Granted: true}
}
