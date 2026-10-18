package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

const (
	sshTestServiceID   = "srv-12345678901234567890"
	sshTestWorkspaceID = "tea-12345678901234567890"
)

func newSSHTestServer(t *testing.T) *renderapi.Server {
	t.Helper()
	server := renderapi.NewServer(t)
	svc := server.Services.Add(renderapi.NewWebService(renderapi.WebServiceAttrs{
		Service: renderapi.CommonServiceAttrs{ID: sshTestServiceID, Name: "test-service", OwnerID: sshTestWorkspaceID},
	}))
	details, err := svc.ServiceDetails.AsWebServiceDetails()
	require.NoError(t, err)
	details.SshAddress = pointers.From(sshTestServiceID + "@ssh.oregon.render.com")
	require.NoError(t, svc.ServiceDetails.FromWebServiceDetails(details))
	server.Deploys.Add(renderapi.NewDeploy(sshTestServiceID, client.Deploy{Status: pointers.From(client.DeployStatusLive)}))
	return server
}

// fakeSSH puts a fake ssh on PATH. The fake prints its arguments and stdin to
// stdout, prints a line to stderr, and then runs the shell command in last.
func fakeSSH(t *testing.T, last string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake SSH process requires a POSIX shell")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '<%s>\\n' \"$@\"\ncat\nprintf 'ssh stderr\\n' >&2\n" + last + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func executeSSHCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_HOST", server.URL()+"/")
	t.Setenv("RENDER_WORKSPACE", sshTestWorkspaceID)
	t.Setenv("RENDER_OUTPUT", "")
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{}, nil
	}
	root := newRootCmd()
	setupSSHCommands(root, deps)
	setupRootCmdPersistentRun(root, deps)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader("ssh stdin\n"))
	root.SetArgs(append([]string{"ssh"}, args...))
	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

func TestSSHNonInteractive(t *testing.T) {
	tests := []struct {
		name   string
		target string
		output string
		user   string
	}{
		{name: "non-TTY default", target: sshTestServiceID, user: sshTestServiceID},
		{name: "service name", target: "test-service", output: "text", user: sshTestServiceID},
		{name: "instance ID", target: sshTestServiceID + "-abc12", output: "text", user: sshTestServiceID + "-abc12"},
		{name: "json streams raw output", target: sshTestServiceID, output: "json", user: sshTestServiceID},
		{name: "yaml streams raw output", target: sshTestServiceID, output: "yaml", user: sshTestServiceID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newSSHTestServer(t)
			fakeSSH(t, "exit 0")
			args := []string{tt.target}
			if tt.output != "" {
				args = append(args, "--output", tt.output)
			}
			args = append(args, "--", "-o", "BatchMode=yes", "echo hello; printf '%s' \"$HOME\"", "", "--help")
			result, err := executeSSHCommand(t, server, args...)
			require.NoError(t, err)
			require.Equal(t, "<"+tt.user+"@ssh.oregon.render.com>\n<-o>\n<BatchMode=yes>\n<echo hello; printf '%s' \"$HOME\">\n<>\n<--help>\nssh stdin\n", result.Stdout)
			require.Equal(t, "ssh stderr\n", result.Stderr)
			require.True(t, server.HasRequest("GET", "/services/"+sshTestServiceID+"/deploys"))
			require.False(t, server.HasRequest("GET", "/instances"))
		})
	}
}

func TestSSHExitCode(t *testing.T) {
	tests := []struct {
		name     string
		last     string
		exitCode int
	}{
		{name: "remote command failed", last: "exit 7", exitCode: 7},
		{name: "ssh connection failed", last: "exit 255", exitCode: 255},
		{name: "signal maps to 128 plus signal", last: "kill -TERM $$", exitCode: 143},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newSSHTestServer(t)
			fakeSSH(t, tt.last)
			result, err := executeSSHCommand(t, server, sshTestServiceID, "--", "false")
			var exitCoder command.ExitCoder
			require.ErrorAs(t, err, &exitCoder)
			require.Equal(t, tt.exitCode, exitCoder.ExitCode())
			require.Equal(t, tt.exitCode, exitCodeFromError(err))
			require.Equal(t, "ssh stderr\n", result.Stderr)
		})
	}
}

func TestSSHRejectsArgumentsBeforeDash(t *testing.T) {
	server := newSSHTestServer(t)
	_, err := executeSSHCommand(t, server, sshTestServiceID, "echo hello")
	require.EqualError(t, err, `unexpected argument "echo hello": put SSH arguments after --`)
	require.Empty(t, server.Requests)
}

func TestSSHRequiresTarget(t *testing.T) {
	for _, args := range [][]string{nil, {"--", "echo hello"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			server := newSSHTestServer(t)
			_, err := executeSSHCommand(t, server, args...)
			require.EqualError(t, err, "service ID, service name, or instance ID is required in non-interactive mode")
			require.Empty(t, server.Requests)
		})
	}
}

func TestSSHRequiresLiveDeploy(t *testing.T) {
	server := newSSHTestServer(t)
	server.Deploys.Instances = nil
	result, err := executeSSHCommand(t, server, sshTestServiceID, "--", "echo hello")
	require.ErrorContains(t, err, "Cannot SSH into a service with no live deploys")
	require.Empty(t, result.Stdout)
}

func TestSSHSelectedInstance(t *testing.T) {
	for _, instanceID := range []string{"", sshTestServiceID + "-abc12"} {
		t.Run(instanceID, func(t *testing.T) {
			stack := tui.NewStack()
			ctx := tui.SetStackInContext(context.Background(), stack)
			input := &views.SSHInput{ServiceIDOrName: sshTestServiceID}
			originalView := InteractiveSSHView
			t.Cleanup(func() { InteractiveSSHView = originalView })
			selected := false
			InteractiveSSHView = func(ctx context.Context, input *views.SSHInput, breadcrumb string) tea.Cmd {
				selected = true
				require.Equal(t, instanceID, input.InstanceID)
				require.Equal(t, sshTestServiceID, input.ServiceIDOrName)
				return nil
			}
			interactiveSSHView(ctx, newSSHCmd(), input, "SSH")
			picker := stack.Pop()
			require.NotNil(t, picker)
			_, _ = picker.Model.Update(tui.LoadDataMsg[[]views.SSHInstanceOption]{
				Data: []views.SSHInstanceOption{{ID: instanceID, Name: "test instance"}},
			})
			_, _ = picker.Model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			require.True(t, selected)
		})
	}
}
