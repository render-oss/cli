package workflows

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/types"
	"github.com/render-oss/cli/pkg/workflows/store"
)

type TaskListInput struct {
	WorkflowVersionID string `cli:"arg:0"`
	WorkflowID        string
	LatestVersionOnly bool
	Local             bool
}

type TaskRunInput struct {
	TaskSlug string `cli:"arg:0"`
	Input    string `cli:"input"`
}

type TaskRunTargetInput struct {
	TaskRunID string `cli:"arg:0"`
}

func (t TaskRunTargetInput) Validate(_ bool) error {
	if !strings.HasPrefix(t.TaskRunID, store.TaskRunIDPrefix+"-") {
		return fmt.Errorf("invalid task run ID %q: must start with %q", t.TaskRunID, store.TaskRunIDPrefix+"-")
	}
	return nil
}

type VersionListInput struct {
	WorkflowID string `cli:"arg:0"`
}

type TaskRunListInput struct {
	TaskSlug string `cli:"arg:0"`
	Local    bool
}

type VersionReleaseInput struct {
	WorkflowID string  `cli:"arg:0"`
	CommitID   *string `cli:"commit"`
	Wait       bool    `cli:"wait"`
}

func (t TaskListInput) Validate(interactive bool) error {
	if !interactive && t.Local && t.WorkflowVersionID != "" {
		return errors.New("workflow version id is not supported in local mode")
	}
	if !interactive && !t.Local && t.WorkflowVersionID == "" {
		return errors.New("workflow version id must be specified when output is not interactive")
	}
	return nil
}

func (t TaskRunListInput) Validate(interactive bool) error {
	if !interactive && !t.Local && t.TaskSlug == "" {
		return errors.New("task slug must be specified when output is not interactive")
	}
	return nil
}

func (t TaskRunInput) Validate(interactive bool) error {
	if !interactive && t.TaskSlug == "" {
		return errors.New("task slug must be specified when output is not interactive")
	}

	if !interactive && t.Input == "" {
		return errors.New("input must be specified when output is not interactive")
	} else if t.Input != "" && !json.Valid([]byte(t.Input)) {
		return errors.New("input must be valid JSON")
	}

	return nil
}

func (v VersionListInput) Validate(interactive bool) error {
	if !interactive && v.WorkflowID == "" {
		return errors.New("workflow id must be specified when output is not interactive")
	}
	return nil
}

func (v VersionReleaseInput) String() []string {
	return []string{v.WorkflowID}
}

func (v VersionReleaseInput) Validate(isInteractive bool) error {
	if v.WorkflowID == "" {
		if types.IsNonZeroString(v.CommitID) {
			return errors.New("workflow id must be specified when commit is specified")
		}
		if v.Wait {
			return errors.New("workflow id must be specified when wait is true")
		}
		if !isInteractive {
			return errors.New("workflow id must be specified when output is not interactive")
		}
	}
	return nil
}
