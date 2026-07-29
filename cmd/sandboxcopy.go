package cmd

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
)

// sandboxRemoteArg matches an scp-style remote argument: a sandbox ID, a
// colon, and an absolute path (e.g. sbx-abc123:/app/main.py).
var sandboxRemoteArg = regexp.MustCompile(`^(sbx-[A-Za-z0-9]+):(.*)$`)

type sandboxCopyEndpoint struct {
	sandboxID string // empty for a local endpoint
	path      string
}

func parseSandboxCopyArgs(src, dst string) (from, to sandboxCopyEndpoint, err error) {
	from = parseSandboxCopyEndpoint(src)
	to = parseSandboxCopyEndpoint(dst)

	switch {
	case from.sandboxID != "" && to.sandboxID != "":
		return from, to, fmt.Errorf("copying between sandboxes is not supported; one of <src> and <dst> must be a local path")
	case from.sandboxID == "" && to.sandboxID == "":
		return from, to, fmt.Errorf("one of <src> and <dst> must be a sandbox path like sbx-abc123:/app/file")
	}

	remote := &from
	if to.sandboxID != "" {
		remote = &to
	}
	// scp semantics: a relative sandbox path resolves under the sandbox home
	// directory, an absolute one addresses the filesystem root, and an empty
	// one ("sbx-abc:") is the home directory itself.
	remote.path = path.Clean(remote.path)
	if remote.path == ".." || strings.HasPrefix(remote.path, "../") {
		return from, to, fmt.Errorf("sandbox path must not escape the home directory, got %q", remote.path)
	}
	return from, to, nil
}

func parseSandboxCopyEndpoint(arg string) sandboxCopyEndpoint {
	if m := sandboxRemoteArg.FindStringSubmatch(arg); m != nil {
		return sandboxCopyEndpoint{sandboxID: m[1], path: m[2]}
	}
	return sandboxCopyEndpoint{path: arg}
}

func newSandboxCopyCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy <src> <dst>",
		Aliases: []string{"cp"},
		Short:   "Copy files to or from a sandbox",
		Long: `Copy a file or directory between the local filesystem and a running sandbox.

Prefix the remote side with the sandbox ID and a colon, like scp. A relative
sandbox path resolves inside the sandbox's home directory; an absolute path
addresses the sandbox filesystem root. Directories are transferred as
archives: uploading a local directory recreates it at the sandbox path, and
downloading a sandbox directory recreates it under the local path. "cp" works
as an alias.

Unlike cp, a directory destination is not nested into: copying ./src to
sbx-abc123:/tmp/src puts src's contents at /tmp/src, not at /tmp/src/src. A
single file copied to an existing local directory does land inside it.

Examples:
  render ea sandboxes copy ./main.py sbx-abc123:main.py
  render ea sandboxes copy sbx-abc123:output.json ./output.json
  render ea sandboxes copy ./src sbx-abc123:/tmp/src
  render ea sandboxes cp sbx-abc123:. ./sandbox-home
  render ea sandboxes copy sbx-abc123:output.json ./downloads/ --output json
`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		from, to, err := parseSandboxCopyArgs(args[0], args[1])
		if err != nil {
			return err
		}

		loadData := func() (*sandbox.CopyOut, error) {
			svc := deps.SandboxService()
			if to.sandboxID != "" {
				if err := svc.Upload(cmd.Context(), to.sandboxID, from.path, to.path); err != nil {
					return nil, err
				}
				return &sandbox.CopyOut{Data: sandbox.CopyOutData{
					SandboxID:  to.sandboxID,
					Direction:  sandbox.CopyDirectionUpload,
					LocalPath:  from.path,
					RemotePath: to.path,
				}}, nil
			}

			// Download resolves the local path it wrote, which is the one worth
			// reporting: a directory destination takes the name from the sandbox.
			localPath, err := svc.Download(cmd.Context(), from.sandboxID, from.path, to.path)
			if err != nil {
				return nil, err
			}
			return &sandbox.CopyOut{Data: sandbox.CopyOutData{
				SandboxID:  from.sandboxID,
				Direction:  sandbox.CopyDirectionDownload,
				LocalPath:  localPath,
				RemotePath: from.path,
			}}, nil
		}

		_, err = command.NonInteractive(cmd, loadData, sandboxCopyTextOutput)
		return err
	}

	return cmd
}

func sandboxCopyTextOutput(r *sandbox.CopyOut) string {
	if r.Data.Direction == sandbox.CopyDirectionUpload {
		return fmt.Sprintf("Uploaded %s to %s:%s\n", r.Data.LocalPath, r.Data.SandboxID, r.Data.RemotePath)
	}
	return fmt.Sprintf("Downloaded %s:%s to %s\n", r.Data.SandboxID, r.Data.RemotePath, r.Data.LocalPath)
}
