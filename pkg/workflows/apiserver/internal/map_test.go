package internal_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	logClient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/apiserver/internal"
	"github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/stretchr/testify/require"
)

func TestMapLogSearchParams(t *testing.T) {
	t.Run("maps log search params", func(t *testing.T) {
		now := time.Now()
		params := client.ListLogsParams{
			TaskRun:   pointers.From([]string{"taskRunID"}),
			Text:      pointers.From([]string{"text"}),
			StartTime: pointers.From(now.Add(-time.Hour)),
			EndTime:   pointers.From(now),
		}

		searchParams := internal.MapLogSearchParams(params)
		require.Equal(t, []string{"taskRunID"}, searchParams.TaskRunID)
		require.Equal(t, []string{"text"}, searchParams.Text)
		require.Equal(t, now.Add(-time.Hour), searchParams.StartTime)
		require.Equal(t, now, searchParams.EndTime)
	})
}

func TestForwardLogsToWebsocket(t *testing.T) {
	t.Run("forwards logs to websocket", func(t *testing.T) {
		now := time.Now().UTC()
		ch := make(chan *logs.Log, 1)
		readCh := make(chan internal.WebSocketData, 1)
		writeCh := make(chan internal.WebSocketData, 1)

		go internal.ForwardLogsToWebsocket(ch, readCh, writeCh)

		ch <- &logs.Log{
			ID:        "logID",
			Message:   "logMessage",
			Timestamp: now,
		}

		result := <-writeCh
		parsed := &logClient.Log{}
		require.NoError(t, json.Unmarshal(result.Data, parsed))
		require.Equal(t, "logID", parsed.Id)
		require.Equal(t, "logMessage", parsed.Message)
		require.Equal(t, now.Truncate(time.Second), parsed.Timestamp.Truncate(time.Second))
	})
}
