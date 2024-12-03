package views

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/deploy"
	"github.com/renderinc/cli/pkg/pointers"
	"github.com/renderinc/cli/pkg/service"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/types"
)

const deployTimeout = time.Hour

func CreateDeploy(ctx context.Context, input types.DeployInput) (*client.Deploy, error) {
	deployRepo, err := newDeployRepo()
	if err != nil {
		return nil, err
	}

	if input.CommitID != nil && *input.CommitID == "" {
		input.CommitID = nil
	}

	if input.ImageURL != nil && *input.ImageURL == "" {
		input.ImageURL = nil
	}

	d, err := deployRepo.TriggerDeploy(ctx, input.ServiceID, deploy.TriggerDeployInput{
		ClearCache: &input.ClearCache,
		CommitId:   input.CommitID,
		ImageUrl:   input.ImageURL,
	})
	if err != nil {
		return nil, err
	}

	return d, nil
}

func newDeployRepo() (*deploy.Repo, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	deployRepo := deploy.NewRepo(c)
	return deployRepo, nil
}

func WaitForDeploy(ctx context.Context, serviceID, deployID string) (*client.Deploy, error) {
	deployRepo, err := newDeployRepo()
	if err != nil {
		return nil, err
	}

	timeoutTimer := time.NewTimer(deployTimeout)

	for {
		select {
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("timed out waiting for deploy to finish")
		default:
			d, err := deployRepo.GetDeploy(ctx, serviceID, deployID)
			if err != nil {
				return nil, err
			}

			if deploy.IsComplete(d.Status) {
				return d, nil
			}

			if d.Status == nil || *d.Status == client.DeployStatusCreated {
				time.Sleep(10 * time.Second)
			} else {
				// if the deploy has started, poll more frequently
				time.Sleep(5 * time.Second)
			}
		}
	}
}

type DeployCreateView struct {
	formAction *tui.FormWithAction[*client.Deploy]

	ctx    context.Context
	input  types.DeployInput
	logCmd func(d *client.Deploy) tea.Cmd
}

func NewDeployCreateView(ctx context.Context, input types.DeployInput, logCmd func(d *client.Deploy) tea.Cmd) *DeployCreateView {
	return &DeployCreateView{
		ctx:    ctx,
		input:  input,
		logCmd: logCmd,
	}
}

func DeployCreateConfirm(ctx context.Context, input types.DeployInput) func() (string, error) {
	return func() (string, error) {
		c, err := client.NewDefaultClient()
		if err != nil {
			return "", fmt.Errorf("failed to create client: %w", err)
		}
		serviceRepo := service.NewRepo(c)
		svc, err := serviceRepo.GetService(ctx, input.ServiceID)
		if err != nil {
			return "", fmt.Errorf("failed to get service: %w", err)
		}

		return fmt.Sprintf("Are you sure you want to deploy %s?", svc.Name), nil
	}
}

func (v *DeployCreateView) setupForm() tea.Cmd {
	c, err := client.NewDefaultClient()
	if err != nil {
		return func() tea.Msg { return tui.ErrorMsg{Err: fmt.Errorf("failed to create client: %w", err)} }
	}

	serviceRepo := service.NewRepo(c)
	svc, err := serviceRepo.GetService(v.ctx, v.input.ServiceID)
	if err != nil {
		return func() tea.Msg { return tui.ErrorMsg{Err: fmt.Errorf("failed to get service: %w", err)} }
	}

	var inputs []huh.Field
	if svc.ImagePath != nil {
		if v.input.ImageURL == nil {
			v.input.ImageURL = pointers.From("")
		}

		inputs = append(inputs, huh.NewInput().
			Title("Image URL").
			Placeholder("Enter Docker image URL (optional)").
			Value(v.input.ImageURL))
	} else {
		if v.input.CommitID == nil {
			v.input.CommitID = pointers.From("")
		}

		inputs = append(inputs, huh.NewInput().
			Title("Commit ID").
			Placeholder("Enter commit ID (optional)").
			Value(v.input.CommitID))
	}

	deployForm := huh.NewForm(huh.NewGroup(inputs...))

	action := tui.NewFormAction(
		v.logCmd,
		command.WrapInConfirm(command.LoadCmd(v.ctx, CreateDeploy, v.input), DeployCreateConfirm(v.ctx, v.input)),
	)

	v.formAction = tui.NewFormWithAction(action, deployForm)

	return v.formAction.Init()
}

func (v *DeployCreateView) Init() tea.Cmd {
	return v.setupForm()
}

func (v *DeployCreateView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v.formAction.Update(msg)
}

func (v *DeployCreateView) View() string {
	return v.formAction.View()
}
