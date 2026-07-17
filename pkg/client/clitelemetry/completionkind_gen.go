// This file has been generated from our REST API schema. Do not edit it manually
// For more details, see public-api-schema/README.md.

// Code generated automatically. DO NOT EDIT.
package client

// CliTelemetryEventPOSTInputCompletionKindValues returns every valid CliTelemetryEventPOSTInputCompletionKind value defined in the
// public API schema.
func CliTelemetryEventPOSTInputCompletionKindValues() []CliTelemetryEventPOSTInputCompletionKind {
	return []CliTelemetryEventPOSTInputCompletionKind{
		CliTelemetryEventPOSTInputCompletionKind("success"),
		CliTelemetryEventPOSTInputCompletionKind("help"),
		CliTelemetryEventPOSTInputCompletionKind("version"),
		CliTelemetryEventPOSTInputCompletionKind("discovery_error"),
		CliTelemetryEventPOSTInputCompletionKind("validation_error"),
		CliTelemetryEventPOSTInputCompletionKind("setup_error"),
		CliTelemetryEventPOSTInputCompletionKind("execution_error"),
		CliTelemetryEventPOSTInputCompletionKind("explicit_exit"),
	}
}
