package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/flows"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
)

func NewVersionReleaseCmd(deps flows.WorkflowDeps) *cobra.Command {
	var versionReleaseCmd = &cobra.Command{
		Use:   "release [workflowID]",
		Short: "Release a new workflow version",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, local, err := getLocalDeps(cmd, deps)
			if err != nil {
				return fmt.Errorf("failed to get local deps: %w", err)
			}

			var input workflowviews.VersionReleaseInput
			err = command.ParseCommand(cmd, args, &input)
			if err != nil {
				return fmt.Errorf("failed to parse input: %w", err)
			}

			// if wait flag is used, default to non-interactive output
			if input.Wait {
				command.DefaultFormatNonInteractive(cmd)
			}

			nonInteractive := nonInteractiveVersionRelease(cmd, input, deps)
			if nonInteractive {
				return nil
			}

			flows.NewWorkflow(deps, flows.NewLogFlow(deps, flows.WithLocal(local)), local).VersionRelease(cmd.Context(), &input)
			return nil
		},
	}

	// TODO CAP-7490
	// https://linear.app/render-com/issue/CAP-7490/flesh-out-workflow-version-information-at-least-restgql-if-not-present
	// these are stubbed and non-functional
	// the underlying information we need to display/act on these is not yet available
	versionReleaseCmd.Flags().String("commit", "", "The commit ID to release")
	versionReleaseCmd.Flags().Bool("wait", false, "Wait for release to finish. Returns non-zero exit code if release fails")
	// optionally, image backed is not in scope for alpha, native env only
	// versionReleaseCmd.Flags().String("image", "", "The Docker image URL to release")

	return versionReleaseCmd
}

func nonInteractiveVersionRelease(cmd *cobra.Command, input workflowviews.VersionReleaseInput, deps flows.WorkflowDeps) bool {
	var wfv *wfclient.WorkflowVersion
	releaseVersion := func() (*wfclient.WorkflowVersion, error) {
		v, err := deps.WorkflowLoader().ReleaseVersion(cmd.Context(), input)
		if err != nil {
			return nil, err
		}

		if v == nil {
			_, err = fmt.Fprintf(cmd.OutOrStderr(), "Waiting for version to be released...\n\n")
			if err != nil {
				return nil, err
			}
			wfv, err = deps.WorkflowLoader().WaitForVersionRelease(cmd.Context(), input.WorkflowID)
			if err != nil {
				return nil, err
			}

			v = wfv
		}

		if input.Wait {
			_, err = fmt.Fprintf(cmd.OutOrStderr(), "Waiting for release %s to complete...\n\n", v.Id)
			if err != nil {
				return nil, err
			}
			wfv, err = deps.WorkflowLoader().WaitForVersion(cmd.Context(), input.WorkflowID, v.Id)
			return wfv, err
		}

		return v, err
	}

	nonInteractive, err := command.NonInteractiveWithConfirm(cmd, releaseVersion, text.Version(input.WorkflowID), deps.WorkflowLoader().VersionReleaseConfirm(cmd.Context(), input))
	if err != nil {
		_, err = fmt.Fprint(cmd.OutOrStderr(), err.Error()+"\n")
		os.Exit(1)
	}
	if !nonInteractive {
		return false
	}

	/*
		if input.Wait && !workflowversion.IsSuccessful(wfv.Status) {
			os.Exit(1)
		}
	*/

	return nonInteractive
}
