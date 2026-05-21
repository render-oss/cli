package keyvalue

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
)

// List fetches Key Value instances for the active workspace, optionally
// filtered by environment.
func List(ctx context.Context, params *client.ListKeyValueParams) ([]*Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	svc := NewService(NewRepo(c), environment.NewRepo(c), project.NewRepo(c))
	return svc.ListKeyValue(ctx, params)
}
