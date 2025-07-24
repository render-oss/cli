package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/render-oss/cli/pkg/types"
)

var serviceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new service",
}

var serviceCreateStaticCmd = &cobra.Command{
	Use:   "static",
	Short: "Create a new static site",
	Args:  cobra.NoArgs,
}

var serviceCreateWebCmd = &cobra.Command{
	Use:   "web",
	Short: "Create a new web service",
	Args:  cobra.NoArgs,
}

var serviceCreatePrivateCmd = &cobra.Command{
	Use:   "private",
	Short: "Create a new private service",
	Args:  cobra.NoArgs,
}

var serviceCreateWorkerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Create a new background worker",
	Args:  cobra.NoArgs,
}

var InteractiveServiceCreate = func(ctx context.Context, input types.ServiceCreateInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		serviceCreateCmd,
		breadcrumb,
		&input,
		views.NewServiceCreateView(ctx, input, func(s *client.Service) tea.Cmd {
			// For now, just show the service ID - we can add details view later
			return nil
		}))
}

func interactiveServiceCreate(cmd *cobra.Command, input types.ServiceCreateInput) tea.Cmd {
	ctx := cmd.Context()
	return InteractiveServiceCreate(ctx, input, "Create Service")
}

func nonInteractiveServiceCreate(cmd *cobra.Command, input types.ServiceCreateInput) bool {
	ctx := cmd.Context()
	
	if input.Name == "" {
		command.Fatal(cmd, fmt.Errorf("service name is required in non-interactive mode"))
		return true
	}

	// Get workspace ID
	workspace, err := config.WorkspaceID()
	if err != nil {
		command.Fatal(cmd, err)
		return true
	}
	if workspace == "" {
		command.Fatal(cmd, fmt.Errorf("workspace is required"))
		return true
	}

	// Create service request
	req := client.ServicePOST{
		Name:    input.Name,
		OwnerId: workspace,
		Type:    input.Type,
	}

	// Set common fields
	if input.Repo != "" {
		req.Repo = pointers.From(input.Repo)
	}
	if input.Branch != "" {
		req.Branch = pointers.From(input.Branch)
	}
	if input.RootDir != "" {
		req.RootDir = pointers.From(input.RootDir)
	}

	// For now, we'll create services with minimal configuration
	// The API will use sensible defaults which can be updated later
	// This simplifies the initial implementation and avoids complex type marshaling

	// Create the service
	c, err := client.NewDefaultClient()
	if err != nil {
		command.Fatal(cmd, fmt.Errorf("failed to create client: %w", err))
		return true
	}
	repo := service.NewRepo(c)
	svc, err := repo.CreateService(ctx, req)
	if err != nil {
		command.Fatal(cmd, err)
		return true
	}

	// Handle output format
	format := command.GetFormatFromContext(ctx)
	if format != nil {
		if _, err := command.PrintData(cmd, svc, nil); err != nil {
			command.Fatal(cmd, err)
			return true
		}
	} else {
		command.Println(cmd, fmt.Sprintf("Service created: %s (%s)", svc.Name, svc.Id))
	}

	return true
}

func setupServiceCreateCommand(cmd *cobra.Command, serviceType client.ServiceType) {
	// Common flags
	cmd.Flags().String("name", "", "Service name")
	cmd.Flags().String("repo", "", "Git repository URL")
	cmd.Flags().String("branch", "", "Git branch")
	cmd.Flags().String("root-dir", "", "Root directory")
	
	// Service type specific flags
	switch serviceType {
	case client.StaticSite:
		cmd.Flags().String("build-command", "", "Build command")
		cmd.Flags().String("publish-path", "", "Publish path")
		
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			input := types.ServiceCreateInput{
				Type: serviceType,
			}
			
			// Parse common flags manually
			input.Name, _ = cmd.Flags().GetString("name")
			input.Repo, _ = cmd.Flags().GetString("repo")
			input.Branch, _ = cmd.Flags().GetString("branch")
			input.RootDir, _ = cmd.Flags().GetString("root-dir")
			
			// Parse static site specific flags
			input.BuildCommand, _ = cmd.Flags().GetString("build-command")
			input.PublishPath, _ = cmd.Flags().GetString("publish-path")

			nonInteractive := nonInteractiveServiceCreate(cmd, input)
			if nonInteractive {
				return nil
			}

			interactiveServiceCreate(cmd, input)
			return nil
		}
		
	case client.WebService, client.PrivateService, client.BackgroundWorker:
		cmd.Flags().String("build-command", "", "Build command")
		cmd.Flags().String("start-command", "", "Start command")
		cmd.Flags().String("runtime", "", "Runtime (e.g., docker, node, python-3, ruby-3, go-1)")
		cmd.Flags().String("env", "", "Environment (deprecated, use --runtime)")
		cmd.Flags().String("plan", "starter", "Instance type")
		cmd.Flags().Int("num-instances", 1, "Number of instances")
		
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			input := types.ServiceCreateInput{
				Type: serviceType,
			}
			
			// Parse common flags manually
			input.Name, _ = cmd.Flags().GetString("name")
			input.Repo, _ = cmd.Flags().GetString("repo")
			input.Branch, _ = cmd.Flags().GetString("branch")
			input.RootDir, _ = cmd.Flags().GetString("root-dir")
			
			// Parse server specific flags
			input.BuildCommand, _ = cmd.Flags().GetString("build-command")
			input.StartCommand, _ = cmd.Flags().GetString("start-command")
			input.Runtime, _ = cmd.Flags().GetString("runtime")
			input.Env, _ = cmd.Flags().GetString("env")
			input.Plan, _ = cmd.Flags().GetString("plan")
			input.NumInstances, _ = cmd.Flags().GetInt("num-instances")

			nonInteractive := nonInteractiveServiceCreate(cmd, input)
			if nonInteractive {
				return nil
			}

			interactiveServiceCreate(cmd, input)
			return nil
		}
	}
}

func init() {
	setupServiceCreateCommand(serviceCreateStaticCmd, client.StaticSite)
	setupServiceCreateCommand(serviceCreateWebCmd, client.WebService)
	setupServiceCreateCommand(serviceCreatePrivateCmd, client.PrivateService)
	setupServiceCreateCommand(serviceCreateWorkerCmd, client.BackgroundWorker)

	serviceCreateCmd.AddCommand(serviceCreateStaticCmd)
	serviceCreateCmd.AddCommand(serviceCreateWebCmd)
	serviceCreateCmd.AddCommand(serviceCreatePrivateCmd)
	serviceCreateCmd.AddCommand(serviceCreateWorkerCmd)

	servicesCmd.AddCommand(serviceCreateCmd)
}