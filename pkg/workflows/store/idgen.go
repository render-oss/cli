package store

import (
	"fmt"

	"github.com/rs/xid"
)

const (
	TaskIDPrefix    = "tsk"
	TaskRunIDPrefix = "trn"
)

func NewObjectID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, xid.New().String())
}

func NewTaskID() string {
	return NewObjectID(TaskIDPrefix)
}

func NewTaskRunID() string {
	return NewObjectID(TaskRunIDPrefix)
}
