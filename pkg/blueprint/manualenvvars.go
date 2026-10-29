package blueprint

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/render-oss/cli/internal/files"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/validate"
)

// manualEnvVarsDirName is the state subdirectory that holds manual env var files.
const manualEnvVarsDirName = "manual-env-vars"

// ManualEnvVarsFileHelpPath is the default location of a manual env var file,
// written for help text.
const ManualEnvVarsFileHelpPath = "~/.render/state/" + manualEnvVarsDirName + "/<blueprintID>.json"

// PrepareManualEnvVarsFile writes a manual env var file with empty string values
// when a successful create plan requires manual values. Existing files are
// replaced. It returns the file path on success, an empty path with no error
// when no file is needed, and an empty path with an error on failure.
func PrepareManualEnvVarsFile(result *bptypes.CreateBlueprintResponse) (string, error) {
	if result == nil || result.PlanningOutcome != bptypes.PlanningOutcomePlanned {
		return "", nil
	}
	sync := result.Sync
	if sync == nil || len(sync.RequiredManualEnvVars) == 0 {
		return "", nil
	}
	// The server-provided ID becomes a filename, so validate it before joining
	// it into a path. A malformed or malicious response could otherwise write
	// outside the manual env vars directory (for example, "../../x").
	if !validate.IsBlueprintID(result.Blueprint.Id) {
		return "", fmt.Errorf("prepare manual environment variables file: unexpected Blueprint ID %q", result.Blueprint.Id)
	}
	vars := make([]bptypes.SuppliedManualEnvVar, 0, len(sync.RequiredManualEnvVars))
	for _, required := range sync.RequiredManualEnvVars {
		vars = append(vars, bptypes.SuppliedManualEnvVar{
			ServiceType: required.ServiceType,
			ServiceName: required.ServiceName,
			Key:         required.Key,
			Value:       new(""),
		})
	}
	content, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return "", fmt.Errorf("format manual environment variables: %w", err)
	}
	stateDir, err := config.StateDir()
	if err != nil {
		return "", fmt.Errorf("resolve manual environment variables directory: %w", err)
	}
	filename := filepath.Join(stateDir, manualEnvVarsDirName, result.Blueprint.Id+".json")
	if err := files.Write(filename, append(content, '\n')); err != nil {
		return "", fmt.Errorf("write manual environment variables file %q: %w", filename, err)
	}
	return filename, nil
}
