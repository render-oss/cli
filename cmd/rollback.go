/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/renderinc/render-cli/pkg/renderclient"
	"github.com/renderinc/render-cli/pkg/resource"
	"github.com/spf13/cobra"
)

// rollbackCmd represents the rollback command
var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback a service to a previous deploy",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		c, err := renderclient.NewClient()
		if err != nil {
			return err
		}

		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}

		deployID, err := cmd.Flags().GetString("deploy")
		if err != nil {
			return err
		}

		serviceID, err := cmd.Flags().GetString("id")
		if err != nil {
			return err
		}

		if serviceID == "" {
			svcs, err := resource.ServicesForInput(ctx, c, &resource.ServiceListInput{
				Name: name,
			})
			if err != nil {
				return err
			}

			services := *svcs

			if len(services) == 0 {
				return fmt.Errorf("no services found for name %s", name)
			}

			if len(services) > 1 {
				// TODO: prompt user to select a service

				for _, service := range services {
					fmt.Println("Service", service.Service.Id, service.Service.Name)
				}
				fmt.Println("Please specify a service ID to rollback")
				return nil
			}
			serviceID = services[0].Service.Id
		}

		if deployID == "" {
			deploys, err := resource.DeploysForInput(ctx, c, &resource.DeployListInput{
				ServiceID: serviceID,
			})
			if err != nil {
				return err
			}

			for _, deployWithCursor := range *deploys {
				fmt.Printf("Deploy %s: %s %s\n",
					deployWithCursor.Deploy.Id,
					*deployWithCursor.Deploy.CreatedAt,
					*deployWithCursor.Deploy.Commit.Message,
				)
			}
			// TODO: prompt user to select a deployWithCursor
			fmt.Println("Please specify a deployWithCursor ID to rollback to")
			return nil
		}

		deploy, err := resource.Rollback(ctx, c, serviceID, deployID)
		if err != nil {
			return err
		}

		fmt.Println("Rolled back to deploy", deploy.Id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// rollbackCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	rollbackCmd.Flags().String("id", "", "ID of the service to rollback")
	rollbackCmd.Flags().String("name", "", "Name of the service to rollback")
	rollbackCmd.Flags().String("deploy", "", "Deploy ID to rollback to")
}
