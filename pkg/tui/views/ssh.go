package views

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/deploy"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui"
)

type SSHInput struct {
	ServiceIDOrName string `cli:"arg:0"`
	InstanceID      string // Set when a specific instance is selected or provided
	Project         *client.Project
	EnvironmentIDs  []string

	Args []string
}

// NewSSHView creates an SSH execution view - always returns ExecModel
func NewSSHView(ctx context.Context, input *SSHInput) *tui.ExecModel {
	return tui.NewExecModel("ssh", handleSSHError, command.LoadCmd(ctx, loadDataSSH, input))
}

func handleSSHError(err error) error {
	return tui.UserFacingError{
		Title:   "Failed to SSH",
		Message: fmt.Sprintf("Check the docs (https://render.com/docs/ssh) to ensure SSH is properly configured: %s", err),
	}
}

func getServiceFromIDOrName(ctx context.Context, c *client.ClientWithResponses, idOrName string) (*client.Service, error) {
	serviceRepo := service.NewRepo(c)

	if matchesServiceId(idOrName) || matchesCronJobId(idOrName) {
		// We can't easily disambiguate between an ID and a name (since technically a name could be
		// a valid ID), so we'll prefer the ID if it's valid.
		service, err := serviceRepo.GetService(ctx, idOrName)
		if err == nil {
			return service, nil
		}
	}

	services, err := serviceRepo.ListServices(ctx, &client.ListServicesParams{
		Name: &client.NameParam{idOrName},
	})

	if err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, tui.UserFacingError{
			Title: "Failed to SSH", Message: fmt.Sprintf("No service found with name or ID '%s'", idOrName),
		}
	}
	if len(services) > 1 {
		return nil, tui.UserFacingError{
			Title: "Failed to SSH", Message: fmt.Sprintf("Multiple services found with name '%s'. Please specify the service ID instead.", idOrName),
		}
	}
	return services[0], nil
}

func loadDataSSH(ctx context.Context, in *SSHInput) (*exec.Cmd, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	serviceInfo, err := getServiceFromIDOrName(ctx, c, in.ServiceIDOrName)
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

	deploys, err := deploy.NewRepo(c).ListDeploysForService(ctx, serviceInfo.Id, &client.ListDeploysParams{})
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

	// Modify SSH address to use specific instance if selected
	finalSSHAddress := *sshAddress
	if in.InstanceID != "" {
		// Replace the user part of the SSH address with the instance ID
		// From: srv-123@hostname -> srv-123-asdf@hostname
		parts := strings.SplitN(finalSSHAddress, "@", 2)
		if len(parts) == 2 {
			finalSSHAddress = in.InstanceID + "@" + parts[1]
		}
	}

	args := []string{finalSSHAddress}
	args = append(args, in.Args...)

	return exec.Command("ssh", args...), nil
}
