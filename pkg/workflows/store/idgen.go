package store

import (
	"fmt"

	"github.com/rs/xid"
)

const (
	TaskIDPrefix    = "tsk"
	TaskRunIDPrefix = "trn"
	FlowIDPrefix    = "flw"
)

func NewObjectID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, xid.New().String())
}

func NewTaskID() string {
	return NewObjectID(TaskIDPrefix)
}

func NewFlowID() string {
	return NewObjectID(FlowIDPrefix)
}

func NewTaskRunID() string {
	return NewObjectID(TaskRunIDPrefix)
}
