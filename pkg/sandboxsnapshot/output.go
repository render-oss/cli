package sandboxsnapshot

import (
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type DeleteOut struct {
	Data *sandboxesclient.SandboxSnapshot `json:"data"`
	Meta DeleteOutMeta                    `json:"meta"`
}

type DeleteOutMeta struct {
	Deleted bool   `json:"deleted"`
	Message string `json:"message,omitempty"`
}
