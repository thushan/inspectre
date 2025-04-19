package task

import (
	"context"
	"fmt"
	"time"
)

// startWorkers initializes and starts the initial worker pool
func (m *Manager) startWorkers() {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	m.logger.Info("Starting worker pool with %d workers", m.minWorkerCount)

	// Start initial set of workers
	for i := 0; i < m.minWorkerCount; i++ {
		m.startWorkerLocked(i)
	}

	m.logger.Info("Worker pool started successfully with %d workers", m.currentWorkers)
}

// startWorkerLocked launches a new worker goroutine while holding the lock
func (m *Manager) startWorkerLocked(workerID int) {
	workerCtx, workerCancel := context.WithCancel(m.ctx)

	m.logger.Debug("Creating worker %d", workerID)

	m.workerStates[workerID] = &WorkerState{
		ID:        workerID,
		IsIdle:    true,
		LastUsed:  time.Now(),
		TaskCount: 0,
		Cancel:    workerCancel,
	}

	m.currentWorkers++

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			// Recover from panics to prevent goroutine leaks
			if r := recover(); r != nil {
				m.logger.Error("Worker %d panic: %v", workerID, r)
			}
		}()

		m.logger.Debug("Worker %d goroutine started", workerID)
		m.workerLoop(workerCtx, workerID)

		// Worker is exiting, clean up its state
		m.workersMu.Lock()
		delete(m.workerStates, workerID)
		m.currentWorkers--
		m.workersMu.Unlock()

		m.logger.Debug("Worker %d exited", workerID)
	}()

	m.logger.Debug("Worker %d created and started", workerID)
}

// startWorker launches a new worker goroutine
func (m *Manager) startWorker() {
	m.workersMu.Lock()
	workerID := m.currentWorkers
	m.startWorkerLocked(workerID)
	m.workersMu.Unlock()
}

// stopWorker terminates a worker
func (m *Manager) stopWorker(workerID int) error {
	m.workersMu.Lock()
	worker, exists := m.workerStates[workerID]
	if !exists {
		m.workersMu.Unlock()
		return fmt.Errorf("worker %d not found", workerID)
	}

	// Signal worker to cancel
	worker.Cancel()
	m.workersMu.Unlock()

	m.logger.Debug("Sent cancellation signal to worker %d", workerID)
	return nil
}

// workerLoop is the main execution loop for a worker
func (m *Manager) workerLoop(ctx context.Context, workerID int) {
	m.logger.Debug("Worker %d started processing loop", workerID)

	for {
		// Mark worker as idle while waiting for a task
		m.setWorkerIdle(workerID, true)
		m.logger.Debug("Worker %d waiting for task", workerID)

		select {
		case <-ctx.Done():
			m.logger.Debug("Worker %d shutting down: %v", workerID, ctx.Err())
			return

		case task, ok := <-m.taskQueue:
			if !ok {
				// Channel closed, exit worker
				m.logger.Debug("Worker %d exiting: task queue closed", workerID)
				return
			}

			m.logger.Debug("Worker %d received task %s", workerID, task.ID)

			// Mark worker as busy
			m.setWorkerIdle(workerID, false)

			// Check if this worker should handle this task
			m.tasksMu.Lock()
			if task.Status != StatusQueued {
				// Task was cancelled or already processed
				m.logger.Warning("Worker %d skipping task %s: status is %s (not queued)",
					workerID, task.ID, task.Status)
				m.tasksMu.Unlock()
				continue
			}

			// Mark as running
			task.Status = StatusRunning
			m.tasksMu.Unlock()

			// Log start of processing
			m.logger.TaskInfo(task.ID, "Worker %d processing task %s for repository %s",
				workerID, task.ID, task.Repository)

			// Send UI event
			m.sendUIEvent(UIEvent{
				TaskID:    task.ID,
				EventType: "started",
				Message:   fmt.Sprintf("Processing repository %s", task.Repository),
			})

			// Create a task-specific context with timeout
			_, taskCancel := context.WithTimeout(ctx, TaskExecutionTimeout)

			// Execute the task
			m.logger.Debug("Worker %d executing task %s", workerID, task.ID)
			result := m.executeTask(task)
			m.logger.Debug("Worker %d completed execution of task %s", workerID, task.ID)

			// Always cancel the task context
			taskCancel()

			// Update worker state
			m.workersMu.Lock()
			if state, exists := m.workerStates[workerID]; exists {
				state.TaskCount++
				state.LastUsed = time.Now()
			}
			m.workersMu.Unlock()

			// Send result for processing with timeout to prevent deadlocks
			m.logger.Debug("Worker %d sending results for task %s", workerID, task.ID)
			select {
			case m.taskResults <- result:
				m.logger.Debug("Worker %d: results for task %s sent successfully", workerID, task.ID)
			case <-ctx.Done():
				// Context cancelled, exit worker
				m.logger.Warning("Worker %d exiting during result send: context cancelled", workerID)
				return
			case <-time.After(5 * time.Second):
				// Couldn't send results after timeout, log and continue
				m.logger.Warning("Worker %d: timeout sending results for task %s, dropping results",
					workerID, task.ID)
			}
		}
	}
}

// startWorkerScaling starts the worker scaling goroutine
func (m *Manager) startWorkerScaling() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Panic in worker scaling goroutine: %v", r)
			}
		}()

		ticker := time.NewTicker(WorkerScaleInterval)
		defer ticker.Stop()

		m.logger.Info("Worker scaling goroutine started")

		for {
			select {
			case <-m.ctx.Done():
				m.logger.Info("Worker scaling goroutine shutting down: context cancelled")
				return
			case <-ticker.C:
				m.scaleWorkers()
			}
		}
	}()
}

// scaleWorkers adjusts the number of workers based on load
func (m *Manager) scaleWorkers() {
	// First gather data about workers under a read lock
	m.workersMu.RLock()

	// Skip if shutting down
	if m.closed {
		m.workersMu.RUnlock()
		return
	}

	// Count idle workers and identify idle ones to potentially terminate
	idleCount := 0
	busyCount := 0
	idleTooLong := make([]int, 0)

	now := time.Now()
	for id, state := range m.workerStates {
		if state.IsIdle {
			idleCount++
			idleTime := now.Sub(state.LastUsed)

			// If worker has been idle for too long and we're above min count, mark for removal
			if idleTime > IdleWorkerTimeout && m.currentWorkers > m.minWorkerCount {
				idleTooLong = append(idleTooLong, id)
			}
		} else {
			busyCount++
		}
	}

	// Check if we need to scale up (all workers are busy)
	scaleUp := idleCount == 0 && m.currentWorkers < m.maxWorkerCount

	// Release the read lock
	m.workersMu.RUnlock()

	// Scale up if needed
	if scaleUp {
		m.logger.Debug("Scaling up workers: %d -> %d", m.currentWorkers, m.currentWorkers+1)
		m.startWorker()
	}

	// Scale down by terminating idle workers if we have too many
	for _, id := range idleTooLong {
		// Double-check with lock that worker still exists and we're still above min
		m.workersMu.Lock()
		shouldStop := false
		if _, exists := m.workerStates[id]; exists && m.currentWorkers > m.minWorkerCount {
			shouldStop = true
		}
		m.workersMu.Unlock()

		if shouldStop {
			m.logger.Debug("Scaling down: terminating idle worker %d", id)
			m.stopWorker(id)
		}
	}
}

// setWorkerIdle updates the worker's idle state
func (m *Manager) setWorkerIdle(workerID int, idle bool) {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	if state, exists := m.workerStates[workerID]; exists {
		state.IsIdle = idle
		if idle {
			state.LastUsed = time.Now()
		}
	}
}
