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

// CopyOut is the structured result of a sandbox file transfer.
type CopyOut struct {
	Data CopyOutData `json:"data"`
}

// CopyOutData describes the transfer that ran. LocalPath is the path actually
// written on this machine, which is not always the one the caller named: a
// download into an existing directory lands under the name the sandbox
// suggested.
type CopyOutData struct {
	SandboxID  string        `json:"sandboxId"`
	Direction  CopyDirection `json:"direction"`
	LocalPath  string        `json:"localPath"`
	RemotePath string        `json:"remotePath"`
}

// CopyDirection is which way a transfer moved, named from the sandbox's
// perspective as the copy subcommand's arguments are.
type CopyDirection string

const (
	CopyDirectionUpload   CopyDirection = "upload"
	CopyDirectionDownload CopyDirection = "download"
)
