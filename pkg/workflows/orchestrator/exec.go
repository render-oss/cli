package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/render-oss/cli/pkg/proctree"
	"github.com/render-oss/cli/pkg/workflows/logs"
)

type Exec struct {
	logsStore *logs.LogStore
	command   string
	args      []string
	debug     bool
}

type Mode string

const (
	ModeRun      Mode = "run"
	ModeRegister Mode = "register"
)

const (
	SocketPathEnv = "RENDER_SDK_SOCKET_PATH"
	ModeEnv       = "RENDER_SDK_MODE"
)

type CleanupFunc func() error

func NewExec(logsStore *logs.LogStore, debug bool, command string, args ...string) *Exec {
	return &Exec{
		logsStore: logsStore,
		command:   command,
		args:      args,
		debug:     debug,
	}
}

func (e *Exec) StartService(ctx context.Context, taskRunID string, socketPath string, mode Mode) (CleanupFunc, error) {
	cmd := exec.CommandContext(ctx, e.command, e.args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf(SocketPathEnv+"=%s", socketPath), fmt.Sprintf(ModeEnv+"=%s", mode))

	stdoutWriter := io.Writer(os.Stdout)
	stderrWriter := io.Writer(os.Stderr)
	if !e.debug {
		stdoutWriter = io.Discard
		stderrWriter = io.Discard
	}
	stdOutInterceptor := logs.NewLogInterceptor(taskRunID, stdoutWriter, e.logsStore)
	stdErrInterceptor := logs.NewLogInterceptor(taskRunID, stderrWriter, e.logsStore)

	cmd.Stdout = stdOutInterceptor
	cmd.Stderr = stdErrInterceptor

	pt := proctree.New(cmd)

	go func() {
		<-ctx.Done()
		pt.Kill()
	}()

	return func() error {
		return pt.Kill()
	}, cmd.Start()
}
