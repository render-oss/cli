package blueprint

import bptypes "github.com/render-oss/cli/pkg/client/blueprints"

// CreateOut preserves the REST create result, including a saved Blueprint when
// planning fails.
type CreateOut struct {
	Data *bptypes.CreateBlueprintResponse `json:"data"`
	Meta CreateOutMeta                    `json:"meta"`
}

type CreateOutMeta struct {
	Next *string `json:"next"`
}
