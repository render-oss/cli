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

// postResponse is the part of a telemetry response the CLI acts on.
type postResponse struct {
	StatusCode int
	Status     string
	RetryAfter string
}

// postEvent posts one event using the caller's context and reduces the response
// to the fields the CLI acts on. The body is closed unread: delivery is
// best-effort and nothing consumes error response bodies.
func (s *Sender) postEvent(ctx context.Context, payload client.CreateCliTelemetryEventJSONRequestBody) (postResponse, error) {
	response, err := s.client.CreateCliTelemetryEvent(ctx, payload)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return postResponse{}, err
	}
	if response == nil {
		return postResponse{}, errors.New("empty response")
	}
	return postResponse{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		RetryAfter: response.Header.Get("Retry-After"),
	}, nil
}
