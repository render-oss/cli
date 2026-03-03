package workflows

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskListInputValidate(t *testing.T) {
	t.Run("interactive mode allows empty ID", func(t *testing.T) {
		input := TaskListInput{}
		assert.NoError(t, input.Validate(true))
	})

	t.Run("non-interactive requires workflow version ID", func(t *testing.T) {
		input := TaskListInput{}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow version id must be specified")
	})

	t.Run("non-interactive with ID succeeds", func(t *testing.T) {
		input := TaskListInput{WorkflowVersionID: "wfv-123"}
		assert.NoError(t, input.Validate(false))
	})
}

func TestTaskRunInputValidate(t *testing.T) {
	t.Run("non-interactive missing task ID", func(t *testing.T) {
		input := TaskRunInput{Input: `["a"]`}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task id must be specified")
	})

	t.Run("non-interactive missing input", func(t *testing.T) {
		input := TaskRunInput{TaskID: "tsk-1"}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "input must be specified")
	})

	t.Run("invalid JSON input errors even in interactive mode", func(t *testing.T) {
		input := TaskRunInput{TaskID: "tsk-1", Input: "not-json"}
		err := input.Validate(true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "input must be valid JSON")
	})

	t.Run("valid array JSON", func(t *testing.T) {
		input := TaskRunInput{TaskID: "tsk-1", Input: `["a","b"]`}
		assert.NoError(t, input.Validate(false))
	})

	t.Run("valid object JSON", func(t *testing.T) {
		input := TaskRunInput{TaskID: "tsk-1", Input: `{"k":"v"}`}
		assert.NoError(t, input.Validate(false))
	})
}

func TestTaskRunListInputValidate(t *testing.T) {
	t.Run("interactive allows empty task ID", func(t *testing.T) {
		input := TaskRunListInput{}
		assert.NoError(t, input.Validate(true))
	})

	t.Run("non-interactive requires task ID", func(t *testing.T) {
		input := TaskRunListInput{}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task id must be specified")
	})

	t.Run("non-interactive with task ID succeeds", func(t *testing.T) {
		input := TaskRunListInput{TaskID: "tsk-1"}
		assert.NoError(t, input.Validate(false))
	})
}

func TestVersionListInputValidate(t *testing.T) {
	t.Run("interactive allows empty workflow ID", func(t *testing.T) {
		input := VersionListInput{}
		assert.NoError(t, input.Validate(true))
	})

	t.Run("non-interactive requires workflow ID", func(t *testing.T) {
		input := VersionListInput{}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow id must be specified")
	})

	t.Run("non-interactive with workflow ID succeeds", func(t *testing.T) {
		input := VersionListInput{WorkflowID: "wf-1"}
		assert.NoError(t, input.Validate(false))
	})
}

func TestVersionReleaseInputValidate(t *testing.T) {
	commitID := "abc123"

	t.Run("interactive with empty workflow ID and no commit/wait", func(t *testing.T) {
		input := VersionReleaseInput{}
		assert.NoError(t, input.Validate(true))
	})

	t.Run("empty workflow ID with commit set errors", func(t *testing.T) {
		input := VersionReleaseInput{CommitID: &commitID}
		err := input.Validate(true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow id must be specified when commit is specified")
	})

	t.Run("empty workflow ID with wait true errors", func(t *testing.T) {
		input := VersionReleaseInput{Wait: true}
		err := input.Validate(true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow id must be specified when wait is true")
	})

	t.Run("non-interactive with empty workflow ID errors", func(t *testing.T) {
		input := VersionReleaseInput{}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow id must be specified when output is not interactive")
	})

	t.Run("non-interactive with workflow ID succeeds", func(t *testing.T) {
		input := VersionReleaseInput{WorkflowID: "wf-1"}
		assert.NoError(t, input.Validate(false))
	})
}
