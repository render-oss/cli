package serversideevents_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/pointers"
	sse "github.com/render-oss/cli/pkg/workflows/apiserver/internal/serversideevents"
)

type streamRecorder struct {
	h            http.Header
	status       int
	messages     []string
	flushedCount int

	mu sync.Mutex
}

func newStreamRecorder() *streamRecorder {
	return &streamRecorder{
		h: make(http.Header),
	}
}

func (s *streamRecorder) Header() http.Header  { return s.h }
func (s *streamRecorder) WriteHeader(code int) { s.status = code }
func (s *streamRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, string(p))
	return len(p), nil
}
func (s *streamRecorder) Flush() {
	s.flushedCount++
}

func TestServerSideEvents(t *testing.T) {
	t.Run("should write messages to the client", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		ch := make(chan sse.Message[string], 3)
		t.Cleanup(func() {
			close(ch)
			cancel()
		})
		sendMessage := func(id int64) {
			ch <- sse.Message[string]{
				ID:    uint64(id),
				Event: pointers.From("testEvent"),
				Data:  fmt.Sprintf("test %d", id),
			}
		}

		handler := sse.ServerSideEvents(ch)

		writer := newStreamRecorder()
		request := httptest.NewRequest("GET", "/events", nil)
		request = request.WithContext(ctx)

		go func() {
			handler(writer, request)
		}()

		require.Eventually(t, func() bool {
			return writer.h.Get("Content-Type") == "text/event-stream"
		}, time.Second*1, time.Millisecond*10)

		require.Equal(t, "text/event-stream", writer.h.Get("Content-Type"))
		require.Equal(t, "no-cache", writer.h.Get("Cache-Control"))
		require.Equal(t, "keep-alive", writer.h.Get("Connection"))

		require.Eventually(t, func() bool {
			return len(writer.messages) == 1 && writer.flushedCount == 1
		}, time.Second*1, time.Millisecond*10)
		require.Equal(t, "retry: 2000\n\n", string(writer.messages[0]))

		sendMessage(1)

		require.Eventually(t, func() bool {
			// 3 more messages, and 1 more flush
			return len(writer.messages) == 4 && writer.flushedCount == 2
		}, time.Second*1, time.Millisecond*10)
		require.Equal(t, "id: 1\n", string(writer.messages[1]))
		require.Equal(t, "event: testEvent\n", string(writer.messages[2]))
		require.Equal(t, "data: \"test 1\"\n\n", string(writer.messages[3]))

		sendMessage(2)
		require.Eventually(t, func() bool {
			// 3 more messages, and 1 more flush
			return len(writer.messages) == 7 && writer.flushedCount == 3
		}, time.Second*1, time.Millisecond*10)

		require.Equal(t, "id: 2\n", string(writer.messages[4]))
		require.Equal(t, "event: testEvent\n", string(writer.messages[5]))
		require.Equal(t, "data: \"test 2\"\n\n", string(writer.messages[6]))
	})
}
