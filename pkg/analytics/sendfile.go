package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/render-oss/cli/pkg/client"
)

const maxEventFileSizeBytes = 64 << 10 // 64 KiB

// SendFile sends one analytics event from a file in (config dir)/state/analytics/events.
// The file is removed before the bounded request, so a failed delivery intentionally drops the
// event instead of retaining it for a later invocation.
//
// When analytics logging is enabled, mirroring Send's internal logging gate,
// a successful send writes one outcome line to diagnostics and failures are
// written there as well. Failures are also returned, so a
// caller waiting on this process can detect a failed send through the exit
// code even when logging is off. The event payload is not logged here: the
// process that writes an event file should log it at write time.
func (s *Sender) SendFile(ctx context.Context, path string, diagnostics io.Writer) error {
	err := s.sendFile(ctx, path, diagnostics)
	if err != nil && s.shouldLog {
		_, _ = fmt.Fprintf(diagnostics, "analytics error: %v\n", err)
	}
	return err
}

// sendFile is the internal implementation details for [Sender.SendFile]
// If successful, it adds a success line to diagnostics.
// Errors are returned for [Sender.SendFile] to log
func (s *Sender) sendFile(ctx context.Context, path string, diagnostics io.Writer) error {
	file, err := openEventFile(path)
	if err != nil {
		return err
	}

	payload, readErr := readEvent(file)
	closeErr := file.Close()
	removeErr := os.Remove(path)
	cleanupErr := errors.Join(
		wrapError("close analytics event file", closeErr),
		wrapError("remove analytics event file", removeErr),
	)
	if readErr != nil {
		return errors.Join(readErr, cleanupErr)
	}

	// Cleanup failures do not block the send because nothing retains or sweeps
	// event files. If a sweeper is introduced, require removal to succeed before
	// sending so a retained file cannot be sent twice.
	//
	// A child process constructs a fresh Sender, so this gate reflects the
	// environment at send time: an opt-out between launch and send wins.
	if !s.shouldSend {
		return cleanupErr
	}

	requestCtx, cancel := context.WithTimeout(ctx, sendFileRequestTimeout)
	defer cancel()

	statusCode, status, err := s.postEvent(requestCtx, payload)
	if err != nil {
		return errors.Join(fmt.Errorf("send analytics event: %w", err), cleanupErr)
	}
	if statusCode < 200 || statusCode >= 300 {
		return errors.Join(fmt.Errorf("send analytics event: unexpected response %s", status), cleanupErr)
	}
	if s.shouldLog {
		_, _ = fmt.Fprintf(diagnostics, "analytics response: %s\n", status)
	}
	return cleanupErr
}

// openEventFile validates that path identifies an event file in the events
// directory, then opens that same file.
func openEventFile(path string) (*os.File, error) {
	eventsDir, err := eventsDirectory()
	if err != nil {
		return nil, fmt.Errorf("resolve analytics event directory: %w", err)
	}
	// path arrives as a command argument and SendFile removes what it opens, so
	// this is the check that keeps an arbitrary file from being deleted.
	// Requiring an already-clean absolute path keeps it the same string SendFile
	// passes to os.Remove.
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) != eventsDir {
		return nil, fmt.Errorf("analytics event file must be a direct child of %s", eventsDir)
	}
	name := filepath.Base(path)
	if !strings.HasPrefix(name, eventFilePrefix) || !strings.HasSuffix(name, eventFileSuffix) {
		return nil, errors.New("analytics event file has an invalid name")
	}

	// Lstat so that a symlink fails IsRegular instead of being followed.
	// Permission bits are not checked: Windows synthesizes them rather than
	// reporting the mode we wrote.
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect analytics event file: %w", err)
	}
	if !pathInfo.Mode().IsRegular() {
		return nil, errors.New("analytics event file must be a regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open analytics event file: %w", err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect opened analytics event file: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
		_ = file.Close()
		return nil, errors.New("analytics event file changed while opening")
	}
	return file, nil
}

// readEvent reads one size-bounded file and decodes the analytics event it
// holds. The cap bounds a corrupt or truncated file; the typed decode drops
// anything that is not an event field and rejects trailing JSON values.
func readEvent(file *os.File) (client.CreateCliTelemetryEventJSONRequestBody, error) {
	data, err := io.ReadAll(io.LimitReader(file, maxEventFileSizeBytes+1))
	if err != nil {
		return client.CreateCliTelemetryEventJSONRequestBody{}, fmt.Errorf("read analytics event file: %w", err)
	}
	if len(data) > maxEventFileSizeBytes {
		return client.CreateCliTelemetryEventJSONRequestBody{}, fmt.Errorf("analytics event file exceeds %d bytes", maxEventFileSizeBytes)
	}

	// The typed decode rejects every non-object except null, which unmarshals
	// as a no-op and would send a zero-valued event.
	if trimmed := bytes.TrimLeft(data, " \t\r\n"); len(trimmed) == 0 || trimmed[0] != '{' {
		return client.CreateCliTelemetryEventJSONRequestBody{}, errors.New("analytics event file must contain a JSON object")
	}

	var payload client.CreateCliTelemetryEventJSONRequestBody
	if err := json.Unmarshal(data, &payload); err != nil {
		return client.CreateCliTelemetryEventJSONRequestBody{}, fmt.Errorf("decode analytics event file: %w", err)
	}
	return payload, nil
}

// wrapError adds action context to err while preserving nil errors for use
// with errors.Join.
func wrapError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}
