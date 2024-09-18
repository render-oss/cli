package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/input"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [serviceID]",
	Short: "Update a service",
	Args:  cobra.ExactArgs(1),
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

func init() {
	servicesCmd.AddCommand(updateCmd)
}
