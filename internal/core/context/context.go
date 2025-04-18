package context

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/thushan/inspectre/internal/core/logging"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful shutdown
	DefaultShutdownTimeout = 5 * time.Second
)

// ShutdownHandler is a function that is called during shutdown
type ShutdownHandler func(ctx context.Context) error

// AppContext manages application context and graceful shutdowns
type AppContext struct {
	ctx           context.Context
	cancel        context.CancelFunc
	shutdownWg    sync.WaitGroup
	shutdownHooks []ShutdownHandler
	shutdownOnce  sync.Once
	logger        *logging.Logger
	timeout       time.Duration
	signalCh      chan os.Signal
}

// NewAppContext creates a new application context
func NewAppContext(timeout time.Duration) *AppContext {
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	appCtx := &AppContext{
		ctx:      ctx,
		cancel:   cancel,
		timeout:  timeout,
		signalCh: signalCh,
		logger:   logging.GetLogger(),
	}

	// Start signal handler
	go appCtx.handleSignals()

	return appCtx
}

// handleSignals handles OS signals
func (a *AppContext) handleSignals() {
	sig := <-a.signalCh
	a.logger.Info("Received signal: %s", sig)
	a.Shutdown()
}

// AddShutdownHook adds a function to be called during shutdown
func (a *AppContext) AddShutdownHook(handler ShutdownHandler) {
	a.shutdownHooks = append(a.shutdownHooks, handler)
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
	a.shutdownOnce.Do(func() {
		a.logger.Info("Initiating graceful shutdown...")

		// Cancel the context to signal shutdown
		a.cancel()

		// Create a timeout context for shutdown hooks
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), a.timeout)
		defer timeoutCancel()

		// Run shutdown hooks
		for _, hook := range a.shutdownHooks {
			if err := hook(timeoutCtx); err != nil {
				a.logger.Error("Shutdown hook error: %v", err)
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
		}

		// Flush logs
		a.logger.Flush()
	})
}

// WaitForShutdown blocks until shutdown is complete
func (a *AppContext) WaitForShutdown() {
	<-a.ctx.Done()
	a.shutdownWg.Wait()
}
