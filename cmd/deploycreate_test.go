package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func TestDeployCreateJSONConfirmation(t *testing.T) {
	for _, tty := range []bool{false, true} {
		name := "non-TTY"
		if tty {
			name = "TTY"
		}
		t.Run(name, func(t *testing.T) {
			for _, tt := range []struct {
				name    string
				input   string
				confirm bool
				wantEOF bool
			}{
				{name: "closed stdin", wantEOF: true},
				{name: "confirmed prompt", input: "y\n"},
				{name: "confirm flag", confirm: true},
			} {
				t.Run(tt.name, func(t *testing.T) {
					// Use a focused server to observe exactly when the deploy POST occurs;
					// the shared fake currently only supports listing deploys.
					var requests []string
					var deployBody client.CreateDeployJSONRequestBody
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests = append(requests, r.Method+" "+r.URL.Path)
						w.Header().Set("Content-Type", "application/json")
						switch r.Method + " " + r.URL.Path {
						case "GET /services/srv-test":
							_, _ = io.WriteString(w, `{"id":"srv-test","name":"test-service"}`)
						case "POST /services/srv-test/deploys":
							if err := json.NewDecoder(r.Body).Decode(&deployBody); err != nil {
								t.Errorf("invalid deploy body: %v", err)
								w.WriteHeader(http.StatusBadRequest)
								return
							}
							w.WriteHeader(http.StatusCreated)
							_, _ = io.WriteString(w, `{"id":"dep-test","status":"queued"}`)
						default:
							t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
							w.WriteHeader(http.StatusNotFound)
						}
					}))
					t.Cleanup(server.Close)
					t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
					t.Setenv("RENDER_HOST", server.URL)
					t.Setenv("RENDER_API_KEY", "test-api-key")
					c, err := client.NewClientWithResponses(server.URL)
					require.NoError(t, err)
					deps := dependencies.New(c)
					deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
						return command.RuntimeSignals{StdinTTY: tty, StdoutTTY: tty, StderrTTY: tty}, nil
					}
					root := newRootCmd()
					create := &cobra.Command{Use: "create [serviceID]", RunE: deployCreateCmd.RunE}
					create.Flags().String("commit", "", "")
					create.Flags().String("image", "", "")
					create.Flags().Bool("clear-cache", false, "")
					create.Flags().Bool("wait", false, "")
					deploys := &cobra.Command{Use: "deploys"}
					deploys.AddCommand(create)
					root.AddCommand(deploys)
					setupRootCmdPersistentRun(root, deps)
					var stdout, stderr bytes.Buffer
					root.SetOut(&stdout)
					root.SetErr(&stderr)
					root.SetIn(strings.NewReader(tt.input))
					args := []string{"deploys", "create", "srv-test", "--commit", "0123abcd", "--output", "json"}
					if tt.confirm {
						args = append(args, "--confirm")
					}
					root.SetArgs(args)

					err = root.Execute()
					if tt.wantEOF {
						require.ErrorIs(t, err, io.EOF)
						assert.Contains(t, err.Error(), "--confirm")
						assert.Empty(t, stdout.String())
						assert.Equal(t, []string{"GET /services/srv-test"}, requests)
						return
					}
					require.NoError(t, err)
					assert.JSONEq(t, `{"id":"dep-test","status":"queued"}`, stdout.String())
					assert.Equal(t, "POST /services/srv-test/deploys", requests[len(requests)-1])
					require.NotNil(t, deployBody.CommitId)
					assert.Equal(t, "0123abcd", *deployBody.CommitId)
					if tt.confirm {
						assert.Empty(t, stderr.String())
					} else {
						assert.Contains(t, stderr.String(), "Are you sure you want to deploy test-service?")
					}
				})
			}
		})
	}
}
