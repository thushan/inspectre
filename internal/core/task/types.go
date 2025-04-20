package task

import (
	"time"
)

type TaskStatus string
type EventType string

type Event struct {
	Type      EventType
	TaskID    string
	Timestamp time.Time
	Data      map[string]interface{}
}

type EventHandler func(Event)

type EventEmitter struct {
	handlers map[EventType][]EventHandler
}

const (
	TaskCreated   TaskStatus = "created"
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID           string
	Description  string
	Status       TaskStatus
	Repository   string
	StartTime    time.Time
	EndTime      time.Time
	Progress     float64
	Error        error
	Dependencies []string
	Result       any
}
