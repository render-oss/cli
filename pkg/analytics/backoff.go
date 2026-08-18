package analytics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/render-oss/cli/internal/files"
)

const (
	backoffFileName = "backoff.json"
	// maxBackoffFileSizeBytes bounds a corrupt or truncated file. A well-formed
	// one is well under a hundred bytes.
	maxBackoffFileSizeBytes = 4 << 10

	// minBackoff floors a Retry-After the server sends. A rate limiter's window
	// can be a second or two, which would leave the fleet effectively unquieted.
	minBackoff = 30 * time.Second

	// maxBackoff is the max backoff for all causes.
	// This prevents a bug or skewed clock from silencing analytics indefinitely.
	maxBackoff = 24 * time.Hour
)

// defaultBackoffByStatus defines which statuses cause the CLI to  backoff and the
// duration to use when they do not have a valid Retry-After header. A status
// absent from this map never triggers a backoff, even if it has Retry-After.
var defaultBackoffByStatus = map[int]time.Duration{
	http.StatusTooManyRequests:    5 * time.Minute,
	http.StatusServiceUnavailable: 30 * time.Minute,
	http.StatusNotFound:           time.Hour,
}

// backoff is the data recorded when the API asks the CLI to stop sending.
// This data must be persisted to the CLI's state dir in order to use it across
// invocations.
type backoff struct {
	// Until is the instant sending may resume.
	Until time.Time `json:"until"`
	// StatusCode is the response that caused the backoff, kept for logging.
	StatusCode int `json:"status_code"`
}

// inEffect reports whether we should not send at the provided moment to comply
// with a backoff. Zero-value backoffs and backoffs whose remaining duration is
// longer than [maxBackoff] are not in effect.
func (b backoff) inEffect(now time.Time) bool {
	// A longer backoff cannot have come from backoffFor, which bounds every
	// backoff it builds. It could mean a skewed clock or a tampered file, so
	// ignore it rather than let it silence analytics indefinitely.
	if b.Until.After(now.Add(maxBackoff)) {
		return false
	}
	return now.Before(b.Until)
}

// backoffFor determines how long to back off, if at all, from the status code
// and Retry-After header of an attempted event POST.
func backoffFor(response postResponse, now time.Time) backoff {
	window, ok := defaultBackoffByStatus[response.StatusCode]
	if !ok {
		return backoff{}
	}

	if retryAfter := parseRetryAfter(response.RetryAfter, now); retryAfter > 0 {
		window = retryAfter
	}
	window = min(max(window, minBackoff), maxBackoff)

	return backoff{
		Until:      now.Add(window),
		StatusCode: response.StatusCode,
	}
}

// parseRetryAfter reads a Retry-After header in either form RFC 9110 allows:
// delta-seconds, or an HTTP-date. A value that is absent, unparseable,
// non-positive, or already in the past is treated as zero.
func parseRetryAfter(value string, now time.Time) time.Duration {
	// maxSeconds is maxBackoff in the unit delta-seconds uses. Bounding the
	// header's value while it is still in seconds keeps a large one from
	// overflowing the scale to a Duration and wrapping negative.
	const maxSeconds = int(maxBackoff / time.Second)

	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(min(max(seconds, 0), maxSeconds)) * time.Second
	}

	if at, err := http.ParseTime(value); err == nil {
		if delay := at.Sub(now); delay > 0 {
			return delay
		}
	}
	return 0
}

// loadBackoff reads the persisted backoff, returning the zero backoff when the
// file is missing, unreadable, or unparseable. Callers use [backoff.inEffect] to
// determine whether loaded state should suppress a send.
func loadBackoff() backoff {
	path, err := backoffPath()
	if err != nil {
		return backoff{}
	}

	data, err := readBackoffFile(path)
	if err != nil {
		return backoff{}
	}

	var b backoff
	if err := json.Unmarshal(data, &b); err != nil {
		return backoff{}
	}
	return b
}

// recordBackoff persists a backoff. The zero backoff is not a backoff and is
// never written.
//
// Write failures are discarded rather than reported: state that cannot be
// written only means the next invocation tries the send again.
func recordBackoff(b backoff) {
	if b.Until.IsZero() {
		return
	}

	path, err := backoffPath()
	if err != nil {
		return
	}
	data, err := json.Marshal(b)
	if err != nil {
		return
	}
	_ = files.Write(path, data)
}

// backoffPath returns the file path for backoff data.
func backoffPath() (string, error) {
	analyticsDir, err := analyticsDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(analyticsDir, backoffFileName), nil
}

// readBackoffFile reads one size-bounded backoff file, reporting an oversized
// one as unusable rather than loading it.
func readBackoffFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxBackoffFileSizeBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBackoffFileSizeBytes {
		return nil, fmt.Errorf("analytics backoff file exceeds %d bytes", maxBackoffFileSizeBytes)
	}
	return data, nil
}
