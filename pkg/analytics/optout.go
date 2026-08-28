package analytics

// OptOutReason identifies the mechanism that disabled analytics.
type OptOutReason string

const (
	// OptOutReasonNone means no opt-out mechanism applies and analytics is enabled.
	OptOutReasonNone OptOutReason = ""
	// OptOutReasonDoNotTrack means DO_NOT_TRACK disabled analytics.
	OptOutReasonDoNotTrack OptOutReason = "DO_NOT_TRACK"
	// OptOutReasonDisableAnalyticsEnv means RENDER_CLI_DISABLE_ANALYTICS
	// disabled analytics.
	OptOutReasonDisableAnalyticsEnv OptOutReason = "RENDER_CLI_DISABLE_ANALYTICS"
)
