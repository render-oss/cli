/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh [serviceID]",
	Short: "SSH into a server",
	Long:  `SSH into a server given a service ID.`,
}
var InteractiveSSH = command.Wrap(sshCmd, loadDataSSH, renderSSH)

type SSHInput struct {
	ServiceID string
}

func (p SSHInput) String() []string {
	return []string{p.ServiceID}
}

func loadDataSSH(ctx context.Context, in SSHInput) (string, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return "", err
	}

	serviceInfo, err := service.NewRepo(c).GetService(ctx, in.ServiceID)
	if err != nil {
		return "", err
	}

	var sshAddress *string
	if details, err := serviceInfo.ServiceDetails.AsWebServiceDetails(); err == nil {
		sshAddress = details.SshAddress
	} else if details, err := serviceInfo.ServiceDetails.AsPrivateServiceDetails(); err == nil {
		sshAddress = details.SshAddress
	} else if details, err := serviceInfo.ServiceDetails.AsBackgroundWorkerDetails(); err == nil {
		sshAddress = details.SshAddress
	} else {
		return "", fmt.Errorf("unsupported service type")
	}

	if sshAddress == nil {
		return "", fmt.Errorf("service does not support ssh")
	}

	return *sshAddress, nil
}

func renderSSH(ctx context.Context, loadData func(in SSHInput) (string, error), in SSHInput) (tea.Model, error) {
	sshAddress, err := loadData(in)
	if err != nil {
		return nil, err
	}

	return tui.NewExecModel(exec.Command("ssh", sshAddress)), nil
}

func init() {
	rootCmd.AddCommand(sshCmd)

	sshCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		serviceID := args[0]

		InteractiveSSH(ctx, SSHInput{ServiceID: serviceID})

		return nil
	}
}
