package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/deploy"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/types"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy [serviceID]",
	Short: "Deploy a service and tail logs",
	Args:  cobra.MaximumNArgs(1),
}

var InteractiveDeploy = command.Wrap(deployCmd, createDeploy, renderCreateDeploy)

func createDeploy(ctx context.Context, input types.DeployInput) (*client.Deploy, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	deployRepo := deploy.NewRepo(c)

	d, err := deployRepo.TriggerDeploy(ctx, input.ServiceID, deploy.TriggerDeployInput{
		ClearCache: input.ClearCache,
		CommitId:   input.CommitID,
		ImageUrl:   input.ImageURL,
	})
	if err != nil {
		return nil, err
	}

	return d, nil
}

func renderCreateDeploy(ctx context.Context, loadData func(types.DeployInput) (*client.Deploy, error), input types.DeployInput) (tea.Model, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	serviceRepo := service.NewRepo(c)
	svc, err := serviceRepo.GetService(ctx, input.ServiceID)
	if err != nil {
		return nil, err
	}

	var inputs []huh.Field
	if svc.ImagePath != nil {
		if input.ImageURL == nil {
			input.ImageURL = pointers.From("")
		}

		inputs = append(inputs, huh.NewInput().
			Title("Image URL").
			Placeholder("Enter Docker image URL (optional)").
			Value(input.ImageURL))
	} else {
		if input.CommitID == nil {
			input.CommitID = pointers.From("")
		}

		inputs = append(inputs, huh.NewInput().
			Title("Commit ID").
			Placeholder("Enter commit ID (optional)").
			Value(input.CommitID))
	}

	deployForm := huh.NewForm(huh.NewGroup(inputs...))

	deployRepo := deploy.NewRepo(c)

	logData := func(in LogInput) (*client.Logs200Response, error) {
		return loadLogData(ctx, in)
	}

	logModelFunc := func() (tea.Model, error) {
		model, err := renderLogs(ctx, logData, LogInput{ResourceIDs: []string{input.ServiceID}})
		if err != nil {
			return nil, err
		}
		model.Init()
		return model, nil
	}

	onSubmit := func() tea.Cmd {
		return func() tea.Msg {
			if input.CommitID != nil && *input.CommitID == "" {
				input.CommitID = nil
			}

			if input.ImageURL != nil && *input.ImageURL == "" {
				input.ImageURL = nil
			}

			var err error
			_, err = deployRepo.TriggerDeploy(ctx, input.ServiceID, deploy.TriggerDeployInput{
				CommitId: input.CommitID,
				ImageUrl: input.ImageURL,
			})
			if err != nil {
				return tui.ErrorMsg{Err: fmt.Errorf("failed to trigger deploy: %w", err)}
			}

			return tea.Println("Deploy triggered")
		}
	}

	action := tui.NewFormAction(
		deployForm,
		logModelFunc,
		onSubmit,
	)

	return tui.NewFormWithAction(action, deployForm), nil
}

func init() {
	deployCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input types.DeployInput
		if len(args) > 0 {
			input.ServiceID = args[0]
		}

		clearCache, _ := cmd.Flags().GetBool("clear-cache")
		commitID, _ := cmd.Flags().GetString("commit-id")
		imageURL, _ := cmd.Flags().GetString("image-url")

		input.ClearCache = &clearCache
		input.CommitID = &commitID
		input.ImageURL = &imageURL

		InteractiveDeploy(cmd.Context(), input)
		return nil
	}

	rootCmd.AddCommand(deployCmd)

	deployCmd.Flags().Bool("clear-cache", false, "Clear build cache before deploying")
	deployCmd.Flags().String("commit-id", "", "The commit ID to deploy")
	deployCmd.Flags().String("image-url", "", "The Docker image URL to deploy")
}
