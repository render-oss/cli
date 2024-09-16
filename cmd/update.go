/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/input"
	"github.com/renderinc/render-cli/pkg/services"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		serviceRepo := services.NewServiceRepo(http.DefaultClient, os.Getenv("RENDER_HOST"), os.Getenv("RENDER_API_KEY"))
		srv, err := serviceRepo.GetService(serviceID)
		if err != nil {
			return err
		}

		svc, err := stripReadOnlyFields(srv)
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

		_, err = serviceRepo.UpdateService(serviceID, updatedService)
		if err != nil {
			return err
		}

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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// updateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// updateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
