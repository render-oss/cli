package logs_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/workflows/logs"
	"github.com/stretchr/testify/require"
)

func TestLogInterceptor(t *testing.T) {
	t.Run("Can write logs to file and interceptor", func(t *testing.T) {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		file, err := os.CreateTemp("", "test-log-*.log")
		require.NoError(t, err)
		defer os.Remove(file.Name())

		ls := logs.NewLogStore()
		ls.Start(ctx)

		interceptor := logs.NewLogInterceptor("test", file, ls)

		interceptor.Write([]byte("Hello, world!\n"))
		interceptor.Write([]byte("Goodbye, world!\n"))

		require.Eventually(t, func() bool {
			return len(ls.GetLogs(logs.LogSearch{TaskRunID: []string{"test"}})) == 2
		}, time.Second*3, time.Millisecond*10)

		require.Equal(t, "Hello, world!\n", ls.GetLogs(logs.LogSearch{TaskRunID: []string{"test"}})[0].Message)
		require.Equal(t, "Goodbye, world!\n", ls.GetLogs(logs.LogSearch{TaskRunID: []string{"test"}})[1].Message)
	})
}

func TestLogs(t *testing.T) {
	t.Run("Can filter logs", func(t *testing.T) {
		now := time.Now()
		ls := logs.Logs{
			&logs.Log{
				TaskRunID: "test",
				Message:   "Hello, world!",
				Timestamp: now,
			}, &logs.Log{
				TaskRunID: "test",
				Message:   "Goodbye, world!",
				Timestamp: now.Add(time.Minute),
			}}

		t.Run("Can filter by task run ID", func(t *testing.T) {
			results := ls.GetLogs(logs.LogSearch{TaskRunID: []string{"test"}})
			require.Equal(t, 2, len(results))
			require.Equal(t, "Hello, world!", results[0].Message)
			require.Equal(t, "Goodbye, world!", results[1].Message)
		})

		t.Run("Can filter by timestamp", func(t *testing.T) {
			results := ls.GetLogs(logs.LogSearch{StartTime: now.Add(-time.Second), EndTime: now.Add(time.Second)})
			require.Equal(t, 1, len(results))
			require.Equal(t, "Hello, world!", results[0].Message)
		})

		t.Run("Can filter by text", func(t *testing.T) {
			results := ls.GetLogs(logs.LogSearch{Text: []string{"Hello"}})
			require.Equal(t, 1, len(results))
			require.Equal(t, "Hello, world!", results[0].Message)
		})
	})
}

func TestLogChan(t *testing.T) {
	t.Run("Streams logs", func(t *testing.T) {
		ls := logs.NewLogStore()
		ls.Start(context.Background())

		ch := ls.LogChan(logs.LogSearch{TaskRunID: []string{"test"}})

		ls.AddLog(&logs.Log{
			TaskRunID: "test",
			Message:   "Hello, world!",
			Timestamp: time.Now(),
		})

		require.Equal(t, "Hello, world!", (<-ch).Message)
	})

	t.Run("Streams previous logs", func(t *testing.T) {
		ls := logs.NewLogStore()
		ls.Start(context.Background())

		now := time.Now()

		ls.AddLog(&logs.Log{
			TaskRunID: "test",
			Message:   "Hello, world!",
			Timestamp: now,
		})

		ch := ls.LogChan(logs.LogSearch{TaskRunID: []string{"test"}, StartTime: now.Add(-time.Second)})

		require.Equal(t, "Hello, world!", (<-ch).Message)
	})
}

func TestRemoveLogChan(t *testing.T) {
	t.Run("Removes log chan", func(t *testing.T) {
		ls := logs.NewLogStore()
		ls.Start(context.Background())

		ch := ls.LogChan(logs.LogSearch{TaskRunID: []string{"test"}})
		ch2 := ls.LogChan(logs.LogSearch{TaskRunID: []string{"test"}})
		ls.RemoveLogChan(ch)

		// Should be closed
		<-ch

		ls.AddLog(&logs.Log{
			TaskRunID: "test",
			Message:   "Hello, world!",
			Timestamp: time.Now(),
		})

		// ch2 should still be able to receive logs
		require.Equal(t, "Hello, world!", (<-ch2).Message)
	})
}
