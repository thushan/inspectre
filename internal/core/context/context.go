package context

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/thushan/inspectre/internal/core/logging"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful shutdown
	DefaultShutdownTimeout = 10 * time.Second
)

var (
	// ErrShutdownInProgress is returned when trying to add hooks during shutdown
	ErrShutdownInProgress = errors.New("shutdown already in progress")

	// ErrShutdownTimedOut is returned when shutdown exceeds timeout
	ErrShutdownTimedOut = errors.New("shutdown timed out")
)

// ShutdownPriority defines priority levels for shutdown hooks
type ShutdownPriority int

const (
	// PriorityHighest should be used for hooks that must run first
	PriorityHighest ShutdownPriority = 100

	// PriorityHigh for hooks that should run early (e.g., saving state)
	PriorityHigh ShutdownPriority = 75

	// PriorityNormal for standard shutdown hooks
	PriorityNormal ShutdownPriority = 50

	// PriorityLow for hooks that should run late
	PriorityLow ShutdownPriority = 25

	// PriorityLowest for hooks that run at the very end (cleanup tasks)
	PriorityLowest ShutdownPriority = 0
)

// ShutdownHandler is a function that is called during shutdown
type ShutdownHandler struct {
	Name     string                      // A name for the handler
	Priority ShutdownPriority            // Execution priority (higher runs first)
	Handler  func(context.Context) error // The actual handler function
}

// AppContext manages application context and graceful shutdowns
type AppContext struct {
	ctx            context.Context
	cancel         context.CancelFunc
	shutdownWg     sync.WaitGroup
	shutdownHooks  []ShutdownHandler
	hooksMu        sync.RWMutex
	shutdownOnce   sync.Once
	isShuttingDown bool
	logger         *logging.Logger
	timeout        time.Duration
	signalCh       chan os.Signal
}

// NewAppContext creates a new application context
func NewAppContext(timeout time.Duration) *AppContext {
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalCh := make(chan os.Signal, 1)

	// Register for common termination signals
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	appCtx := &AppContext{
		ctx:            ctx,
		cancel:         cancel,
		timeout:        timeout,
		signalCh:       signalCh,
		logger:         logging.GetLogger(),
		isShuttingDown: false,
	}

	// Start signal handler
	go appCtx.handleSignals()

	return appCtx
}

// handleSignals handles OS signals
func (a *AppContext) handleSignals() {
	// Get first signal
	sig := <-a.signalCh
	a.logger.Info("Received signal: %s", sig)

	// Initiate graceful shutdown
	a.Shutdown()

	// If we get another signal during shutdown, force exit
	sig = <-a.signalCh
	a.logger.Warning("Received second signal: %s, forcing exit", sig)
	os.Exit(1)
}

// AddShutdownHook adds a function to be called during shutdown
func (a *AppContext) AddShutdownHook(handler func(context.Context) error) {
	a.AddShutdownHookWithPriority("anonymous", PriorityNormal, handler)
}

// AddShutdownHookWithPriority adds a named shutdown hook with specified priority
func (a *AppContext) AddShutdownHookWithPriority(name string, priority ShutdownPriority, handler func(context.Context) error) error {
	a.hooksMu.Lock()
	defer a.hooksMu.Unlock()

	// Check if shutdown is already in progress
	if a.isShuttingDown {
		return ErrShutdownInProgress
	}

	a.shutdownHooks = append(a.shutdownHooks, ShutdownHandler{
		Name:     name,
		Priority: priority,
		Handler:  handler,
	})

	return nil
}

// IncrementWaitGroup increments the wait group counter
func (a *AppContext) IncrementWaitGroup() {
	a.shutdownWg.Add(1)
}

// DecrementWaitGroup decrements the wait group counter
func (a *AppContext) DecrementWaitGroup() {
	a.shutdownWg.Done()
}

// Done returns a channel that's closed when the context is done
func (a *AppContext) Done() <-chan struct{} {
	return a.ctx.Done()
}

// Context returns the underlying context
func (a *AppContext) Context() context.Context {
	return a.ctx
}

// Shutdown initiates a graceful shutdown
func (a *AppContext) Shutdown() {
	var err error

	a.shutdownOnce.Do(func() {
		a.logger.Info("Initiating graceful shutdown...")

		// Mark as shutting down to prevent new hooks
		a.hooksMu.Lock()
		a.isShuttingDown = true
		hooks := make([]ShutdownHandler, len(a.shutdownHooks))
		copy(hooks, a.shutdownHooks)
		a.hooksMu.Unlock()

		// Cancel the context to signal shutdown
		a.cancel()

		// Create a timeout context for shutdown hooks
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), a.timeout)
		defer timeoutCancel()

		// Sort hooks by priority (highest first)
		sortShutdownHooks(hooks)

		// Run shutdown hooks in priority order
		for _, hook := range hooks {
			hookName := hook.Name
			startTime := time.Now()

			a.logger.Info("Running shutdown hook: %s (priority %d)", hookName, hook.Priority)
			if hookErr := hook.Handler(timeoutCtx); hookErr != nil {
				a.logger.Error("Shutdown hook %s error: %v", hookName, hookErr)
			}

			duration := time.Since(startTime)
			a.logger.Info("Shutdown hook %s completed in %v", hookName, duration)

			// Check if context is done after each hook
			if timeoutCtx.Err() != nil {
				a.logger.Warning("Shutdown hooks timed out: %v", timeoutCtx.Err())
				err = fmt.Errorf("shutdown hooks timed out: %w", timeoutCtx.Err())
				return
			}
		}

		// Wait for all goroutines to exit
		waitCh := make(chan struct{})
		go func() {
			a.shutdownWg.Wait()
			close(waitCh)
		}()

		// Wait for either completion or timeout
		select {
		case <-waitCh:
			a.logger.Info("Graceful shutdown completed")
		case <-timeoutCtx.Done():
			a.logger.Warning("Graceful shutdown timed out after %v", a.timeout)
			err = ErrShutdownTimedOut
		}

		// Try to flush logs in any case
		a.logger.Flush()
	})

	if err != nil {
		a.logger.Error("Shutdown error: %v", err)
	}
}

// WaitForShutdown blocks until shutdown is complete
func (a *AppContext) WaitForShutdown() {
	<-a.ctx.Done()
	a.shutdownWg.Wait()
}

// sortShutdownHooks sorts shutdown hooks by priority (highest first)
func sortShutdownHooks(hooks []ShutdownHandler) {
	// Simple insertion sort
	for i := 1; i < len(hooks); i++ {
		key := hooks[i]
		j := i - 1

		for j >= 0 && hooks[j].Priority < key.Priority {
			hooks[j+1] = hooks[j]
			j--
		}

		hooks[j+1] = key
	}
}
