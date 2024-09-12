package cmd

import (
	"fmt"

	"github.com/renderinc/render-cli/pkg/input"
	"github.com/renderinc/render-cli/pkg/renderclient"
	"github.com/renderinc/render-cli/pkg/resource"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback a service to a previous deploy",
	RunE:  runRollback,
}

func init() {
	rootCmd.AddCommand(rollbackCmd)

	rollbackCmd.Flags().String("id", "", "ID of the service to rollback")
	rollbackCmd.Flags().String("name", "", "Name of the service to rollback")
	rollbackCmd.Flags().String("deploy", "", "Deploy ID to rollback to")
}

func runRollback(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := renderclient.NewClient()
	if err != nil {
		return err
	}

	idFlag, _ := cmd.Flags().GetString("id")
	nameFlag, _ := cmd.Flags().GetString("name")
	serviceID, err := input.GetServiceID(ctx, c, idFlag, nameFlag)
	if err != nil {
		return err
	}

	deployFlag, _ := cmd.Flags().GetString("deploy")
	deployID, err := input.GetDeployID(ctx, serviceID, deployFlag)
	if err != nil {
		return err
	}

	deploy, err := resource.Rollback(ctx, c, serviceID, deployID)
	if err != nil {
		return err
	}

	fmt.Println("Rolled back to deploy", deploy.Id)
	return nil
}
