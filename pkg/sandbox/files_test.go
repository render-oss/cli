package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

func TestTarRoundTrip(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(src, "top.txt"), []byte("top"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "deep.txt"), []byte("deep"), 0o640))
	require.NoError(t, os.Symlink("top.txt", filepath.Join(src, "link")))

	var buf bytes.Buffer
	require.NoError(t, writeTar(&buf, src))

	dst := t.TempDir()
	require.NoError(t, extractTar(dst, &buf))

	top, err := os.ReadFile(filepath.Join(dst, "top.txt"))
	require.NoError(t, err)
	assert.Equal(t, "top", string(top))
	deep, err := os.ReadFile(filepath.Join(dst, "sub", "deep.txt"))
	require.NoError(t, err)
	assert.Equal(t, "deep", string(deep))
	link, err := os.Readlink(filepath.Join(dst, "link"))
	require.NoError(t, err)
	assert.Equal(t, "top.txt", link)
}

// A stray socket in a project directory (a running dev server, say) must not
// take the whole upload down with archive/tar's "sockets not supported".
func TestWriteTarSkipsSpecialFiles(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o640))
	listener, err := net.Listen("unix", filepath.Join(src, "dev.sock"))
	require.NoError(t, err)
	defer listener.Close()

	var buf bytes.Buffer
	require.NoError(t, writeTar(&buf, src))

	dst := t.TempDir()
	require.NoError(t, extractTar(dst, &buf))

	got, err := os.ReadFile(filepath.Join(dst, "keep.txt"))
	require.NoError(t, err)
	assert.Equal(t, "keep", string(got))

	_, err = os.Lstat(filepath.Join(dst, "dev.sock"))
	assert.True(t, os.IsNotExist(err), "the socket must be skipped, not recreated")
}

// Upload's os.Stat follows a symlink to a directory and takes the tar path, so
// the walk has to follow it too or the archive comes out empty and the transfer
// silently succeeds having sent nothing.
func TestWriteTarFollowsSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	require.NoError(t, os.MkdirAll(filepath.Join(real, "sub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(real, "sub", "x.txt"), []byte("x"), 0o640))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(real, link))

	var buf bytes.Buffer
	require.NoError(t, writeTar(&buf, link))

	dst := t.TempDir()
	require.NoError(t, extractTar(dst, &buf))

	got, err := os.ReadFile(filepath.Join(dst, "sub", "x.txt"))
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))
}

// The agent's tar (tar -c -C src .) emits "./"-prefixed entry names, including
// a bare "./" root entry; extraction must handle them.
func TestExtractTarDotSlashEntries(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o750}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "./sub/", Typeflag: tar.TypeDir, Mode: 0o750}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "./sub/x.txt", Typeflag: tar.TypeReg, Mode: 0o640, Size: 1}))
	_, err := tw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	dst := t.TempDir()
	require.NoError(t, extractTar(dst, &buf))

	got, err := os.ReadFile(filepath.Join(dst, "sub", "x.txt"))
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))
}

// The sandbox's tar emits TypeLink for multiply-linked files. Skipping them
// would drop the file entirely and still report success.
func TestExtractTarHardLink(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "./orig.txt", Typeflag: tar.TypeReg, Mode: 0o640, Size: 4}))
	_, err := tw.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "./sub/hard.txt", Typeflag: tar.TypeLink, Linkname: "./orig.txt", Mode: 0o640}))
	require.NoError(t, tw.Close())

	dst := t.TempDir()
	require.NoError(t, extractTar(dst, &buf))

	got, err := os.ReadFile(filepath.Join(dst, "sub", "hard.txt"))
	require.NoError(t, err)
	assert.Equal(t, "data", string(got))

	orig, err := os.Stat(filepath.Join(dst, "orig.txt"))
	require.NoError(t, err)
	hard, err := os.Stat(filepath.Join(dst, "sub", "hard.txt"))
	require.NoError(t, err)
	assert.True(t, os.SameFile(orig, hard), "extracted entry must be the same file, not a copy")
}

func TestExtractTarRejectsEscapingHardLink(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "stolen.txt", Typeflag: tar.TypeLink, Linkname: outside, Mode: 0o640}))
	require.NoError(t, tw.Close())

	dst := t.TempDir()
	require.Error(t, extractTar(dst, &buf))
	_, err := os.Stat(filepath.Join(dst, "stolen.txt"))
	assert.True(t, os.IsNotExist(err), "must not link to a target outside the destination")
}

func TestExtractTarRejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "../escape.txt", Typeflag: tar.TypeReg, Mode: 0o640, Size: 5}))
	_, err := tw.Write([]byte("pwned"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	parent := t.TempDir()
	dst := filepath.Join(parent, "dst")
	require.Error(t, extractTar(dst, &buf))
	_, err = os.Stat(filepath.Join(parent, "escape.txt"))
	assert.True(t, os.IsNotExist(err), "nothing must be written outside the destination")
}

// A malicious archive can pass the name check with a symlink escaping the
// destination and then a file written through it. os.Root must refuse the
// write, since the archive comes from an untrusted sandbox.
func TestExtractTarRejectsSymlinkEscape(t *testing.T) {
	outside := t.TempDir()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "evil", Typeflag: tar.TypeSymlink, Linkname: outside, Mode: 0o777}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "evil/pwned", Typeflag: tar.TypeReg, Mode: 0o640, Size: 5}))
	_, err := tw.Write([]byte("pwned"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	dst := t.TempDir()
	require.Error(t, extractTar(dst, &buf))
	_, err = os.Stat(filepath.Join(outside, "pwned"))
	assert.True(t, os.IsNotExist(err), "nothing must be written through an escaping symlink")
}

// A file whose name merely starts with ".." (but doesn't traverse upward) is a
// legitimate local entry; the old lexical guard rejected it, os.Root does not.
func TestExtractTarAllowsDotDotPrefixName(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "..foo", Typeflag: tar.TypeReg, Mode: 0o640, Size: 3}))
	_, err := tw.Write([]byte("baz"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	dst := t.TempDir()
	require.NoError(t, extractTar(dst, &buf))

	got, err := os.ReadFile(filepath.Join(dst, "..foo"))
	require.NoError(t, err)
	assert.Equal(t, "baz", string(got))
}

// TestServiceUploadDownloadDirectory exercises the service tar path end to
// end: a local directory is uploaded as a tar stream, then the same archive is
// served back on download and extracted.
func TestServiceUploadDownloadDirectory(t *testing.T) {
	const sandboxID = "sbx-abc123"

	var uploadedTar []byte
	var uploadedType string

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/upload/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/upload", Method: http.MethodPut})
	})
	mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadedType = r.Header.Get("Content-Type")
		uploadedTar, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/download/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/download", Method: http.MethodGet})
	})
	mux.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", FileContentTypeGzip)
		_, _ = w.Write(uploadedTar)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	svc := NewService(newTestRepo(t, srv.URL+"/v1/", "api-key-xyz"))

	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "d"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(src, "d", "x.txt"), []byte("content-x"), 0o640))

	require.NoError(t, svc.Upload(context.Background(), sandboxID, src, "/app/src"))
	assert.Equal(t, FileContentTypeTar, uploadedType, "a directory uploads as an x-tar archive")
	require.NotEmpty(t, uploadedTar)
	assert.True(t, uploadedTar[0] == 0x1f && uploadedTar[1] == 0x8b, "upload body must be gzipped")

	dst := t.TempDir()
	written, err := svc.Download(context.Background(), sandboxID, "/app/src", dst)
	require.NoError(t, err)
	assert.Equal(t, dst, written)

	got, err := os.ReadFile(filepath.Join(dst, "d", "x.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content-x", string(got))
}

func TestServiceDownloadFileIntoDirectory(t *testing.T) {
	const sandboxID = "sbx-abc123"

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/download/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/download", Method: http.MethodGet})
	})
	mux.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", FileContentTypeOctetStream)
		w.Header().Set("Content-Disposition", `attachment; filename="out.json"`)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	svc := NewService(newTestRepo(t, srv.URL+"/v1/", "api-key-xyz"))

	// Destination is an existing directory: the server-suggested filename lands inside it.
	dst := t.TempDir()
	written, err := svc.Download(context.Background(), sandboxID, "/app/out.json", dst)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dst, "out.json"), written)

	got, err := os.ReadFile(written)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(got))
}

type uploadedRequest struct {
	contentType      string
	contentEncoding  string
	contentLength    int64
	transferEncoding []string
	body             []byte
}

// serveUpload stands up a mint + upload pair recording the transfer request, and
// returns a Repo pointed at it.
func serveUpload(t *testing.T, sandboxID string, got *uploadedRequest) *Repo {
	t.Helper()

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/upload/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/upload", Method: http.MethodPut})
	})
	mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		got.contentType = r.Header.Get("Content-Type")
		got.contentEncoding = r.Header.Get("Content-Encoding")
		got.contentLength = r.ContentLength
		got.transferEncoding = r.TransferEncoding
		got.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	return newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")
}

// A single file has a known size, so it uploads with a real Content-Length
// rather than the chunked stream a directory archive needs.
func TestServiceUploadFileSendsLength(t *testing.T) {
	const sandboxID = "sbx-abc123"

	var got uploadedRequest
	svc := NewService(serveUpload(t, sandboxID, &got))

	const content = "hello, sandbox"
	src := filepath.Join(t.TempDir(), "data.txt")
	require.NoError(t, os.WriteFile(src, []byte(content), 0o640))

	require.NoError(t, svc.Upload(context.Background(), sandboxID, src, "/app/data.txt"))

	assert.Equal(t, FileContentTypeOctetStream, got.contentType)
	assert.Equal(t, int64(len(content)), got.contentLength)
	assert.Empty(t, got.transferEncoding)
	assert.Equal(t, content, string(got.body))
}

func TestServiceUploadDirectoryStreamsChunked(t *testing.T) {
	const sandboxID = "sbx-abc123"

	var got uploadedRequest
	svc := NewService(serveUpload(t, sandboxID, &got))

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "x.txt"), []byte("x"), 0o640))

	require.NoError(t, svc.Upload(context.Background(), sandboxID, src, "/app/src"))

	assert.Equal(t, FileContentTypeTar, got.contentType)
	assert.Equal(t, "gzip", got.contentEncoding, "compressed on the wire via Content-Encoding")
	assert.Equal(t, int64(-1), got.contentLength, "a tar stream has no length to declare")
	assert.Contains(t, got.transferEncoding, "chunked")
}

// serveDownloadHandler stands up a mint endpoint pointing at the given download
// handler, and returns a Repo pointed at it.
func serveDownloadHandler(t *testing.T, sandboxID string, download http.HandlerFunc) *Repo {
	t.Helper()

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/"+sandboxID+"/files/download/token", func(w http.ResponseWriter, r *http.Request) {
		writeConnectResponse(w, sandboxclient.SandboxConnectResponse{Token: "tok", Uri: serverURL + "/files/download", Method: http.MethodGet})
	})
	mux.HandleFunc("/files/download", download)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	serverURL = srv.URL

	t.Setenv("RENDER_WORKSPACE", "tea-workspace")

	return newTestRepo(t, srv.URL+"/v1/", "api-key-xyz")
}

// serveDownload serves a complete response with the given Content-Type,
// Content-Disposition, and body.
func serveDownload(t *testing.T, sandboxID, contentType, disposition string, body []byte) *Repo {
	t.Helper()

	return serveDownloadHandler(t, sandboxID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		if disposition != "" {
			w.Header().Set("Content-Disposition", disposition)
		}
		_, _ = w.Write(body)
	})
}

// An explicit destination wins over the server-suggested filename, which only
// supplies a name when the destination is a directory.
func TestServiceDownloadFileToExplicitPath(t *testing.T) {
	const sandboxID = "sbx-abc123"

	svc := NewService(serveDownload(t, sandboxID, FileContentTypeOctetStream, `attachment; filename="server-name.json"`, []byte(`{"ok":true}`)))

	dest := filepath.Join(t.TempDir(), "chosen.json")
	written, err := svc.Download(context.Background(), sandboxID, "/app/out.json", dest)
	require.NoError(t, err)
	assert.Equal(t, dest, written)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(got))
}

// A transfer cut off mid-body must leave no truncated file passing for a
// complete one, and must not take out whatever was already at the destination.
func TestServiceDownloadInterruptedLeavesDestinationIntact(t *testing.T) {
	const sandboxID = "sbx-abc123"

	svc := NewService(serveDownloadHandler(t, sandboxID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", FileContentTypeOctetStream)
		w.Header().Set("Content-Length", "4096") // never delivered in full
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("trunc"))
	}))

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o640))

	_, err := svc.Download(context.Background(), sandboxID, "/app/out.bin", dest)
	require.Error(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "original", string(got), "a failed download must not replace the existing file")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no partial file may be left behind: %v", entries)
}

// The file a download streams into before renaming is named uniquely, so an
// unrelated file that happens to sit at the old fixed name is left alone.
func TestServiceDownloadKeepsUnrelatedPartialFile(t *testing.T) {
	const sandboxID = "sbx-abc123"

	svc := NewService(serveDownload(t, sandboxID, FileContentTypeOctetStream, "", []byte("downloaded")))

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	bystander := dest + ".render-partial"
	require.NoError(t, os.WriteFile(bystander, []byte("someone else's data"), 0o640))

	written, err := svc.Download(context.Background(), sandboxID, "/app/out.bin", dest)
	require.NoError(t, err)
	assert.Equal(t, dest, written)

	got, err := os.ReadFile(bystander)
	require.NoError(t, err)
	assert.Equal(t, "someone else's data", string(got), "the download must not write through an unrelated file")
}

// A gzipped archive truncated after the end-of-archive marker but before the
// gzip trailer extracts every entry cleanly, so only reading to EOF catches it.
func TestServiceDownloadDetectsTruncatedArchive(t *testing.T) {
	const sandboxID = "sbx-abc123"

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "x.txt"), []byte("content-x"), 0o640))

	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	require.NoError(t, writeTar(gz, src))
	require.NoError(t, gz.Close())

	// Drop the CRC32/ISIZE trailer, leaving the tar entries and the
	// end-of-archive marker intact.
	truncated := archive.Bytes()[:archive.Len()-8]

	svc := NewService(serveDownload(t, sandboxID, FileContentTypeGzip, "", truncated))

	dst := t.TempDir()
	_, err := svc.Download(context.Background(), sandboxID, "/app/src", dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify archive")
}

// Nothing in the Content-Disposition path is trustworthy: the header is set by
// the sandbox, so a filename of "../../x" must not steer the write out of the
// destination directory.
func TestServiceDownloadRejectsEscapingFilename(t *testing.T) {
	const sandboxID = "sbx-abc123"

	for _, filename := range []string{"../../pwned.txt", "../pwned.txt", "/etc/pwned.txt", "sub/pwned.txt", ".."} {
		t.Run(filename, func(t *testing.T) {
			svc := NewService(serveDownload(t, sandboxID, FileContentTypeOctetStream,
				fmt.Sprintf("attachment; filename=%q", filename), []byte("pwned")))

			base := t.TempDir()
			dst := filepath.Join(base, "a", "b")
			require.NoError(t, os.MkdirAll(dst, 0o750))

			written, err := svc.Download(context.Background(), sandboxID, "/app/out.txt", dst)
			if err == nil {
				assert.Equal(t, dst, filepath.Dir(written), "write must stay in the destination directory")
			}

			for _, dir := range []string{base, filepath.Join(base, "a")} {
				entries, readErr := os.ReadDir(dir)
				require.NoError(t, readErr)
				for _, entry := range entries {
					assert.NotContains(t, entry.Name(), "pwned", "wrote outside the destination at %s", filepath.Join(dir, entry.Name()))
				}
			}
		})
	}
}

// An absent or unusable Content-Disposition falls back to the remote basename.
func TestServiceDownloadFilenameFallback(t *testing.T) {
	const sandboxID = "sbx-abc123"

	for name, disposition := range map[string]string{
		"no header":         "",
		"no filename":       "attachment",
		"unusable filename": `attachment; filename=".."`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := NewService(serveDownload(t, sandboxID, FileContentTypeOctetStream, disposition, []byte("body")))

			dst := t.TempDir()
			written, err := svc.Download(context.Background(), sandboxID, "/app/out.txt", dst)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(dst, "out.txt"), written)
		})
	}
}
