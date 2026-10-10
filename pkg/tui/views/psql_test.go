package views

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPSQLResult_JSONMarshal(t *testing.T) {
	tests := []struct {
		name     string
		result   PSQLResult
		expected string
	}{
		{
			name:     "tabular output",
			result:   PSQLResult{Output: " id | name\n----+------\n  1 | test\n(1 row)\n"},
			expected: `{"output":" id | name\n----+------\n  1 | test\n(1 row)\n"}`,
		},
		{
			name:     "empty output",
			result:   PSQLResult{Output: ""},
			expected: `{"output":""}`,
		},
		{
			name:     "csv output from passthrough",
			result:   PSQLResult{Output: "id,name\n1,alice\n2,bob\n"},
			expected: `{"output":"id,name\n1,alice\n2,bob\n"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.result)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(b))
		})
	}
}

func TestPSQLResult_YAMLMarshal(t *testing.T) {
	result := PSQLResult{Output: "hello world\n"}

	b, err := yaml.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(b), "output:")
	require.Contains(t, string(b), "hello world")
}

const (
	psqlHelperEnv       = "RENDER_CLI_TEST_PSQL_HELPER"
	psqlHelperMarkerEnv = "RENDER_CLI_TEST_PSQL_HELPER_MARKER"

	testPostgresID       = "dpg-12345678901234567890"
	testPostgresName     = "private-db"
	testPostgresPassword = "test-password"
	testExternalURL      = "postgresql://user:" + testPostgresPassword + "@dpg-12345678901234567890.example.com/db"
)

// TestMain lets the test binary stand in for psql/pgcli. When psqlHelperEnv is set, the
// process records that it was launched and echoes its arguments instead of running tests.
func TestMain(m *testing.M) {
	if os.Getenv(psqlHelperEnv) == "1" {
		if marker := os.Getenv(psqlHelperMarkerEnv); marker != "" {
			_ = os.WriteFile(marker, []byte("launched"), 0o600)
		}
		_ = json.NewEncoder(os.Stdout).Encode(os.Args[1:])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// setupPSQLTest serves the Postgres detail and connection-info endpoints, stubs public IP
// discovery, and returns a tool path that records whether it was launched.
func setupPSQLTest(t *testing.T, connectionInfo string, userIP net.IP, userIPOK bool, allowList string) (tool PSQLTool, launched func() bool) {
	t.Helper()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/postgres/"+testPostgresID+"/connection-info"):
			_, _ = w.Write([]byte(connectionInfo))
		case strings.HasSuffix(r.URL.Path, "/postgres/"+testPostgresID):
			_, _ = fmt.Fprintf(w, `{"id":%q,"name":%q,"ipAllowList":%s}`, testPostgresID, testPostgresName, allowList)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Close)

	t.Setenv("RENDER_API_KEY", "test-key")
	t.Setenv("RENDER_HOST", s.URL)

	origGetUserIP := getUserIP
	getUserIP = func() (net.IP, bool) { return userIP, userIPOK }
	t.Cleanup(func() { getUserIP = origGetUserIP })

	marker := filepath.Join(t.TempDir(), "launched")
	t.Setenv(psqlHelperEnv, "1")
	t.Setenv(psqlHelperMarkerEnv, marker)

	exe, err := os.Executable()
	require.NoError(t, err)

	return PSQLTool(exe), func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}
}

func connectionInfoJSON(external string) string {
	b, _ := json.Marshal(map[string]string{
		"password":                 testPostgresPassword,
		"internalConnectionString": "postgresql://user:" + testPostgresPassword + "@dpg-12345678901234567890-a/db",
		"externalConnectionString": external,
		"psqlCommand":              "PGPASSWORD=" + testPostgresPassword + " psql -h dpg-12345678901234567890-a",
	})
	return string(b)
}

var unusableExternalTargets = []struct {
	name           string
	connectionInfo string
}{
	{name: "empty", connectionInfo: connectionInfoJSON("")},
	{name: "whitespace", connectionInfo: connectionInfoJSON(" \t\n ")},
	{name: "missing key", connectionInfo: `{"password":"` + testPostgresPassword + `","internalConnectionString":"postgresql://internal/db","psqlCommand":"psql"}`},
}

var ipDiscoveryOutcomes = []struct {
	name   string
	userIP net.IP
	ok     bool
}{
	{name: "ip discovery failed", userIP: nil, ok: false},
	{name: "ip allowed", userIP: net.ParseIP("192.0.2.77"), ok: true},
}

const allowAllIPs = `[{"cidrBlock":"0.0.0.0/0","description":"everywhere"}]`

func requireUnusableTargetError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no external connection string available for "+testPostgresName)
	require.NotContains(t, err.Error(), testPostgresPassword)
}

func TestExecutePSQLNonInteractive_RejectsUnusableExternalTarget(t *testing.T) {
	for _, target := range unusableExternalTargets {
		for _, ip := range ipDiscoveryOutcomes {
			t.Run(target.name+"/"+ip.name, func(t *testing.T) {
				tool, launched := setupPSQLTest(t, target.connectionInfo, ip.userIP, ip.ok, allowAllIPs)

				result, err := ExecutePSQLNonInteractive(context.Background(), &PSQLInput{
					PostgresIDOrName: testPostgresID,
					Tool:             tool,
					Command:          "SELECT 1",
				})

				requireUnusableTargetError(t, err)
				require.Nil(t, result)
				require.False(t, launched(), "child must not be launched without a usable target")
			})
		}
	}
}

func TestExecutePSQLNonInteractive_PassesExternalTarget(t *testing.T) {
	for _, ip := range ipDiscoveryOutcomes {
		t.Run(ip.name, func(t *testing.T) {
			tool, launched := setupPSQLTest(t, connectionInfoJSON(testExternalURL), ip.userIP, ip.ok, allowAllIPs)

			result, err := ExecutePSQLNonInteractive(context.Background(), &PSQLInput{
				PostgresIDOrName: testPostgresID,
				Tool:             tool,
				Command:          "SELECT 1",
				Args:             []string{"--csv"},
			})
			require.NoError(t, err)
			require.True(t, launched())

			var args []string
			require.NoError(t, json.Unmarshal([]byte(result.Output), &args))
			require.Equal(t, []string{testExternalURL, "-c", "SELECT 1", "--csv"}, args)
		})
	}
}

func TestExecutePSQLNonInteractive_IPNotAllowed(t *testing.T) {
	tool, launched := setupPSQLTest(t, connectionInfoJSON(testExternalURL), net.ParseIP("192.0.2.77"), true, `[]`)

	_, err := ExecutePSQLNonInteractive(context.Background(), &PSQLInput{
		PostgresIDOrName: testPostgresID,
		Tool:             tool,
		Command:          "SELECT 1",
	})
	require.EqualError(t, err, "IP address (192.0.2.77) not in allow list for "+testPostgresName)
	require.False(t, launched())
}

func TestLoadDataPSQL_RejectsUnusableExternalTarget(t *testing.T) {
	for _, tool := range []PSQLTool{PSQL, PGCLI} {
		for _, target := range unusableExternalTargets {
			for _, ip := range ipDiscoveryOutcomes {
				t.Run(string(tool)+"/"+target.name+"/"+ip.name, func(t *testing.T) {
					setupPSQLTest(t, target.connectionInfo, ip.userIP, ip.ok, allowAllIPs)

					cmd, err := loadDataPSQL(context.Background(), &PSQLInput{
						PostgresIDOrName: testPostgresID,
						Tool:             tool,
					})

					requireUnusableTargetError(t, err)
					require.Nil(t, cmd)
				})
			}
		}
	}
}

func TestLoadDataPSQL_PassesExternalTarget(t *testing.T) {
	for _, tool := range []PSQLTool{PSQL, PGCLI} {
		for _, ip := range ipDiscoveryOutcomes {
			t.Run(string(tool)+"/"+ip.name, func(t *testing.T) {
				setupPSQLTest(t, connectionInfoJSON(testExternalURL), ip.userIP, ip.ok, allowAllIPs)

				cmd, err := loadDataPSQL(context.Background(), &PSQLInput{
					PostgresIDOrName: testPostgresID,
					Tool:             tool,
					Args:             []string{"--single-transaction"},
				})
				require.NoError(t, err)
				require.Equal(t, []string{string(tool), testExternalURL, "--single-transaction"}, cmd.Args)
			})
		}
	}
}
