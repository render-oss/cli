package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	lclient "github.com/render-oss/cli/pkg/client/logs"
)

// decodeYAMLStream strictly decodes every document in out until EOF.
func decodeYAMLStream(t *testing.T, out string) []lclient.Log {
	t.Helper()
	dec := yaml.NewDecoder(strings.NewReader(out))
	dec.KnownFields(true)
	var decoded []lclient.Log
	for {
		var log lclient.Log
		err := dec.Decode(&log)
		if errors.Is(err, io.EOF) {
			return decoded
		}
		require.NoError(t, err, "output:\n%s", out)
		decoded = append(decoded, log)
	}
}

func orderedLogs() []lclient.Log {
	base := time.Date(2026, 10, 8, 5, 0, 0, 0, time.UTC)
	return []lclient.Log{
		renderapi.NewLog(lclient.Log{Message: "first marker", Timestamp: base}),
		renderapi.NewLog(lclient.Log{Message: "second marker", Timestamp: base.Add(time.Second)}),
	}
}

// Two listed entries must be two YAML documents, not one mapping with
// duplicate keys that strict parsers reject and permissive parsers collapse.
func TestLogsListYAMLIsAMultiDocumentStream(t *testing.T) {
	server := renderapi.NewServer(t)
	logs := orderedLogs()
	server.Logs.Instances = logs

	result, err := executeLogsCommand(t, server, "--output", "yaml")
	require.NoError(t, err)

	decoded := decodeYAMLStream(t, result.Stdout)
	require.Len(t, decoded, 2)
	for i := range logs {
		require.Equal(t, logs[i].Id, decoded[i].Id)
		require.Equal(t, logs[i].Message, decoded[i].Message)
		require.True(t, logs[i].Timestamp.Equal(decoded[i].Timestamp))
	}
}

func TestLogsListYAMLWithNoEntriesIsEmpty(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeLogsCommand(t, server, "--output", "yaml")
	require.NoError(t, err)
	require.Empty(t, result.Stdout)
}

func TestLogsTailYAMLIsAMultiDocumentStream(t *testing.T) {
	server := renderapi.NewServer(t)
	logs := orderedLogs()
	// The malformed trailing message ends the tail so the command returns.
	server.Logs.QueueStreams(renderapi.LogStreamConfig{Logs: logs, RawMessage: "invalid JSON"})

	result, _ := executeLogsCommand(t, server, "--tail", "--output", "yaml")

	decoded := decodeYAMLStream(t, result.Stdout)
	require.Len(t, decoded, 2)
	require.Equal(t, "first marker", decoded[0].Message)
	require.Equal(t, "second marker", decoded[1].Message)
}

// JSON and text output are unchanged: JSON stays one indented object per entry.
func TestLogsListJSONAndTextOutputUnchanged(t *testing.T) {
	server := renderapi.NewServer(t)
	logs := orderedLogs()
	server.Logs.Instances = logs

	result, err := executeLogsCommand(t, server, "--output", "json")
	require.NoError(t, err)
	var want strings.Builder
	for i := range logs {
		b, err := json.MarshalIndent(&logs[i], "", "  ")
		require.NoError(t, err)
		want.Write(b)
	}
	require.Equal(t, want.String(), result.Stdout)

	result, err = executeLogsCommand(t, server, "--output", "text")
	require.NoError(t, err)
	require.Equal(t,
		"2026-10-08 05:00:00  first marker\n2026-10-08 05:00:01  second marker\n",
		result.Stdout)
}
