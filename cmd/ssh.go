package cmd

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/project"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/resource"
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

var InteractiveSSH = command.Wrap(sshCmd, loadDataSSH, renderSSH)
var InteractiveSSHSelectService = command.Wrap(sshCmd, listServices, renderSSHSelection)

type SSHInput struct {
	ServiceID string `cli:"arg:0"`
}

func (s SSHInput) String() []string {
	return []string{s.ServiceID}
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

func renderSSHSelection(ctx context.Context, loadData func(in SSHInput) ([]*service.Model, error), _ SSHInput) (tea.Model, error) {
	columns := resource.ColumnsForResources()

	loadDataFunc := func() ([]*service.Model, error) {
		return loadData(SSHInput{})
	}

	createRowFunc := func(s *service.Model) table.Row {
		return resource.RowForResource(s)
	}

	onSelect := func(rows []table.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}
		return InteractiveSSH(ctx, SSHInput{ServiceID: rows[0].Data["ID"].(string)})
	}

	t := tui.NewTable(
		columns,
		loadDataFunc,
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

		InteractiveSSHSelectService(ctx, input)
		return nil
	}
}
