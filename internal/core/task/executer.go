package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thushan/inspectre/internal/core/types"
)

type Executor struct {
	eventEmitter *EventEmitter
	workerPool   *WorkerPool
}

func NewExecutor(poolSize int) (*Executor, error) {
	wp, err := NewWorkerPool(poolSize)
	if err != nil {
		return nil, err
	}

	return &Executor{
		eventEmitter: NewEventEmitter(),
		workerPool:   wp,
	}, nil
}

func (e *Executor) Execute(task *Task, fn func(ctx context.Context) error, options *types.TaskOptions) error {
	if options == nil {
		options = &types.TaskOptions{
			Timeout:    30 * time.Minute,
			MaxRetries: 0,
		}
	}

	task.Status = TaskQueued
	e.eventEmitter.EmitTaskCreated(task.ID, task.Description, task.Repository)

	return e.workerPool.Submit(func() {
		e.executeWithRetries(task, fn, options)
	})
}

func (e *Executor) executeWithRetries(task *Task, fn func(ctx context.Context) error, options *types.TaskOptions) {
	var err error
	retryCount := 0

	for retryCount <= options.MaxRetries {
		// If this is a retry, wait before trying again
		if retryCount > 0 {
			// Simple exponential backoff
			backoffTime := time.Duration(1<<uint(retryCount)) * time.Second
			time.Sleep(backoffTime)

			e.eventEmitter.Emit(EventType("task.retry"), task.ID, map[string]interface{}{
				"retry_count": retryCount,
				"max_retries": options.MaxRetries,
				"backoff":     backoffTime.String(),
			})
		}

		err = e.executeOnce(task, fn, options)

		if err == nil || errors.Is(err, context.Canceled) {
			break
		}

		retryCount++
	}

	if err != nil {
		task.Status = TaskFailed
		task.Error = err
		e.eventEmitter.EmitTaskFailed(task.ID, err)
	}
}

func (e *Executor) executeOnce(task *Task, fn func(ctx context.Context) error, options *types.TaskOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	task.Status = TaskRunning
	task.StartTime = time.Now()
	e.eventEmitter.EmitTaskStarted(task.ID)

	err := fn(ctx)

	task.EndTime = time.Now()

	if err != nil {
		return fmt.Errorf("task execution error: %w", err)
	}

	task.Status = TaskCompleted
	e.eventEmitter.EmitTaskCompleted(task.ID, task.Result)

	return nil
}

func (e *Executor) Cleanup() {
	e.workerPool.Release()
}

func (e *Executor) RegisterUIHandlers(progressTracker func(event Event)) {
	e.eventEmitter.Subscribe(EventTaskProgress, progressTracker)
	e.eventEmitter.Subscribe(EventTaskStarted, progressTracker)
	e.eventEmitter.Subscribe(EventTaskCompleted, progressTracker)
	e.eventEmitter.Subscribe(EventTaskFailed, progressTracker)
}
