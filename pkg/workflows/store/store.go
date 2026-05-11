package store

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

type Task struct {
	ID          string
	Name        string
	CreatedAt   time.Time
	RetryConfig *taskserver.RetryConfig
}

type TaskAttempt struct {
	StartedAt   *time.Time
	CompletedAt *time.Time
	Status      TaskRunStatus
	Error       *string
}

type TaskRun struct {
	ID       string
	TaskName string
	Input    json.RawMessage
	Output   json.RawMessage
	Status   TaskRunStatus
	Error    *string

	// StartedAt is when the run first began; it never changes once set.
	StartedAt *time.Time
	// AttemptStartedAt is when the current attempt began; reset on each retry.
	AttemptStartedAt *time.Time
	CompletedAt      *time.Time

	ParentTaskRunID *string
	// RootTaskRunID is the ID of the top-level task run that initiated this
	// chain of subtasks. For root tasks, this equals ID.
	RootTaskRunID string

	// Attempts holds the history of previous (failed) attempts for this task
	// run. The current attempt's state is represented by the top-level fields.
	Attempts []TaskAttempt
}

type TaskRunStatus string

const (
	TaskRunStatusPending  TaskRunStatus = "pending"
	TaskRunStatusRunning  TaskRunStatus = "running"
	TaskRunStatusComplete TaskRunStatus = "complete"
	TaskRunStatusFailed   TaskRunStatus = "failed"
	TaskRunStatusCanceled TaskRunStatus = "canceled"
)

type TaskStore struct {
	tasks        map[string]*Task
	taskRuns     []*TaskRun
	taskRunChans []chan *TaskRun

	mu sync.Mutex
}

func NewTaskStore() *TaskStore {
	return &TaskStore{}
}

func (s *TaskStore) SetTasks(tasks []taskserver.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newTasks := make(map[string]*Task)

	for _, task := range tasks {
		var retryConfig *taskserver.RetryConfig
		if task.Options != nil {
			retryConfig = task.Options.Retry
		}
		if existing, ok := s.tasks[task.Name]; !ok {
			newTasks[task.Name] = &Task{
				ID:          NewTaskID(),
				Name:        task.Name,
				CreatedAt:   time.Now(),
				RetryConfig: retryConfig,
			}
		} else {
			existing.RetryConfig = retryConfig
			newTasks[task.Name] = existing
		}
	}

	s.tasks = newTasks
}

func (s *TaskStore) StartTaskRun(taskName string, input []byte, parentTaskRunID *string) *TaskRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := NewTaskRunID()

	// Propagate the root task run ID from the parent. For root tasks (no
	// parent), the task run is its own root.
	rootTaskRunID := id
	if parentTaskRunID != nil {
		if parent := s.getTaskRun(*parentTaskRunID); parent != nil {
			rootTaskRunID = parent.RootTaskRunID
		}
	}

	now := time.Now()
	taskRun := &TaskRun{
		ID:       id,
		TaskName: taskName,
		Input:    input,
		Status:   TaskRunStatusRunning,

		StartedAt:        &now,
		AttemptStartedAt: &now,

		ParentTaskRunID: parentTaskRunID,
		RootTaskRunID:   rootTaskRunID,
	}

	s.taskRuns = append(s.taskRuns, taskRun)

	return taskRun
}

func (s *TaskStore) updateTaskRun(taskRunID string, output []byte, errString *string, status TaskRunStatus) (*TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, taskRun := range s.taskRuns {
		if taskRun.ID == taskRunID {
			taskRun.Output = output
			taskRun.Status = status
			taskRun.Error = errString
			taskRun.CompletedAt = pointers.From(time.Now())

			return taskRun, nil
		}
	}

	return nil, fmt.Errorf("task run not found")
}

func (s *TaskStore) CompleteTaskRun(taskRunID string, output []byte) (*TaskRun, error) {
	taskRun, err := s.updateTaskRun(taskRunID, output, nil, TaskRunStatusComplete)
	if err != nil {
		return nil, err
	}

	s.sendResultsToChannels(taskRun)

	return taskRun, nil
}

func (s *TaskStore) FailTaskRun(taskRunID string, errString string) (*TaskRun, error) {
	taskRun, err := s.updateTaskRun(taskRunID, nil, &errString, TaskRunStatusFailed)
	if err != nil {
		return nil, err
	}

	s.sendResultsToChannels(taskRun)

	return taskRun, nil
}

// RetryTaskRun archives the current (failed) attempt into Attempts, resets
// the per-attempt state, and transitions the task run to pending so it can
// await its next attempt. StartedAt is preserved.
func (s *TaskStore) RetryTaskRun(taskRunID string) (*TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskRun := s.getTaskRun(taskRunID)
	if taskRun == nil {
		return nil, fmt.Errorf("task run not found")
	}
	if taskRun.Status != TaskRunStatusFailed {
		return nil, fmt.Errorf("cannot retry task run in status %q", taskRun.Status)
	}

	taskRun.Attempts = append(taskRun.Attempts, TaskAttempt{
		StartedAt:   taskRun.AttemptStartedAt,
		CompletedAt: taskRun.CompletedAt,
		Status:      taskRun.Status,
		Error:       taskRun.Error,
	})

	taskRun.Status = TaskRunStatusPending
	taskRun.Error = nil
	taskRun.CompletedAt = nil
	taskRun.Output = nil
	taskRun.AttemptStartedAt = nil

	return taskRun, nil
}

// MarkRunningTaskRun marks a pending task run as running for its next attempt and
// captures the attempt's start time.
func (s *TaskStore) MarkRunningTaskRun(taskRunID string) (*TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskRun := s.getTaskRun(taskRunID)
	if taskRun == nil {
		return nil, fmt.Errorf("task run not found")
	}

	now := time.Now()
	taskRun.Status = TaskRunStatusRunning
	taskRun.AttemptStartedAt = &now

	return taskRun, nil
}

func (s *TaskStore) CancelTaskRun(taskRunID string) (*TaskRun, error) {
	taskRun, err := s.updateTaskRun(taskRunID, nil, nil, TaskRunStatusCanceled)
	if err != nil {
		return nil, err
	}

	s.sendResultsToChannels(taskRun)

	return taskRun, nil
}

func (s *TaskStore) getTaskRun(taskRunID string) *TaskRun {
	for i := range s.taskRuns {
		if s.taskRuns[i].ID == taskRunID {
			return s.taskRuns[i]
		}
	}
	return nil
}

func (s *TaskStore) GetTaskRun(taskRunID string) *TaskRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getTaskRun(taskRunID)
}

func (s *TaskStore) GetTasks() []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tasks []*Task
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

func (s *TaskStore) GetTaskRuns(taskNameOrID string) []*TaskRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Resolve taskID to task name, matching by ID first, then by name
	var taskName string
	for _, task := range s.tasks {
		if task.ID == taskNameOrID {
			taskName = task.Name
			break
		}
	}
	if taskName == "" {
		for _, task := range s.tasks {
			if task.Name == taskNameOrID {
				taskName = task.Name
				break
			}
		}
	}

	taskRuns := make([]*TaskRun, 0)
	for _, taskRun := range s.taskRuns {
		if taskRun.TaskName == taskName {
			taskRuns = append(taskRuns, taskRun)
		}
	}

	return taskRuns
}

func (s *TaskStore) GetAllTaskRuns() []*TaskRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskRuns := make([]*TaskRun, len(s.taskRuns))
	copy(taskRuns, s.taskRuns)
	return taskRuns
}

func (s *TaskStore) GetTask(taskID string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Strip workflow prefix (e.g. "workflow-name/task-name") and match by
	// plain task name so production slugs work against local dev.
	var name string
	hasStrippedName := false
	if idx := strings.LastIndex(taskID, "/"); idx != -1 {
		name = taskID[idx+1:]
		hasStrippedName = true
	}

	for i := range s.tasks {
		task := s.tasks[i]

		if task.ID == taskID || task.Name == taskID || (hasStrippedName && task.Name == name) {
			return task
		}
	}

	return nil
}

func (s *TaskStore) GetTaskByName(taskName string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].Name == taskName {
			return s.tasks[i]
		}
	}

	return nil
}

func (s *TaskStore) AddTaskRunChan(ch chan *TaskRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskRunChans = append(s.taskRunChans, ch)
}

func (s *TaskStore) RemoveTaskRunChan(ch chan *TaskRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskRunChans = slices.DeleteFunc(s.taskRunChans, func(c chan *TaskRun) bool {
		return c == ch
	})
}

func (s *TaskStore) sendResultsToChannels(result *TaskRun) {
	for _, ch := range s.taskRunChans {
		ch <- result
	}
}
