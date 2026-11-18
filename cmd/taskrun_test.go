package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/text"
)

func TestTaskRunOutputKeepsRawStatuses(t *testing.T) {
	for _, status := range []string{"pending", "running", "sleeping", "paused", "completed", "future_state"} {
		for _, format := range []command.Output{command.JSON, command.YAML, command.TEXT} {
			for _, details := range []bool{false, true} {
				t.Run(status+"/"+string(format)+"/"+map[bool]string{false: "list", true: "details"}[details], func(t *testing.T) {
					cmd := &cobra.Command{}
					cmd.SetContext(command.SetFormatInContext(context.Background(), &format))
					var out bytes.Buffer
					cmd.SetOut(&out)
					var err error
					if details {
						_, err = command.NonInteractive(cmd, func() (*wfclient.TaskRunDetails, error) {
							return &wfclient.TaskRunDetails{Id: "trn-1", Status: wfclient.TaskRunStatus(status)}, nil
						}, text.TaskRunDetails)
					} else {
						_, err = command.NonInteractive(cmd, func() ([]*wfclient.TaskRun, error) {
							return []*wfclient.TaskRun{{Id: "trn-1", Status: wfclient.TaskRunStatus(status)}}, nil
						}, text.TaskRunTable)
					}
					require.NoError(t, err)
					if format == command.TEXT {
						assert.Contains(t, out.String(), status)
						return
					}
					var body any
					if format == command.JSON {
						require.NoError(t, json.Unmarshal(out.Bytes(), &body))
					} else {
						require.NoError(t, yaml.Unmarshal(out.Bytes(), &body))
					}
					if !details {
						body = body.([]any)[0]
					}
					assert.Equal(t, status, body.(map[string]any)["status"])
				})
			}
		}
	}
}
