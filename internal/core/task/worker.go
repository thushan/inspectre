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
		m.startWorker()
	}

	m.logger.Info("Worker pool started successfully with %d workers", m.currentWorkers)
}

// startWorker launches a new worker goroutine
func (m *Manager) startWorker() {
	workerID := m.currentWorkers
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

			// Execute the task
			m.logger.Debug("Worker %d executing task %s", workerID, task.ID)
			result := m.executeTask(task)
			m.logger.Debug("Worker %d completed execution of task %s", workerID, task.ID)

			// Update worker state
			m.workersMu.Lock()
			if state, exists := m.workerStates[workerID]; exists {
				state.TaskCount++
				state.LastUsed = time.Now()
			}
			m.workersMu.Unlock()

			// Send result for processing
			m.logger.Debug("Worker %d sending results for task %s", workerID, task.ID)
			select {
			case m.taskResults <- result:
				m.logger.Debug("Worker %d: results for task %s sent successfully", workerID, task.ID)
			case <-ctx.Done():
				// Context cancelled, exit worker
				m.logger.Warning("Worker %d exiting during result send: context cancelled", workerID)
				return
			}
		}
	}
}

// startWorkerScaling starts the worker scaling goroutine
func (m *Manager) startWorkerScaling() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		ticker := time.NewTicker(WorkerScaleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				m.scaleWorkers()
			}
		}
	}()
}

// scaleWorkers adjusts the number of workers based on load
func (m *Manager) scaleWorkers() {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()

	// Count idle workers and identify idle ones to potentially terminate
	idleCount := 0
	busyCount := 0

	now := time.Now()
	var idleTooLong []int

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
	if idleCount == 0 && m.currentWorkers < m.maxWorkerCount {
		m.logger.Debug("Scaling up workers: %d -> %d", m.currentWorkers, m.currentWorkers+1)
		m.workersMu.Unlock()
		m.startWorker()
		m.workersMu.Lock()
	}

	// Scale down by terminating idle workers if we have too many
	for _, id := range idleTooLong {
		if worker, exists := m.workerStates[id]; exists && m.currentWorkers > m.minWorkerCount {
			m.logger.Debug("Scaling down: terminating idle worker %d", id)
			worker.Cancel()
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
