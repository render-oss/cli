package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRepo builds a Repo whose generated client targets baseURL and signs
// requests with a bearer apiKey, matching how the DI container wires the client.
func newTestRepo(t *testing.T, baseURL, apiKey string) *Repo {
	t.Helper()
	apiClient, err := client.NewClientWithResponses(baseURL, client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}))
	require.NoError(t, err)
	return NewRepo(apiClient)
}

// writeConnectResponse writes a minted connect token the way the API does. The
// JSON content type is load-bearing: the generated client only decodes a body it
// was told is JSON, and without it the token comes back empty.
func writeConnectResponse(w http.ResponseWriter, conn sandboxclient.SandboxConnectResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(conn)
}

// writeAPIError writes an API error response the generated client can decode.
func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func TestExecSandboxStream(t *testing.T) {
	const (
		workspace = "tea-workspace"
		apiKey    = "api-key-xyz"
		runToken  = "run-token-123"
		sandboxID = "sbx-abc123"
	)

	var (
		mintAuth    string
		mintOwnerID string
		streamAuth  string
		streamBody  string
	)

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/runs/stream/token", func(w http.ResponseWriter, r *http.Request) {
		mintAuth = r.Header.Get("Authorization")
		mintOwnerID = r.URL.Query().Get("ownerId")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(sandboxclient.SandboxConnectResponse{
			ExecutionId: "exec-1",
			Token:       runToken,
			Uri:         serverURL + "/exec/stream",
			Method:      http.MethodPost,
			ExpiresAt:   time.Now().Add(time.Hour),
		})
	})
	mux.HandleFunc("/exec/stream", func(w http.ResponseWriter, r *http.Request) {
		streamAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		streamBody = string(b)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: output\ndata: {\"stream\":\"stdout\",\"data\":\"hello\"}\n\nevent: exit\ndata: {\"exit_code\":0}\n\n")
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", workspace)

	repo := newTestRepo(t, srv.URL+"/v1/", apiKey)

	var outputs []ExecOutputEvent
	exitCode, err := repo.ExecSandboxStream(context.Background(), sandboxID, "echo hello", func(e *ExecOutputEvent) error {
		outputs = append(outputs, *e)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "Bearer "+apiKey, mintAuth)
	assert.Equal(t, workspace, mintOwnerID)
	assert.Equal(t, "Bearer "+runToken, streamAuth)
	assert.JSONEq(t, `{"command":"echo hello"}`, streamBody)
	require.Len(t, outputs, 1)
	assert.Equal(t, ExecOutputStreamStdout, outputs[0].Stream)
	assert.Equal(t, "hello", outputs[0].Data)
}

func TestUploadFile(t *testing.T) {
	const (
		sandboxID  = "sbx-abc123"
		fileToken  = "file-token-123"
		remotePath = "/app/data.txt"
	)

	var (
		mintAuth        string
		mintOwnerID     string
		mintPath        string
		uploadedMethod  string
		uploadedAuth    string
		uploadedType    string
		uploadedBody    string
		uploadedRawPath string
	)

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/upload/token", func(w http.ResponseWriter, r *http.Request) {
		mintAuth = r.Header.Get("Authorization")
		mintOwnerID = r.URL.Query().Get("ownerId")
		mintPath = r.URL.Query().Get("path")
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{
			ExecutionId: "exec-1",
			Token:       fileToken,
			Uri:         serverURL + "/files/upload?path=" + url.QueryEscape(remotePath),
			Method:      http.MethodPut,
			ExpiresAt:   time.Now().Add(time.Hour),
		})
	})
	mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadedMethod = r.Method
		uploadedAuth = r.Header.Get("Authorization")
		uploadedType = r.Header.Get("Content-Type")
		uploadedRawPath = r.URL.Query().Get("path")
		b, _ := io.ReadAll(r.Body)
		uploadedBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")

	content := "hello, sandbox"
	err := repo.UploadFile(context.Background(), sandboxID, remotePath, FileContentTypeOctetStream, "", int64(len(content)), strings.NewReader(content))
	require.NoError(t, err)

	assert.Equal(t, "Bearer api-key-xyz", mintAuth, "mint request goes through the authenticated client")
	assert.Equal(t, "tea-workspace", mintOwnerID)
	assert.Equal(t, remotePath, mintPath, "mint request must carry the file path")
	assert.Equal(t, http.MethodPut, uploadedMethod, "transfer must use the method from the mint response")
	assert.Equal(t, "Bearer "+fileToken, uploadedAuth)
	assert.Equal(t, FileContentTypeOctetStream, uploadedType)
	assert.Equal(t, remotePath, uploadedRawPath)
	assert.Equal(t, content, uploadedBody)
}

// The API deploys ahead of the CLI, so a success other than the agent's current
// 204 must not read as a failed upload.
func TestUploadFileAcceptsAnySuccessStatus(t *testing.T) {
	const sandboxID = "sbx-abc123"

	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusAccepted} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var serverURL string
			mux := http.NewServeMux()
			mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/upload/token", func(w http.ResponseWriter, r *http.Request) {
				writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/upload", Method: http.MethodPut})
			})
			mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			})

			srv := httptest.NewServer(mux)
			defer srv.Close()
			serverURL = srv.URL

			t.Setenv("RENDER_WORKSPACE", "tea-workspace")

			repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")
			err := repo.UploadFile(context.Background(), sandboxID, "/f", FileContentTypeOctetStream, "", 1, strings.NewReader("x"))
			assert.NoError(t, err)
		})
	}
}

func TestUploadFileSurfacesAgentError(t *testing.T) {
	const sandboxID = "sbx-abc123"

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/upload/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/upload", Method: http.MethodPut})
	})
	mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInsufficientStorage)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "insufficient_storage", "message": "sandbox is out of disk space"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")

	err := repo.UploadFile(context.Background(), sandboxID, "/f", FileContentTypeOctetStream, "", 1, strings.NewReader("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of disk space")
}

// A failure minting the file token surfaces the API's message, the way
// TestExecSandboxStreamMintError covers the run-token route.
func TestUploadFileMintError(t *testing.T) {
	const sandboxID = "sbx-err"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/upload/token", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, "sandbox not found")
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")

	err := repo.UploadFile(context.Background(), sandboxID, "/f", FileContentTypeOctetStream, "", 1, strings.NewReader("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox not found")
}

func TestDownloadFile(t *testing.T) {
	const (
		sandboxID  = "sbx-abc123"
		fileToken  = "file-token-456"
		remotePath = "/app/output.json"
	)

	var downloadedMethod, downloadedAuth string

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/download/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{
			ExecutionId: "exec-2",
			Token:       fileToken,
			Uri:         serverURL + "/files/download?path=" + url.QueryEscape(remotePath),
			Method:      http.MethodGet,
		})
	})
	mux.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
		downloadedMethod = r.Method
		downloadedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", FileContentTypeOctetStream)
		w.Header().Set("Content-Disposition", `attachment; filename="output.json"`)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")

	stream, err := repo.DownloadFile(context.Background(), sandboxID, remotePath)
	require.NoError(t, err)
	defer stream.Body.Close()

	assert.Equal(t, http.MethodGet, downloadedMethod)
	assert.Equal(t, "Bearer "+fileToken, downloadedAuth)
	assert.Equal(t, FileContentTypeOctetStream, stream.ContentType)
	assert.Equal(t, "output.json", stream.Filename)
	body, err := io.ReadAll(stream.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(body))
}

// Go's Transport adds Accept-Encoding: gzip on a request that didn't set it,
// then transparently decompresses the response and strips the header. So a
// directory download labelled x-tar + Content-Encoding: gzip reaches the caller
// as a plain tar, and the content type still says what the artifact is. If a Go
// release ever changes that, this fails rather than corrupting a download.
func TestDownloadFileTransportDecompressesGzipEncoding(t *testing.T) {
	const sandboxID = "sbx-abc123"

	payload := gzipped(t, []byte("tar bytes"))

	var sentAcceptEncoding string
	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/download/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/download", Method: http.MethodGet})
	})
	mux.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
		sentAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", FileContentTypeTar)
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(payload)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")

	stream, err := repo.DownloadFile(context.Background(), sandboxID, "/app/src")
	require.NoError(t, err)
	defer func() { _ = stream.Body.Close() }()

	assert.Equal(t, "gzip", sentAcceptEncoding, "the transport negotiates gzip on our behalf")
	assert.Equal(t, FileContentTypeTar, stream.ContentType)
	body, err := io.ReadAll(stream.Body)
	require.NoError(t, err)
	assert.Equal(t, "tar bytes", string(body))
}

// The transport only decompresses what it negotiated itself, so a
// Content-Encoding: gzip response it didn't ask for arrives compressed. Nothing
// downstream inspects the encoding, so decoding has to happen here.
func TestDecodeBodyGunzipsUnhandledContentEncoding(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(bytes.NewReader(gzipped(t, []byte("tar bytes")))),
	}

	body, err := decodeBody(resp)
	require.NoError(t, err)
	defer func() { _ = body.Close() }()

	got, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, "tar bytes", string(got))
}

// An identity response passes through untouched: a body that merely happens to
// be gzip (a user's .tar.gz downloaded as a file) must not be decompressed.
func TestDecodeBodyLeavesUnencodedBodyAlone(t *testing.T) {
	payload := gzipped(t, []byte("archive contents"))
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{FileContentTypeOctetStream}},
		Body:   io.NopCloser(bytes.NewReader(payload)),
	}

	body, err := decodeBody(resp)
	require.NoError(t, err)
	defer func() { _ = body.Close() }()

	got, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, payload, got, "the bytes must arrive exactly as sent")
}

// Closing the stream has to close the response body underneath the decoder, or
// the connection leaks.
func TestDecodeBodyCloseClosesUnderlyingBody(t *testing.T) {
	underlying := &trackedCloser{Reader: bytes.NewReader(gzipped(t, []byte("tar bytes")))}
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   underlying,
	}

	body, err := decodeBody(resp)
	require.NoError(t, err)
	require.NoError(t, body.Close())
	assert.True(t, underlying.closed)
}

// Content-Encoding is a token list, and x-gzip is the same coding as gzip
// (RFC 9110). A body we hand over still encoded gets read as a tar or written
// to disk as the artifact, so anything we can't decode has to be an error
// rather than a pass-through.
func TestDecodeBodyContentEncodingTokens(t *testing.T) {
	const payload = "tar bytes"

	for _, tc := range []struct {
		encoding string
		decoded  bool
	}{
		{encoding: "gzip", decoded: true},
		{encoding: "GZIP", decoded: true},
		{encoding: "x-gzip", decoded: true},
		{encoding: " gzip ", decoded: true},
		{encoding: "identity, gzip", decoded: true},
		{encoding: "identity", decoded: false},
		{encoding: "", decoded: false},
	} {
		t.Run("decodes "+tc.encoding, func(t *testing.T) {
			body := gzipped(t, []byte(payload))
			header := http.Header{}
			if tc.encoding != "" {
				header.Set("Content-Encoding", tc.encoding)
			}

			decoded, err := decodeBody(&http.Response{Header: header, Body: io.NopCloser(bytes.NewReader(body))})
			require.NoError(t, err)
			defer func() { _ = decoded.Close() }()

			got, err := io.ReadAll(decoded)
			require.NoError(t, err)
			if tc.decoded {
				assert.Equal(t, payload, string(got))
			} else {
				assert.Equal(t, body, got, "an identity response must arrive byte for byte")
			}
		})
	}

	for _, encoding := range []string{"br", "deflate", "compress", "gzip, br"} {
		t.Run("rejects "+encoding, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{"Content-Encoding": []string{encoding}},
				Body:   io.NopCloser(bytes.NewReader([]byte("whatever"))),
			}

			_, err := decodeBody(resp)
			require.Error(t, err, "a coding we cannot decode must not pass through")
			assert.Contains(t, err.Error(), encoding)
		})
	}
}

type trackedCloser struct {
	io.Reader
	closed bool
}

func (t *trackedCloser) Close() error {
	t.closed = true
	return nil
}

func TestExecSandboxStreamMintError(t *testing.T) {
	const sandboxID = "sbx-err"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/runs/stream/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "sandbox not found"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	repo := newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")

	_, err := repo.ExecSandboxStream(context.Background(), sandboxID, "echo hello", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox not found")
}
