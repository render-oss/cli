package helpers

import (
	"io"
	"os"

	"github.com/render-oss/cli/cmd"
)

func RunCommand(args []string) io.Reader {
	reader, writer := io.Pipe()
	cmd.RootCmd.SetOut(io.MultiWriter(writer, os.Stdout))
	cmd.RootCmd.SetErr(io.MultiWriter(writer, os.Stderr))
	cmd.RootCmd.SetArgs(args)

	go func() {
		defer writer.Close()
		cmd.Execute()
	}()

	return reader
}
