package analytics

import (
	"context"
	"errors"
	"time"

	"github.com/render-oss/cli/pkg/client"
)

// sendFileRequestTimeout bounds the HTTP request made by SendFile. It leaves
// headroom inside the synchronous subprocess's three-second lifecycle budget.
const sendFileRequestTimeout = 2500 * time.Millisecond

// postEvent posts one event using the caller's context and reduces the
// response to its status line. The body is closed unread: delivery is
// best-effort and nothing consumes error responses.
func (s *Sender) postEvent(ctx context.Context, payload client.CreateCliTelemetryEventJSONRequestBody) (statusCode int, status string, err error) {
	response, err := s.client.CreateCliTelemetryEvent(ctx, payload)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return 0, "", err
	}
	if response == nil {
		return 0, "", errors.New("empty response")
	}
	return response.StatusCode, response.Status, nil
}
