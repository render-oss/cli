package sandboximage

import (
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type DeleteOut struct {
	Data *sandboxesclient.SandboxBuild `json:"data"`
	Meta DeleteOutMeta                 `json:"meta"`
}

type DeleteOutMeta struct {
	Deleted bool   `json:"deleted"`
	Message string `json:"message,omitempty"`
}
