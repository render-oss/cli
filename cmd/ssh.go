package cmd

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/project"
	resourcetui "github.com/renderinc/render-cli/pkg/resource/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/spf13/cobra"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh [serviceID]",
	Args:  cobra.MaximumNArgs(1),
	Short: "SSH into a server",
	Long:  `SSH into a server given a service ID. Optionally pass the service id as an argument.`,
}

var InteractiveSSH = command.Wrap(sshCmd, loadDataSSH, renderSSH, nil)
var InteractiveSSHSelectService = command.Wrap(sshCmd, listServices, renderSSHSelection, nil)

type SSHInput struct {
	ServiceID string `cli:"arg:0"`
}

func loadDataSSH(ctx context.Context, in SSHInput) (*exec.Cmd, error) {
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
		return nil, fmt.Errorf("unsupported service type")
	}

	if sshAddress == nil {
		return nil, fmt.Errorf("service does not support ssh")
	}

	return exec.Command("ssh", *sshAddress), nil
}

func renderSSH(ctx context.Context, loadData func(in SSHInput) tui.TypedCmd[*exec.Cmd], in SSHInput) (tea.Model, error) {
	return tui.NewExecModel(loadData(in)), nil
}

func listServices(ctx context.Context, _ SSHInput) ([]*service.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	serviceRepo := service.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)

	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)

	return serviceService.ListServices(ctx, &client.ListServicesParams{
		Type:  &[]client.ServiceType{client.WebService, client.PrivateService, client.BackgroundWorker},
		Limit: pointers.From(100),
	})
}

func renderSSHSelection(ctx context.Context, loadData func(in SSHInput) tui.TypedCmd[[]*service.Model], _ SSHInput) (tea.Model, error) {
	columns := resourcetui.ColumnsForResources()

	createRowFunc := func(s *service.Model) table.Row {
		return resourcetui.RowForResource(s)
	}

	onSelect := func(rows []table.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}
		return InteractiveSSH(ctx, SSHInput{ServiceID: rows[0].Data["ID"].(string)})
	}

	t := tui.NewTable(
		columns,
		loadData(SSHInput{}),
		createRowFunc,
		onSelect,
	)

	return t, nil
}

func init() {
	rootCmd.AddCommand(sshCmd)

	sshCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		input := SSHInput{}
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if input.ServiceID != "" {
			InteractiveSSH(ctx, input)
			return nil
		}

		InteractiveSSHSelectService(ctx, input)
		return nil
	}
}
