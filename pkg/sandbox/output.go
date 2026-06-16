package sandbox

import (
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// TerminateOut is the structured result of a sandbox stop command. It carries
// the sandbox that was (or would be) terminated plus metadata describing
// whether the destructive action actually ran.
type TerminateOut struct {
	Data *sandboxclient.Sandbox `json:"data"`
	Meta TerminateOutMeta       `json:"meta"`
}

type TerminateOutMeta struct {
	Terminated bool   `json:"terminated"`
	Message    string `json:"message,omitempty"`
}
