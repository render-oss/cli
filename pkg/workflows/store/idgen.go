package store

import (
	"fmt"

	"github.com/google/uuid"
)

const (
	TaskIDPrefix    = "tsk"
	TaskRunIDPrefix = "trn"
)

func NewObjectID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String())
}

func NewTaskID() string {
	return NewObjectID(TaskIDPrefix)
}

func NewTaskRunID() string {
	return NewObjectID(TaskRunIDPrefix)
}
