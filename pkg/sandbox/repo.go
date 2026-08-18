package sandbox

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

func (r *Repo) ListSandboxes(ctx context.Context, params *client.ListSandboxesParams) ([]*sandboxclient.Sandbox, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	params.OwnerId = &client.OwnerIdParam{workspace}

	return client.ListAll(ctx, params, r.listSandboxesPage)
}

func (r *Repo) listSandboxesPage(ctx context.Context, params *client.ListSandboxesParams) ([]*sandboxclient.Sandbox, *client.Cursor, error) {
	resp, err := r.client.ListSandboxesWithResponse(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, nil, err
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return nil, nil, nil
	}

	res := *resp.JSON200
	sandboxes := make([]*sandboxclient.Sandbox, 0, len(res))
	for _, swc := range res {
		sb := swc.Sandbox
		sandboxes = append(sandboxes, &sb)
	}

	return sandboxes, &res[len(res)-1].Cursor, nil
}

func (r *Repo) CreateSandbox(
	ctx context.Context,
	body client.CreateSandboxJSONRequestBody,
	onEvent func(*sandboxclient.Sandbox),
) (*sandboxclient.Sandbox, error) {
	resp, err := r.client.CreateSandboxWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create sandbox: success response missing sandbox body")
	}

	if onEvent != nil {
		onEvent(resp.JSON201)
	}

	return resp.JSON201, nil
}

func (r *Repo) GetSandbox(ctx context.Context, id string) (*sandboxclient.Sandbox, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.RetrieveSandboxWithResponse(ctx, id, &client.RetrieveSandboxParams{OwnerId: workspace})
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) ExecSandboxStream(ctx context.Context, id string, command string, onOutput func(*ExecOutputEvent) error) (int, error) {
	conn, err := r.connect(ctx, id, command)
	if err != nil {
		return 0, err
	}

	body, err := json.Marshal(execCommand{Command: command})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, conn.Method, conn.Uri, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build exec stream request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+conn.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, errFromStreamResponse(resp)
	}
	return readSandboxExecStream(resp.Body, onOutput)
}

// File transfer content types. The content type states intent and nothing else:
// a single file travels as octet-stream and a directory as an x-tar archive,
// with Content-Encoding: gzip carrying wire compression independently of either.
const (
	FileContentTypeOctetStream = "application/octet-stream"
	FileContentTypeTar         = "application/x-tar"
)

// FileStream is a downloading file or directory archive. ContentType is
// FileContentTypeTar for a directory archive and anything else for a single
// file. Body carries the artifact's own bytes, with any Content-Encoding already
// decoded. The caller must close Body.
type FileStream struct {
	ContentType string
	// Filename is the server-suggested name from Content-Disposition; empty
	// when the server didn't send one.
	Filename string
	Body     io.ReadCloser
}

// UploadFile mints an upload connect token for remotePath and streams body
// directly through the sandbox proxy. Pass contentLength -1 when unknown
// (e.g. a tar stream). Pass contentEncoding "gzip" to declare a gzip-compressed
// body, which the server gunzips before handling; empty for none.
func (r *Repo) UploadFile(ctx context.Context, id, remotePath, contentType, contentEncoding string, contentLength int64, body io.Reader) error {
	conn, err := r.connectFile(ctx, id, "upload", remotePath)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, conn.Method, conn.Uri, body)
	if err != nil {
		return fmt.Errorf("build upload request: %w", err)
	}
	req.ContentLength = contentLength
	req.Header.Set("Authorization", "Bearer "+conn.Token)
	req.Header.Set("Content-Type", contentType)
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Any 2xx, not just the 204 the agent sends today: the API deploys ahead of
	// the CLI, so pinning the exact code would break on a benign change.
	if !isSuccess(resp.StatusCode) {
		return errFromStreamResponse(resp)
	}
	return nil
}

// DownloadFile mints a download connect token for remotePath and returns the
// streaming body from the sandbox proxy.
func (r *Repo) DownloadFile(ctx context.Context, id, remotePath string) (*FileStream, error) {
	conn, err := r.connectFile(ctx, id, "download", remotePath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, conn.Method, conn.Uri, nil)
	if err != nil {
		return nil, fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+conn.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if !isSuccess(resp.StatusCode) {
		defer resp.Body.Close()
		return nil, errFromStreamResponse(resp)
	}

	filename := ""
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		filename = params["filename"]
	}
	contentType := resp.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}

	body, err := decodeBody(resp)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return &FileStream{ContentType: contentType, Filename: filename, Body: body}, nil
}

// decodeBody strips Content-Encoding from a response body so the caller sees the
// artifact's own bytes, whatever the wire did to them. A coding it cannot decode
// is an error: passing those bytes on means writing them to disk as the file, or
// reading them as a tar.
//
// Decoding here is a fallback. The download request sets no Accept-Encoding, so
// the transport adds one, transparently decompresses a gzip response, and
// removes the header before we get here. The header only survives when the
// transport didn't negotiate the encoding itself. Note that Content-Type:
// application/gzip is a payload type, not an encoding, and is deliberately left
// alone.
func decodeBody(resp *http.Response) (io.ReadCloser, error) {
	raw := resp.Header.Get("Content-Encoding")
	codings := contentCodings(raw)
	if len(codings) == 0 {
		return resp.Body, nil
	}
	// Multiple codings would have to be undone in reverse order; nothing sends
	// them, so refuse rather than guess.
	if len(codings) > 1 || !isGzipCoding(codings[0]) {
		return nil, fmt.Errorf("cannot decode Content-Encoding %q", raw)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode gzip response: %w", err)
	}
	return gzipBody{Reader: gz, underlying: resp.Body}, nil
}

// contentCodings splits a Content-Encoding header into lowercased tokens,
// dropping the identity coding, which by definition changes nothing.
func contentCodings(header string) []string {
	var codings []string
	for _, token := range strings.Split(header, ",") {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" || token == "identity" {
			continue
		}
		codings = append(codings, token)
	}
	return codings
}

// isGzipCoding reports whether a coding token is gzip. x-gzip is the same
// coding under a deprecated name (RFC 9110).
func isGzipCoding(coding string) bool { return coding == "gzip" || coding == "x-gzip" }

// gzipBody closes the response body along with the decompressor sitting on it.
type gzipBody struct {
	*gzip.Reader
	underlying io.Closer
}

func (g gzipBody) Close() error {
	err := g.Reader.Close()
	if closeErr := g.underlying.Close(); err == nil {
		err = closeErr
	}
	return err
}

type execCommand struct {
	Command string `json:"command"`
}

// connect mints a run connect token, recording command in the request body so
// the API can store a sanitized copy for the sandbox execution audit trail.
func (r *Repo) connect(ctx context.Context, id, command string) (*sandboxclient.SandboxConnectResponse, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	body := client.ConnectSandboxRunJSONRequestBody{}
	if command != "" {
		body.Command = &command
	}

	resp, err := r.client.ConnectSandboxRunWithResponse(ctx, id, "stream",
		&client.ConnectSandboxRunParams{OwnerId: workspace}, body)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("connect sandbox: success response missing connect token")
	}

	return resp.JSON201, nil
}

// connectFile mints a connect token for a file operation (upload or download)
// against remotePath.
func (r *Repo) connectFile(ctx context.Context, id, operation, remotePath string) (*sandboxclient.SandboxConnectResponse, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.ConnectSandboxFilesWithResponse(ctx, id, operation, &client.ConnectSandboxFilesParams{
		OwnerId: workspace,
		Path:    remotePath,
	})
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("connect sandbox files: success response missing connect token")
	}

	return resp.JSON201, nil
}

func isSuccess(statusCode int) bool { return statusCode >= 200 && statusCode < 300 }

// errFromStreamResponse parses an error out of a raw response.
// client.ErrorFromResponse reflects over the generated *WithResponse structs and
// can't be used on the *http.Response from a hand-built request, so this
// mirrors its behavior: map the standard auth codes to shared sentinels and
// surface the API's structured error message where present.
func errFromStreamResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return client.ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return client.ErrForbidden
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("received response code %d", resp.StatusCode)
	}

	var apiErr client.Error
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != nil && *apiErr.Message != "" {
		return fmt.Errorf("received response code %d: %s", resp.StatusCode, *apiErr.Message)
	}

	if len(body) > 0 {
		return fmt.Errorf("received response code %d: %s", resp.StatusCode, body)
	}
	return fmt.Errorf("received response code %d", resp.StatusCode)
}

func (r *Repo) TerminateSandbox(ctx context.Context, id string) error {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return err
	}

	resp, err := r.client.TerminateSandboxWithResponse(ctx, id, &client.TerminateSandboxParams{OwnerId: workspace})
	if err != nil {
		return err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return err
	}

	return nil
}
