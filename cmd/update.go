package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/github"
	"github.com/render-oss/cli/pkg/input"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/service"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:    "update [serviceID]",
	Short:  "Update a service",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		serviceRepo, serviceService, err := newRepositories()
		if err != nil {
			return err
		}

		srv, err := serviceService.GetService(cmd.Context(), serviceID)
		if err != nil {
			return err
		}

		// Handle --path flag
		path, _ := cmd.Flags().GetString("path")
		if path != "" {
			return updateServiceRepo(cmd.Context(), srv.Service, path)
		}

		// Original JSON editor flow
		svc, err := stripReadOnlyFields(srv.Service)
		if err != nil {
			return err
		}

		srvJSON, err := json.MarshalIndent(svc, "", "    ")
		if err != nil {
			return err
		}

		content, err := input.OpenEditorForInput("update-service*.json", string(srvJSON))
		if err != nil {
			return err
		}

		updatedService := client.ServicePATCH{}
		err = json.Unmarshal([]byte(content), &updatedService)
		if err != nil {
			return err
		}

		_, err = serviceRepo.UpdateService(cmd.Context(), serviceID, updatedService)
		if err != nil {
			return err
		}

		fmt.Printf("Service %s updated successfully\n", serviceID)
		return nil
	},
}

// stripReadOnlyFields removes read-only fields from the retrieved service
// by marshalling it to JSON and then unmarshalling it to a ServicePATCH.
// Unfortunately, because the service details are stored as a union type
// we need to cast the ServiceDetails to the correct union type.
func stripReadOnlyFields(retrievedService *client.Service) (*client.ServicePATCH, error) {
	srvAsJSON, err := json.Marshal(retrievedService)
	if err != nil {
		return nil, err
	}

	var patch *client.ServicePATCH
	err = json.Unmarshal(srvAsJSON, &patch)
	if err != nil {
		return nil, err
	}

	switch retrievedService.Type {
	case client.WebService:
		webServiceDetails, err := patch.ServiceDetails.AsWebServiceDetailsPATCH()
		if err != nil {
			return nil, err
		}

		if err := patch.ServiceDetails.FromWebServiceDetailsPATCH(webServiceDetails); err != nil {
			return nil, err
		}
	case client.PrivateService:
		privateServiceDetails, err := patch.ServiceDetails.AsPrivateServiceDetailsPATCH()
		if err != nil {
			return nil, err
		}

		if err := patch.ServiceDetails.FromPrivateServiceDetailsPATCH(privateServiceDetails); err != nil {
			return nil, err
		}
	case client.BackgroundWorker:
		backgroundWorkerDetails, err := patch.ServiceDetails.AsBackgroundWorkerDetailsPATCH()
		if err != nil {
			return nil, err
		}

		if err := patch.ServiceDetails.FromBackgroundWorkerDetailsPATCH(backgroundWorkerDetails); err != nil {
			return nil, err
		}
	case client.StaticSite:
		staticSiteDetails, err := patch.ServiceDetails.AsStaticSiteDetailsPATCH()
		if err != nil {
			return nil, err
		}

		if err := patch.ServiceDetails.FromStaticSiteDetailsPATCH(staticSiteDetails); err != nil {
			return nil, err
		}
	}

	return patch, nil
}

func newRepositories() (*service.Repo, *service.Service, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, nil, err
	}

	serviceRepo := service.NewRepo(c)

	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)

	return serviceRepo, serviceService, nil
}

func updateServiceRepo(ctx context.Context, srv *client.Service, localPath string) error {
	// Check if service has a repo
	if srv.Repo == nil || *srv.Repo == "" {
		return fmt.Errorf("service does not have a connected GitHub repository")
	}

	// Parse the repo URL to get owner and repo name
	repoURL := *srv.Repo
	owner, repoName, err := parseGitHubURL(repoURL)
	if err != nil {
		return fmt.Errorf("failed to parse repository URL: %w", err)
	}

	// Update the GitHub repository with new files
	err = github.UpdateRepoFromPath(ctx, localPath, owner, repoName)
	if err != nil {
		return fmt.Errorf("failed to update GitHub repository: %w", err)
	}

	fmt.Printf("Service %s updated successfully\n", srv.Id)
	return nil
}

func parseGitHubURL(repoURL string) (owner, repo string, err error) {
	// Handle both HTTPS and SSH URLs
	repoURL = strings.TrimSuffix(repoURL, ".git")
	
	if strings.HasPrefix(repoURL, "git@github.com:") {
		// SSH URL: git@github.com:owner/repo
		parts := strings.SplitN(strings.TrimPrefix(repoURL, "git@github.com:"), "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid GitHub SSH URL format")
		}
		return parts[0], parts[1], nil
	}
	
	// HTTPS URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}
	
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub URL format")
	}
	
	return parts[0], parts[1], nil
}

func init() {
	updateCmd.Flags().String("path", "", "Local path (file or directory) to update GitHub repo with")
	servicesCmd.AddCommand(updateCmd)
}
