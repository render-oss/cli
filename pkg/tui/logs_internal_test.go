package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	lclient "github.com/render-oss/cli/pkg/client/logs"
)

func TestFormatLogsSanitizesControlBytes(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	lines := formatLogs([]lclient.Log{{
		Timestamp: ts,
		Message:   "before\x1b[1A\x1b[2K\x1b]8;;https://attacker.example\x1b\\link\x1b]8;;\x1b\\\x1b]52;c;QUJD\x07\rafter",
	}})

	require.Len(t, lines, 1)
	require.Equal(t, "2026-01-02 03:04:05  beforelinkafter", lines[0])
}
