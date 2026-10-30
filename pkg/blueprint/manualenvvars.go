package blueprint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/render-oss/cli/internal/files"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/validate"
)

// manualEnvVarsDirName is the state subdirectory that holds manual env var files.
const manualEnvVarsDirName = "manual-env-vars"

// ManualEnvVarsFileHelpPath is the default location of a manual env var file,
// written for help text.
const ManualEnvVarsFileHelpPath = "~/.render/state/" + manualEnvVarsDirName + "/<blueprintID>.json"

// PrepareManualEnvVarsFile writes a manual env var file with empty string values
// when a successful plan requires manual values. Existing files are replaced.
// It returns the file path on success, an empty path with no error when no
// file is needed, and an empty path with an error on failure.
func PrepareManualEnvVarsFile(blueprintID string, outcome bptypes.PlanningOutcome, sync *bptypes.SyncWithPlan) (string, error) {
	if outcome != bptypes.PlanningOutcomePlanned || sync == nil || len(sync.RequiredManualEnvVars) == 0 {
		return "", nil
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
	filename, err := manualEnvVarsFilePath(blueprintID)
	if err != nil {
		return "", fmt.Errorf("prepare manual environment variables file: %w", err)
	}
	if err := files.Write(filename, append(content, '\n')); err != nil {
		return "", fmt.Errorf("write manual environment variables file \"%s\": %w", filename, err)
	}
	return filename, nil
}

// RemoveManualEnvVarsFile removes the Blueprint's default manual env var file,
// if there is one.
func RemoveManualEnvVarsFile(blueprintID string) error {
	filename, err := manualEnvVarsFilePath(blueprintID)
	if err != nil {
		return fmt.Errorf("remove manual environment variables file: %w", err)
	}
	err = os.Remove(filename)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		// There is no file to remove. ENOTDIR means a parent of filename is
		// not a directory, so the file cannot exist either.
		return nil
	}
	return err
}

// manualEnvVarsFilePath returns the default manual env var file for a
// Blueprint. The ID becomes a filename, so it is validated first; a malformed
// ID, such as "../../x" in a server response, could otherwise point outside
// the manual env vars directory.
func manualEnvVarsFilePath(blueprintID string) (string, error) {
	if !validate.IsBlueprintID(blueprintID) {
		return "", fmt.Errorf("unexpected Blueprint ID %q", blueprintID)
	}
	stateDir, err := config.StateDir()
	if err != nil {
		return "", fmt.Errorf("resolve manual environment variables directory: %w", err)
	}
	return filepath.Join(stateDir, manualEnvVarsDirName, blueprintID+".json"), nil
}

// noCleanup is the cleanup for values that did not come from the default file.
func noCleanup() error { return nil }

// ReadManualEnvVars reads the values to approve with. It reads the file or
// stdin ("-") named by --manual-env-vars-file, or else the default file for
// the Blueprint if it exists. values is nil when no values were supplied.
// Errors never include the input's contents, because it contains secret
// values.
//
// Call cleanup once the sync is approved. It deletes the default file so its
// secret values do not stay on disk, and does nothing when the values came
// from anywhere else; a file passed with --manual-env-vars-file belongs to the
// user. A cleanup error is an *fs.PathError naming the file.
func ReadManualEnvVars(input ApproveInput, stdin io.Reader) (values []bptypes.SuppliedManualEnvVar, cleanup func() error, err error) {
	if input.ManualEnvVarsFile != nil && *input.ManualEnvVarsFile == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, nil, fmt.Errorf("read manual environment variables from stdin: %w", err)
		}
		values, err := parseManualEnvVars(data)
		if err != nil {
			return nil, nil, fmt.Errorf("parse manual environment variables from stdin: %w", err)
		}
		return values, noCleanup, nil
	}

	if input.ManualEnvVarsFile != nil {
		filename, err := command.ExpandPath(*input.ManualEnvVarsFile)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve --manual-env-vars-file path: %w", err)
		}
		values, err := readManualEnvVarsFile(filename)
		if err != nil {
			return nil, nil, err
		}
		return values, noCleanup, nil
	}

	filename, err := manualEnvVarsFilePath(input.BlueprintID)
	if err != nil {
		return nil, nil, fmt.Errorf("read manual environment variables: %w", err)
	}
	values, err = readManualEnvVarsFile(filename)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, noCleanup, nil
	}
	if err != nil {
		return nil, nil, err
	}
	cleanup = func() error {
		if err := os.Remove(filename); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	return values, cleanup, nil
}

func readManualEnvVarsFile(filename string) ([]bptypes.SuppliedManualEnvVar, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read manual environment variables file: %w", err)
	}
	values, err := parseManualEnvVars(data)
	if err != nil {
		return nil, fmt.Errorf("parse manual environment variables file \"%s\": %w", filename, err)
	}
	return values, nil
}

// errManualEnvVarsShape describes the expected manual env vars JSON.
var errManualEnvVarsShape = errors.New("expected a JSON array of objects with serviceType, serviceName, key, and value")

// parseManualEnvVars checks only the JSON shape; the API validates entries.
// A top-level null is rejected rather than treated as no values.
//
// The input holds secret values, so errors must never repeat any of it.
// encoding/json errors can: for an unquoted value, the error quotes the value's
// first character. Instead of wrapping those errors, this rebuilds them from
// the parts that are safe to show, the position and the field, so people can
// still find the problem in their file. For example:
//
//	line 4, column 14: invalid JSON
//	line 4: [0].value must be a string
func parseManualEnvVars(data []byte) ([]bptypes.SuppliedManualEnvVar, error) {
	var values []bptypes.SuppliedManualEnvVar
	err := json.Unmarshal(data, &values)
	if err == nil {
		if values == nil {
			return nil, errManualEnvVarsShape
		}
		return values, nil
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		// Offset counts the bytes read, including the one that failed.
		line, column := lineColumn(data, int(syntaxErr.Offset)-1)
		return nil, fmt.Errorf("line %d, column %d: invalid JSON", line, column)
	// A field path without "." names an array element rather than a property,
	// so it falls through to the shape message below.
	case errors.As(err, &typeErr) && strings.Contains(typeErr.Field, "."):
		// Offset points past the value, so only its line is reliable.
		line, _ := lineColumn(data, int(typeErr.Offset)-1)
		index, field, _ := strings.Cut(typeErr.Field, ".")
		return nil, fmt.Errorf("line %d: [%s].%s must be a %s", line, index, field, typeErr.Type.Kind())
	default:
		return nil, errManualEnvVarsShape
	}
}

// lineColumn converts a byte offset in data to a 1-based line and column.
func lineColumn(data []byte, offset int) (line, column int) {
	before := data[:max(0, min(offset, len(data)))]
	line = bytes.Count(before, []byte("\n")) + 1
	column = utf8.RuneCount(before[bytes.LastIndexByte(before, '\n')+1:]) + 1
	return line, column
}
