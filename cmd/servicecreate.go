package cmd

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/github"
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
	
	// Handle path flag - create GitHub repo if path is provided
	if input.Path != nil && *input.Path != "" {
		if input.Repo != nil && *input.Repo != "" {
			command.Fatal(cmd, fmt.Errorf("cannot specify both --path and --repo"))
			return true
		}
		
		// Generate repo name from service name
		repoName := strings.ReplaceAll(input.Name, " ", "-")
		repoName = strings.ToLower(repoName)
		
		// Get organization if specified, default to maker-week-2025
		org := "maker-week-2025"
		if input.Org != nil && *input.Org != "" {
			org = *input.Org
		}
		
		// Remove the message about creating GitHub repo
		
		// Always create private repos
		repoURL, err := github.CreateRepoFromPath(ctx, *input.Path, repoName, true, org)
		if err != nil {
			command.Fatal(cmd, fmt.Errorf("failed to create GitHub repository: %w", err))
			return true
		}
		
		input.Repo = pointers.From(repoURL)
	}
	
	// Note: Some service types may require additional fields like repo,
	// but we'll let the API return appropriate error messages

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
	if input.Repo != nil {
		req.Repo = input.Repo
	}
	if input.Branch != nil {
		req.Branch = input.Branch
	}
	if input.RootDir != nil {
		req.RootDir = input.RootDir
	}

	// Set service details - required for all service types
	if input.Type != client.StaticSite || input.BuildCommand != nil || input.PublishPath != nil {
		req.ServiceDetails = &client.ServicePOST_ServiceDetails{}
		
		switch input.Type {
		case client.WebService:
			details := client.WebServiceDetailsPOST{}
			// Set optional fields if provided
			if input.Plan != nil {
				plan := client.PaidPlan(*input.Plan)
				details.Plan = &plan
			}
			if input.NumInstances != nil {
				details.NumInstances = input.NumInstances
			}
			
			// Set runtime - use Runtime field, not deprecated Env field
			runtime := client.ServiceRuntime("docker")
			if input.Runtime != nil {
				runtime = client.ServiceRuntime(*input.Runtime)
			}
			details.Runtime = runtime
			
			// Set env specific details based on runtime
			if runtime == "docker" || runtime == "image" {
				// For Docker runtime, use DockerDetailsPOST
				details.EnvSpecificDetails = &client.EnvSpecificDetailsPOST{}
				dockerDetails := client.DockerDetailsPOST{}
				details.EnvSpecificDetails.FromDockerDetailsPOST(dockerDetails)
			} else {
				// For native runtimes (node, python, ruby, go, elixir, rust)
				details.EnvSpecificDetails = &client.EnvSpecificDetailsPOST{}
				nativeDetails := client.NativeEnvironmentDetailsPOST{}
				// Set build and start commands if provided
				if input.BuildCommand != nil {
					nativeDetails.BuildCommand = *input.BuildCommand
				} else {
					nativeDetails.BuildCommand = ""
				}
				if input.StartCommand != nil {
					nativeDetails.StartCommand = *input.StartCommand  
				} else {
					nativeDetails.StartCommand = ""
				}
				details.EnvSpecificDetails.FromNativeEnvironmentDetailsPOST(nativeDetails)
			}
			
			req.ServiceDetails.FromWebServiceDetailsPOST(details)
			
		case client.PrivateService:
			details := client.PrivateServiceDetailsPOST{}
			// Set optional fields if provided
			if input.Plan != nil {
				plan := client.PaidPlan(*input.Plan)
				details.Plan = &plan
			}
			if input.NumInstances != nil {
				details.NumInstances = input.NumInstances
			}
			
			// Set runtime - use Runtime field, not deprecated Env field
			runtime := client.ServiceRuntime("docker")
			if input.Runtime != nil {
				runtime = client.ServiceRuntime(*input.Runtime)
			}
			details.Runtime = runtime
			
			// Set env specific details based on runtime
			if runtime == "docker" || runtime == "image" {
				// For Docker runtime, use DockerDetailsPOST
				details.EnvSpecificDetails = &client.EnvSpecificDetailsPOST{}
				dockerDetails := client.DockerDetailsPOST{}
				details.EnvSpecificDetails.FromDockerDetailsPOST(dockerDetails)
			} else {
				// For native runtimes (node, python, ruby, go, elixir, rust)
				details.EnvSpecificDetails = &client.EnvSpecificDetailsPOST{}
				nativeDetails := client.NativeEnvironmentDetailsPOST{}
				// Set build and start commands if provided
				if input.BuildCommand != nil {
					nativeDetails.BuildCommand = *input.BuildCommand
				} else {
					nativeDetails.BuildCommand = ""
				}
				if input.StartCommand != nil {
					nativeDetails.StartCommand = *input.StartCommand  
				} else {
					nativeDetails.StartCommand = ""
				}
				details.EnvSpecificDetails.FromNativeEnvironmentDetailsPOST(nativeDetails)
			}
			
			req.ServiceDetails.FromPrivateServiceDetailsPOST(details)
			
		case client.BackgroundWorker:
			details := client.BackgroundWorkerDetailsPOST{}
			// Set optional fields if provided
			if input.Plan != nil {
				plan := client.PaidPlan(*input.Plan)
				details.Plan = &plan
			}
			if input.NumInstances != nil {
				details.NumInstances = input.NumInstances
			}
			
			// Set runtime - use Runtime field, not deprecated Env field
			runtime := client.ServiceRuntime("docker")
			if input.Runtime != nil {
				runtime = client.ServiceRuntime(*input.Runtime)
			}
			details.Runtime = runtime
			
			// Set env specific details based on runtime
			if runtime == "docker" || runtime == "image" {
				// For Docker runtime, use DockerDetailsPOST
				details.EnvSpecificDetails = &client.EnvSpecificDetailsPOST{}
				dockerDetails := client.DockerDetailsPOST{}
				details.EnvSpecificDetails.FromDockerDetailsPOST(dockerDetails)
			} else {
				// For native runtimes (node, python, ruby, go, elixir, rust)
				details.EnvSpecificDetails = &client.EnvSpecificDetailsPOST{}
				nativeDetails := client.NativeEnvironmentDetailsPOST{}
				// Set build and start commands if provided
				if input.BuildCommand != nil {
					nativeDetails.BuildCommand = *input.BuildCommand
				} else {
					nativeDetails.BuildCommand = ""
				}
				if input.StartCommand != nil {
					nativeDetails.StartCommand = *input.StartCommand  
				} else {
					nativeDetails.StartCommand = ""
				}
				details.EnvSpecificDetails.FromNativeEnvironmentDetailsPOST(nativeDetails)
			}
			
			req.ServiceDetails.FromBackgroundWorkerDetailsPOST(details)
		
		case client.StaticSite:
			details := client.StaticSiteDetailsPOST{}
			if input.BuildCommand != nil {
				details.BuildCommand = input.BuildCommand
			}
			if input.PublishPath != nil {
				details.PublishPath = input.PublishPath
			}
			
			req.ServiceDetails.FromStaticSiteDetailsPOST(details)
		}
	}

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

	// Check if service is nil
	if svc == nil {
		command.Fatal(cmd, fmt.Errorf("service creation returned nil service"))
		return true
	}

	// Handle output format
	format := command.GetFormatFromContext(ctx)
	if format != nil {
		if _, err := command.PrintData(cmd, svc, func(s *client.Service) string {
			return fmt.Sprintf("Service created: %s (%s)", s.Name, s.Id)
		}); err != nil {
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
	cmd.Flags().String("path", "", "Local path (file or directory) to create GitHub repo from")
	cmd.Flags().String("org", "", "GitHub organization to create repo in (defaults to maker-week-2025)")
	
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
			if repo, _ := cmd.Flags().GetString("repo"); repo != "" {
				input.Repo = pointers.From(repo)
			}
			if branch, _ := cmd.Flags().GetString("branch"); branch != "" {
				input.Branch = pointers.From(branch)
			}
			if rootDir, _ := cmd.Flags().GetString("root-dir"); rootDir != "" {
				input.RootDir = pointers.From(rootDir)
			}
			if path, _ := cmd.Flags().GetString("path"); path != "" {
				input.Path = pointers.From(path)
			}
			if org, _ := cmd.Flags().GetString("org"); org != "" {
				input.Org = pointers.From(org)
			}
			
			// Parse static site specific flags
			if buildCmd, _ := cmd.Flags().GetString("build-command"); buildCmd != "" {
				input.BuildCommand = pointers.From(buildCmd)
			}
			if publishPath, _ := cmd.Flags().GetString("publish-path"); publishPath != "" {
				input.PublishPath = pointers.From(publishPath)
			}

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
		cmd.Flags().String("runtime", "", "Runtime (e.g., docker, node, python, ruby, go, elixir, rust)")
		cmd.Flags().String("plan", "", "Instance type (default \"starter\")")
		cmd.Flags().Int("num-instances", 0, "Number of instances (default 1)")
		
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			input := types.ServiceCreateInput{
				Type: serviceType,
			}
			
			// Parse common flags manually
			input.Name, _ = cmd.Flags().GetString("name")
			if repo, _ := cmd.Flags().GetString("repo"); repo != "" {
				input.Repo = pointers.From(repo)
			}
			if branch, _ := cmd.Flags().GetString("branch"); branch != "" {
				input.Branch = pointers.From(branch)
			}
			if rootDir, _ := cmd.Flags().GetString("root-dir"); rootDir != "" {
				input.RootDir = pointers.From(rootDir)
			}
			if path, _ := cmd.Flags().GetString("path"); path != "" {
				input.Path = pointers.From(path)
			}
			if org, _ := cmd.Flags().GetString("org"); org != "" {
				input.Org = pointers.From(org)
			}
			
			// Parse server specific flags
			if buildCmd, _ := cmd.Flags().GetString("build-command"); buildCmd != "" {
				input.BuildCommand = pointers.From(buildCmd)
			}
			if startCmd, _ := cmd.Flags().GetString("start-command"); startCmd != "" {
				input.StartCommand = pointers.From(startCmd)
			}
			if runtime, _ := cmd.Flags().GetString("runtime"); runtime != "" {
				input.Runtime = pointers.From(runtime)
			}
			if plan, _ := cmd.Flags().GetString("plan"); plan != "" {
				input.Plan = pointers.From(plan)
			}
			if numInstances, _ := cmd.Flags().GetInt("num-instances"); numInstances != 0 {
				input.NumInstances = pointers.From(numInstances)
			}

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