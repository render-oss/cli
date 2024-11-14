package helpers

import (
	"io"

	"github.com/renderinc/cli/cmd"
)

func RunCommand(args []string) io.Reader {
	reader, writer := io.Pipe()
	cmd.RootCmd.SetOut(writer)
	cmd.RootCmd.SetArgs(args)

	go func() {
		defer writer.Close()
		cmd.Execute()
	}()

	return reader
}
