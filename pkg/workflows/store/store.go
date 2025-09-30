package store

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/workflows/taskserver"
)

type Task struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type TaskRun struct {
	ID       string
	TaskName string
	Input    json.RawMessage
	Output   json.RawMessage
	Status   TaskRunStatus
	Error    *string

	StartedAt   *time.Time
	CompletedAt *time.Time

	ParentTaskRunID *string
}

type TaskRunStatus string

const (
	TaskRunStatusRunning  TaskRunStatus = "running"
	TaskRunStatusComplete TaskRunStatus = "complete"
	TaskRunStatusFailed   TaskRunStatus = "failed"
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
		if _, ok := s.tasks[task.Name]; !ok {
			newTasks[task.Name] = &Task{
				ID:        NewTaskID(),
				Name:      task.Name,
				CreatedAt: time.Now(),
			}
		} else {
			newTasks[task.Name] = s.tasks[task.Name]
		}
	}

	s.tasks = newTasks
}

func (s *TaskStore) StartTaskRun(taskName string, input []byte, parentTaskRunID *string) *TaskRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskRun := &TaskRun{
		ID:       NewTaskRunID(),
		TaskName: taskName,
		Input:    input,
		Status:   TaskRunStatusRunning,

		StartedAt: pointers.From(time.Now()),

		ParentTaskRunID: parentTaskRunID,
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

func (s *TaskStore) CompleteTaskRun(taskRunID string, output []byte) error {
	taskRun, err := s.updateTaskRun(taskRunID, output, nil, TaskRunStatusComplete)
	if err != nil {
		return err
	}

	s.sendResultsToChannels(taskRun)

	return nil
}

func (s *TaskStore) FailTaskRun(taskRunID string, errString string) error {
	taskRun, err := s.updateTaskRun(taskRunID, nil, &errString, TaskRunStatusFailed)
	if err != nil {
		return err
	}

	s.sendResultsToChannels(taskRun)

	return nil
}

func (s *TaskStore) GetTaskRun(taskRunID string) *TaskRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.taskRuns {
		if s.taskRuns[i].ID == taskRunID {
			return s.taskRuns[i]
		}
	}

	return nil
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

func (s *TaskStore) GetTaskRuns(taskID string) []*TaskRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	var taskName string
	for _, task := range s.tasks {
		if task.ID == taskID {
			taskName = task.Name
			break
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

func (s *TaskStore) GetTask(taskID string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].ID == taskID {
			return s.tasks[i]
		}
	}

	for _, task := range s.tasks {
		if regexp.MustCompile(fmt.Sprintf("^.*/%s(:.*)?$", task.Name)).MatchString(taskID) {
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
