package logs

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

type Log struct {
	ID        string
	TaskRunID string
	Message   string
	Timestamp time.Time
}

type Logs []*Log

func regexFromArray(array []string) string {
	escapedArray := make([]string, len(array))
	for i, s := range array {
		escapedArray[i] = regexp.QuoteMeta(s)
	}
	return fmt.Sprintf("(%s)", strings.Join(escapedArray, "|"))
}

func (l Log) checkCondition(search LogSearch) bool {
	if len(search.Text) > 0 && !regexp.MustCompile(regexFromArray(search.Text)).MatchString(l.Message) {
		return false
	}

	if len(search.TaskRunID) > 0 && !slices.Contains(search.TaskRunID, l.TaskRunID) {
		return false
	}

	if !search.StartTime.IsZero() && l.Timestamp.Before(search.StartTime) {
		return false
	}

	if !search.EndTime.IsZero() && l.Timestamp.After(search.EndTime) {
		return false
	}

	return true
}

func (l Logs) GetLogs(search LogSearch) []*Log {
	var filteredLogs []*Log
	for _, log := range l {
		if log.checkCondition(search) {
			filteredLogs = append(filteredLogs, log)
		}
	}

	return filteredLogs
}

type LogStreamer struct {
	readCh chan *Log
	filter LogSearch
}

func NewLogStore() *LogStore {
	return &LogStore{
		logs:    Logs{},
		logChan: make(chan *Log),
	}
}

type LogStore struct {
	logs         Logs
	logChan      chan *Log
	readChanLock sync.Mutex
	readChans    []*LogStreamer
}

func (l *LogStore) AddLog(log *Log) {
	l.logChan <- log
}

func (l *LogStore) LogChan(filter LogSearch) <-chan *Log {
	readChan := make(chan *Log)

	l.readChanLock.Lock()
	defer l.readChanLock.Unlock()
	l.readChans = append(l.readChans, &LogStreamer{readCh: readChan, filter: filter})

	// If a start time was set, ensure the client gets all the previous that match their filter
	// We get these logs under the lock to ensure that the client doesn't double receive logs.
	// The lock prevents the `Start` method from sending logs to the client while we're adding
	// the readChan to the list.
	if !filter.StartTime.IsZero() {
		previousLogs := l.logs.GetLogs(filter)
		go func() {
			for _, log := range previousLogs {
				readChan <- log
			}
		}()
	}

	return readChan
}

func (l *LogStore) RemoveLogChan(readChan <-chan *Log) {
	l.readChanLock.Lock()
	defer l.readChanLock.Unlock()
	var chanToRemove chan *Log
	l.readChans = slices.DeleteFunc(l.readChans, func(c *LogStreamer) bool {
		if c.readCh == readChan {
			chanToRemove = c.readCh
			return true
		}
		return false
	})
	close(chanToRemove)
}

type LogSearch struct {
	TaskRunID []string
	StartTime time.Time
	EndTime   time.Time
	Text      []string
}

func (l *LogStore) sendLogs(log *Log) {
	l.readChanLock.Lock()
	defer l.readChanLock.Unlock()
	for _, readChan := range l.readChans {
		if log.checkCondition(readChan.filter) {
			readChan.readCh <- log
		}
	}
}

func (l *LogStore) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case log := <-l.logChan:
				l.sendLogs(log)
				l.logs = append(l.logs, log)
			}
		}
	}()
}

type LogInterceptor struct {
	writer    io.Writer
	taskRunID string
	logs      *LogStore
}

func NewLogInterceptor(taskRunID string, writer io.Writer, logs *LogStore) *LogInterceptor {
	return &LogInterceptor{
		writer:    writer,
		taskRunID: taskRunID,
		logs:      logs,
	}
}

func (l *LogInterceptor) Write(p []byte) (n int, err error) {
	if l.writer != nil {
		if _, err := l.writer.Write(p); err != nil {
			return 0, err
		}
	}
	l.logs.AddLog(&Log{
		TaskRunID: l.taskRunID,
		Message:   string(p),
		Timestamp: time.Now(),
	})
	return len(p), nil
}

func (l *LogStore) GetLogs(search LogSearch) Logs {
	return l.logs.GetLogs(search)
}
