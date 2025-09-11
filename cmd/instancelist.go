package cmd

import (
	"context"
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/views"
)

var instanceListCmd = &cobra.Command{
	Use:   "instances [serviceID]",
	Short: "List instances for a service",
	Args:  cobra.MaximumNArgs(1),
}

func loadInstanceList(ctx context.Context, input views.InstanceListInput) ([]*client.ServiceInstance, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.ListInstancesWithResponse(ctx, input.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to list instances: %s", resp.Status())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response")
	}

	// Convert to pointers and sort by creation time (newest first)
	instances := make([]*client.ServiceInstance, len(*resp.JSON200))
	for i := range *resp.JSON200 {
		instances[i] = &(*resp.JSON200)[i]
	}

	sort.Slice(instances, func(i, j int) bool {
		return instances[i].CreatedAt.After(instances[j].CreatedAt)
	})

	return instances, nil
}

func interactiveInstanceList(cmd *cobra.Command, input views.InstanceListInput) tea.Cmd {
	ctx := cmd.Context()
	if input.ServiceID == "" {
		return command.AddToStackFunc(
			ctx,
			cmd,
			"Instances",
			&input,
			views.NewServiceList(ctx, views.ServiceInput{}, func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceID = r.ID()
				service, err := resource.GetResource(ctx, input.ServiceID)
				if err != nil {
					command.Fatal(cmd, err)
				}
				return InteractiveInstanceList(ctx, input, service, "Instances for "+resource.BreadcrumbForResource(service))
			}),
		)
	}

	service, err := resource.GetResource(ctx, input.ServiceID)
	if err != nil {
		command.Fatal(cmd, err)
	}

	return InteractiveInstanceList(ctx, input, service, "Instances for "+resource.BreadcrumbForResource(service))
}

var InteractiveInstanceList = func(ctx context.Context, input views.InstanceListInput, r resource.Resource, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, instanceListCmd, breadcrumb, &input, views.NewInstanceListView(
		ctx,
		input,
	))
}

func init() {
	servicesCmd.AddCommand(instanceListCmd)

	instanceListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.InstanceListInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]*client.ServiceInstance, error) {
			return loadInstanceList(cmd.Context(), input)
		}, text.InstanceTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		interactiveInstanceList(cmd, input)
		return nil
	}
}
