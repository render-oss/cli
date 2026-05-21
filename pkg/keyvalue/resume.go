package keyvalue

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
)

// Resume resumes the Key Value instance with the given ID via the Render API.
func Resume(ctx context.Context, id string) error {
	c, err := client.NewDefaultClient()
	if err != nil {
		return err
	}
	return NewRepo(c).ResumeKeyValue(ctx, id)
}
