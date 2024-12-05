package views

import (
	"context"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/deploy"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/service"
	"github.com/renderinc/cli/pkg/tui"
)

type SSHInput struct {
	ServiceID      string `cli:"arg:0"`
	Project        *client.Project
	EnvironmentIDs []string

	Args []string
}

type SSHView struct {
	serviceTable *ServiceList
	execModel    *tui.ExecModel
}

func NewSSHView(ctx context.Context, input *SSHInput, opts ...tui.TableOption[*service.Model]) *SSHView {
	sshView := &SSHView{
		execModel: tui.NewExecModel(command.LoadCmd(ctx, loadDataSSH, input)),
	}

	serviceListInput := ServiceInput{
		Project:        input.Project,
		EnvironmentIDs: input.EnvironmentIDs,
		Types:          []client.ServiceType{client.WebService, client.PrivateService, client.BackgroundWorker},
	}

	if input.ServiceID == "" {
		// If a flag or temporary input is provided, that should take precedence. Only get the persistent filter
		// if no input is provided.
		if len(input.EnvironmentIDs) == 0 {
			defaultInput, err := DefaultListResourceInput(ctx)
			if err != nil {
				return &SSHView{
					execModel: tui.NewExecModel(command.LoadCmd(ctx, func(_ context.Context, _ any) (*exec.Cmd, error) {
						return nil, fmt.Errorf("failed to load default project filter: %w", err)
					}, nil)),
				}
			}

			serviceListInput.Project = defaultInput.Project
			serviceListInput.EnvironmentIDs = defaultInput.EnvironmentIDs
		}

		if serviceListInput.Project != nil {
			opts = append(opts, tui.WithHeader[*service.Model](
				fmt.Sprintf("Project: %s", serviceListInput.Project.Name),
			))
		}

		sshView.serviceTable = NewServiceList(ctx, serviceListInput, func(ctx context.Context, r resource.Resource) tea.Cmd {
			return tea.Sequence(
				func() tea.Msg {
					input.ServiceID = r.ID()
					sshView.serviceTable = nil
					return nil
				}, sshView.execModel.Init())
		}, opts...)
	}
	return sshView
}

func loadDataSSH(ctx context.Context, in *SSHInput) (*exec.Cmd, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	serviceInfo, err := service.NewRepo(c).GetService(ctx, in.ServiceID)
	if err != nil {
		return nil, err
	}

	var sshAddress *string
	if details, err := serviceInfo.ServiceDetails.AsWebServiceDetails(); err == nil {
		sshAddress = details.SshAddress
	} else if details, err := serviceInfo.ServiceDetails.AsPrivateServiceDetails(); err == nil {
		sshAddress = details.SshAddress
	} else if details, err := serviceInfo.ServiceDetails.AsBackgroundWorkerDetails(); err == nil {
		sshAddress = details.SshAddress
	} else {
		return nil, tui.UserFacingError{
			Title: "Failed to SSH", Message: fmt.Sprintf("Cannot SSH into %s service type.", serviceInfo.Type),
		}
	}

	if serviceInfo.Suspended == client.ServiceSuspendedSuspended {
		return nil, tui.UserFacingError{Title: "Failed to SSH", Message: "Cannot SSH into a suspended service."}
	}

	deploys, err := deploy.NewRepo(c).ListDeploysForService(ctx, in.ServiceID, &client.ListDeploysParams{})
	if err != nil {
		return nil, err
	}

	foundLiveDeploy := false
	for _, deploy := range deploys {
		if deploy.Status != nil && *deploy.Status == client.DeployStatusLive {
			foundLiveDeploy = true
			break
		}
	}

	if !foundLiveDeploy {
		return nil, tui.UserFacingError{
			Title: "Failed to SSH", Message: "Cannot SSH into a service with no live deploys.",
		}
	}

	if sshAddress == nil {
		return nil, fmt.Errorf("service does not support ssh")
	}

	args := []string{*sshAddress}
	for _, arg := range in.Args {
		args = append(args, arg)
	}

	return exec.Command("ssh", args...), nil
}

func (v *SSHView) Init() tea.Cmd {
	if v.serviceTable != nil {
		return v.serviceTable.Init()
	}

	return v.execModel.Init()
}

func (v *SSHView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if v.serviceTable != nil {
		_, cmd = v.serviceTable.Update(msg)
	} else {
		_, cmd = v.execModel.Update(msg)
	}

	return v, cmd
}

func (v *SSHView) View() string {
	if v.serviceTable != nil {
		return v.serviceTable.View()
	}

	return v.execModel.View()
}
