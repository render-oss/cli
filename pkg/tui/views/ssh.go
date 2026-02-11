package views

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/deploy"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui"
)

type SSHInput struct {
	ServiceIDOrName string `cli:"arg:0"`
	InstanceID      string // Set when a specific instance is selected or provided
	Project         *client.Project
	EnvironmentIDs  []string
	Ephemeral       bool `cli:"ephemeral"`

	Args []string
}

// NewSSHView creates an SSH execution view - always returns ExecModel
func NewSSHView(ctx context.Context, input *SSHInput) *tui.ExecModel {
	loadingMsg := "%s Preparing SSH connection..."
	if input.Ephemeral {
		loadingMsg = "%s Creating ephemeral shell (this may take a moment)..."
	}
	return tui.NewExecModel("ssh", handleSSHError, command.LoadCmdWithLoadingMsg(ctx, loadDataSSH, input, loadingMsg))
}

func handleSSHError(err error) error {
	return tui.UserFacingError{
		Title:   "Failed to SSH",
		Message: fmt.Sprintf("Check the docs (https://render.com/docs/ssh) to ensure SSH is properly configured: %s", err),
	}
}

// createEphemeralShell creates an ephemeral shell pod for the given service.
// This must be called before SSH'ing into an ephemeral shell.
func createEphemeralShell(ctx context.Context, serviceID string) error {
	apiCfg, err := config.DefaultAPIConfig()
	if err != nil {
		return fmt.Errorf("failed to get API config: %w", err)
	}

	url := fmt.Sprintf("%sservices/%s/ephemeral-shell", apiCfg.Host, serviceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(nil))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header = client.AddHeaders(req.Header, apiCfg.Key)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create ephemeral shell: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to extract error message from JSON response
		var apiErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
			return fmt.Errorf("failed to create ephemeral shell: %s", apiErr.Message)
		}
		return fmt.Errorf("failed to create ephemeral shell: received status %d", resp.StatusCode)
	}

	return nil
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

	if in.Ephemeral {
		// Create the ephemeral shell pod first
		if err := createEphemeralShell(ctx, serviceInfo.Id); err != nil {
			return nil, tui.UserFacingError{
				Title:   "Failed to create ephemeral shell",
				Message: err.Error(),
			}
		}

		// Format: ephemeral.{service-id}@{ssh-host}
		parts := strings.SplitN(finalSSHAddress, "@", 2)
		if len(parts) == 2 {
			finalSSHAddress = "ephemeral." + serviceInfo.Id + "@" + parts[1]
		}
	}

	args := []string{finalSSHAddress}
	args = append(args, in.Args...)

	return exec.Command("ssh", args...), nil
}
