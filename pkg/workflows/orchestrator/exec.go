package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/render-oss/cli/pkg/proctree"
	"github.com/render-oss/cli/pkg/workflows/logs"
)

type Exec struct {
	logsStore *logs.LogStore
	command   string
	args      []string
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

func NewExec(logsStore *logs.LogStore, command string, args ...string) *Exec {
	return &Exec{
		logsStore: logsStore,
		command:   command,
		args:      args,
	}
}

func (e *Exec) StartService(ctx context.Context, taskRunID string, socketPath string, mode Mode) (CleanupFunc, error) {
	cmd := exec.CommandContext(ctx, e.command, e.args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf(SocketPathEnv+"=%s", socketPath), fmt.Sprintf(ModeEnv+"=%s", mode))

	stdOutInterceptor := logs.NewLogInterceptor(taskRunID, os.Stdout, e.logsStore)
	stdErrInterceptor := logs.NewLogInterceptor(taskRunID, os.Stderr, e.logsStore)

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
