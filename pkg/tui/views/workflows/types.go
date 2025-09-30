package workflows

import (
	"encoding/json"
	"errors"

	"github.com/render-oss/cli/pkg/types"
)

type TaskListInput struct {
	WorkflowVersionID string `cli:"arg:0"`
}

type TaskRunInput struct {
	TaskID string `cli:"arg:0"`
	Input  string `cli:"input"`
}

type TaskRunDetailsInput struct {
	TaskRunID string `cli:"arg:0"`
}

type VersionListInput struct {
	WorkflowID string `cli:"arg:0"`
}

type TaskRunListInput struct {
	TaskID string `cli:"arg:0"`
}

type VersionReleaseInput struct {
	WorkflowID string  `cli:"arg:0"`
	CommitID   *string `cli:"commit"`
	Wait       bool    `cli:"wait"`
}

func (t TaskListInput) Validate(interactive bool) error {
	if !interactive && t.WorkflowVersionID == "" {
		return errors.New("workflow version id must be specified when output is not interactive")
	}
	return nil
}

func (t TaskRunListInput) Validate(interactive bool) error {
	if !interactive && t.TaskID == "" {
		return errors.New("task id must be specified when output is not interactive")
	}
	return nil
}

func (t TaskRunInput) Validate(interactive bool) error {
	if !interactive && t.TaskID == "" {
		return errors.New("service id must be specified when output is not interactive")
	}

	if !interactive && t.Input == "" {
		return errors.New("input must be specified when output is not interactive")
	} else if !json.Valid([]byte(t.Input)) {
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
