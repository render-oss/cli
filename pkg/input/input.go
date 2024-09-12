package input

import (
	"context"
	"fmt"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/deploys"
	"github.com/renderinc/render-cli/pkg/resource"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/validate"
)

func GetServiceID(ctx context.Context, c *client.ClientWithResponses, idFlag, nameFlag string) (string, error) {
	if validate.IsObjectID(string(validate.ServiceIDPrefix), idFlag) {
		return idFlag, nil
	}

	if nameFlag == "" {
		return "", fmt.Errorf("either --id or --name must be provided")
	}

	services, err := resource.ServicesForInput(ctx, c, &resource.ServiceListInput{Name: nameFlag})
	if err != nil {
		return "", err
	}

	if len(*services) == 0 {
		return "", fmt.Errorf("no services found for name %s", nameFlag)
	}

	if len(*services) > 1 {
		selected, err := tui.SelectFromList("Select a service", *services, func(s client.ServiceWithCursor) string {
			return fmt.Sprintf("%s (%s)", s.Service.Name, s.Service.Id)
		})
		if err != nil {
			return "", err
		}

		for _, s := range *services {
			if fmt.Sprintf("%s (%s)", s.Service.Name, s.Service.Id) == selected {
				return s.Service.Id, nil
			}
		}
		return "", fmt.Errorf("selected service not found")
	}

	return (*services)[0].Service.Id, nil
}

func GetDeployID(ctx context.Context, serviceID, deployFlag string) (string, error) {
	if deployFlag != "" {
		return deployFlag, nil
	}

	deployRepo := deploys.NewDeployRepo(http.DefaultClient, os.Getenv("RENDER_HOST"), os.Getenv("RENDER_API_KEY"))
	m := tui.NewDeployTableModel(deployRepo, serviceID)
	p := tea.NewProgram(m)
	model, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running program: %v", err)
	}

	finalModel := model.(tui.DeployTableModel)
	if finalModel.SelectedID == "" {
		return "", fmt.Errorf("no deploy selected")
	}

	return finalModel.SelectedID, nil
}
