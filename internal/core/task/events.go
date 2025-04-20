package task

import (
	"time"
)

const (
	// Event type constants
	EventTaskCreated        EventType = "task.created"
	EventTaskStarted        EventType = "task.started"
	EventTaskProgress       EventType = "task.progress"
	EventTaskCompleted      EventType = "task.completed"
	EventTaskFailed         EventType = "task.failed"
	EventTaskCancelled      EventType = "task.cancelled"
	EventRepositoryCloned   EventType = "repository.cloned"
	EventExtensionStarted   EventType = "extension.started"
	EventExtensionCompleted EventType = "extension.completed"
	EventExtensionFailed    EventType = "extension.failed"
)

func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		handlers: make(map[EventType][]EventHandler),
	}
}

func (e *EventEmitter) Subscribe(eventType EventType, handler EventHandler) {
	if e.handlers == nil {
		e.handlers = make(map[EventType][]EventHandler)
	}
	e.handlers[eventType] = append(e.handlers[eventType], handler)
}

func (e *EventEmitter) SubscribeAll(handler EventHandler) {
	e.Subscribe(EventTaskCreated, handler)
	e.Subscribe(EventTaskStarted, handler)
	e.Subscribe(EventTaskProgress, handler)
	e.Subscribe(EventTaskCompleted, handler)
	e.Subscribe(EventTaskFailed, handler)
	e.Subscribe(EventTaskCancelled, handler)
	e.Subscribe(EventRepositoryCloned, handler)
	e.Subscribe(EventExtensionStarted, handler)
	e.Subscribe(EventExtensionCompleted, handler)
	e.Subscribe(EventExtensionFailed, handler)
}

func (e *EventEmitter) Emit(eventType EventType, taskID string, data map[string]interface{}) {
	if e.handlers == nil {
		return
	}

	event := Event{
		Type:      eventType,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Data:      data,
	}

	for _, handler := range e.handlers[eventType] {
		go handler(event)
	}

	for _, handler := range e.handlers["*"] {
		go handler(event)
	}
}

func (e *EventEmitter) EmitTaskCreated(taskID string, description string, repository string) {
	e.Emit(EventTaskCreated, taskID, map[string]interface{}{
		"description": description,
		"repository":  repository,
	})
}

func (e *EventEmitter) EmitTaskStarted(taskID string) {
	e.Emit(EventTaskStarted, taskID, nil)
}

func (e *EventEmitter) EmitTaskProgress(taskID string, progress float64, message string) {
	e.Emit(EventTaskProgress, taskID, map[string]interface{}{
		"progress": progress,
		"message":  message,
	})
}

func (e *EventEmitter) EmitTaskCompleted(taskID string, result interface{}) {
	e.Emit(EventTaskCompleted, taskID, map[string]interface{}{
		"result": result,
	})
}

func (e *EventEmitter) EmitTaskFailed(taskID string, err error) {
	e.Emit(EventTaskFailed, taskID, map[string]interface{}{
		"error": err.Error(),
	})
}

func (e *EventEmitter) EmitRepositoryCloned(taskID string, repository string, path string) {
	e.Emit(EventRepositoryCloned, taskID, map[string]interface{}{
		"repository": repository,
		"path":       path,
	})
}
