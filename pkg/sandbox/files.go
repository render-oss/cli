package sandbox

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Upload copies a local file or directory into the sandbox at remotePath. A
// directory is streamed as a gzipped tar archive and extracted at remotePath;
// a file is streamed as raw bytes.
func (s *Service) Upload(ctx context.Context, id, localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		pr, pw := io.Pipe()
		// Closing the read end unblocks the tar goroutine (its writes fail with
		// io.ErrClosedPipe) if UploadFile returns before draining the archive.
		defer pr.Close()
		go func() {
			gz := gzip.NewWriter(pw)
			if err := writeTar(gz, localPath); err != nil {
				pw.CloseWithError(err)
				return
			}
			pw.CloseWithError(gz.Close())
		}()
		return s.repo.UploadFile(ctx, id, remotePath, FileContentTypeGzip, -1, pr)
	}

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.repo.UploadFile(ctx, id, remotePath, FileContentTypeOctetStream, info.Size(), f)
}

// Download copies a file or directory from the sandbox at remotePath to
// localPath, and returns the local path written. A directory arrives as a
// gzipped tar archive and is extracted under localPath; a file is written to
// localPath directly (or into it, when localPath is an existing directory).
func (s *Service) Download(ctx context.Context, id, remotePath, localPath string) (string, error) {
	stream, err := s.repo.DownloadFile(ctx, id, remotePath)
	if err != nil {
		return "", err
	}
	defer stream.Body.Close()

	if stream.ContentType == FileContentTypeGzip {
		gz, err := gzip.NewReader(stream.Body)
		if err != nil {
			return "", err
		}
		defer gz.Close()
		if err := extractTar(localPath, gz); err != nil {
			return "", err
		}
		// tar.Next stops at the end-of-archive marker, which sits before the
		// gzip CRC32/ISIZE trailer, and the trailer is what gzip checks the
		// decompressed contents against. Reading to EOF forces that check, so a
		// transfer truncated in that window doesn't extract cleanly and report
		// success.
		if _, err := io.Copy(io.Discard, gz); err != nil {
			return "", fmt.Errorf("verify archive: %w", err)
		}
		return localPath, nil
	}

	dest := localPath
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		name := baseName(stream.Filename)
		if name == "" {
			name = baseName(remotePath)
		}
		dest = filepath.Join(localPath, name)
	}
	// Stream to a sibling and rename so an interrupted transfer neither leaves a
	// truncated file looking complete at dest nor destroys what was there. The
	// sibling's name is unique, so an unrelated file that happens to share the
	// prefix survives, as does a second copy running against the same dest.
	f, err := os.CreateTemp(filepath.Dir(dest), ".render-partial-*")
	if err != nil {
		return "", err
	}
	partial := f.Name()
	// Clean up on any path that doesn't reach the rename; once renamed, both
	// calls are no-ops on a name that no longer exists.
	defer func() {
		_ = f.Close()
		_ = os.Remove(partial)
	}()

	if _, err := io.Copy(f, stream.Body); err != nil {
		return "", err
	}
	// CreateTemp opens 0o600. Widen to the 0o644 a plain create would produce
	// under a default umask, which is also what the archive path extracts with.
	if err := f.Chmod(0o644); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(partial, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// baseName reduces an untrusted name to a single path component, or "" if
// nothing usable is left. Content-Disposition filenames come from the sandbox
// and nothing in mime.ParseMediaType strips separators, so a filename of
// "../../x" would otherwise escape the destination directory.
func baseName(name string) string {
	base := filepath.Base(filepath.FromSlash(name))
	switch base {
	case ".", "..", string(filepath.Separator), "":
		return ""
	}
	return base
}

// writeTar streams the directory tree rooted at root to w as a tar archive,
// mirroring the sandbox agent's semantics: entry names are relative to root
// (which is not itself emitted) and symlinks are stored, not followed.
func writeTar(w io.Writer, root string) error {
	// WalkDir lstats root, so a symlinked directory would walk as one skipped
	// entry and produce an empty archive. Upload already followed the link with
	// Stat to choose this path, so follow it here too.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}

	tw := tar.NewWriter(w)
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		// tar.FileInfoHeader errors on sockets, so a stray one would abort the
		// whole upload. Skip what extraction skips on the way back in.
		if !fi.IsDir() && !fi.Mode().IsRegular() && fi.Mode()&fs.ModeSymlink == 0 {
			return nil
		}

		var link string
		if fi.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(p); err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if fi.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return err
	}
	return tw.Close()
}

// extractTar extracts a downloaded archive under dest. The archive comes from
// the (untrusted) sandbox, so every write goes through an os.Root confined to
// dest: it resolves each path within dest at the openat layer and refuses any
// component — including a symlink target — that escapes it. A symlink pointing
// outside dest still gets created, but a later write through it fails.
func extractTar(dest string, r io.Reader) error {
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}

		name := filepath.Clean(filepath.FromSlash(hdr.Name)) // "." for the archive's root entry
		// Masking to 0o777 drops setuid/setgid, which an archive from the sandbox
		// has no business setting locally; the or'd owner bits keep the tree
		// writable enough to finish extracting into.
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, fs.FileMode(hdr.Mode)&0o777|0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(name), 0o750); err != nil {
				return err
			}
			f, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode)&0o777|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // size bounded by what the user asked to download
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := root.MkdirAll(filepath.Dir(name), 0o750); err != nil {
				return err
			}
			_ = root.Remove(name)
			if err := root.Symlink(hdr.Linkname, name); err != nil {
				return err
			}
		case tar.TypeLink:
			// The sandbox's tar emits these for multiply-linked files, and tar
			// orders a link after its target. Root.Link resolves both names
			// inside dest, so an escaping target is refused.
			if err := root.MkdirAll(filepath.Dir(name), 0o750); err != nil {
				return err
			}
			_ = root.Remove(name)
			if err := root.Link(filepath.Clean(filepath.FromSlash(hdr.Linkname)), name); err != nil {
				return err
			}
		default:
			// Skip devices, fifos, and other special entries.
		}
	}
}
