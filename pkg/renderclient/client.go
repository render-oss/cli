package renderclient

import (
	"context"
	"net/http"
	"os"

	"github.com/renderinc/render-cli/pkg/client"
)

func NewClient() (*client.ClientWithResponses, error) {
	return client.NewClientWithResponses(
		os.Getenv("RENDER_HOST"),
		client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+os.Getenv("RENDER_API_KEY"))
			return nil
		}),
	)
}
