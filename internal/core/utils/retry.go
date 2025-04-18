package utils

import (
	"context"
	"errors"
	"time"
)

const (
	// DefaultRetryWait is the default wait time between retries
	DefaultRetryWait = 100 * time.Millisecond

	// DefaultMaxRetries is the default maximum number of retries
	DefaultMaxRetries = 3

	// DefaultBackoffFactor is the default backoff multiplier
	DefaultBackoffFactor = 2.0
)

var (
	// ErrMaxRetriesExceeded is returned when the maximum number of retries is exceeded
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")

	// ErrContextCancelled is returned when the context is cancelled during retry
	ErrContextCancelled = errors.New("context cancelled during retry")
)

// RetryOptions configures the retry behavior
type RetryOptions struct {
	// InitialWait is the wait time before the first retry
	InitialWait time.Duration

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// BackoffFactor is the multiplier for wait time after each retry
	BackoffFactor float64

	// RetryableErrors is a list of errors that should trigger a retry
	RetryableErrors []error
}

// DefaultRetryOptions returns the default retry options
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		InitialWait:     DefaultRetryWait,
		MaxRetries:      DefaultMaxRetries,
		BackoffFactor:   DefaultBackoffFactor,
		RetryableErrors: nil, // Retry on all errors
	}
}

// RetryWithContext retries a function with the given context and options
func RetryWithContext(ctx context.Context, maxRetries int, fn func() error) error {
	options := DefaultRetryOptions()
	options.MaxRetries = maxRetries

	return RetryWithOptionsAndContext(ctx, options, fn)
}

// RetryWithOptionsAndContext retries a function with the given context and options
func RetryWithOptionsAndContext(ctx context.Context, options RetryOptions, fn func() error) error {
	var lastErr error

	wait := options.InitialWait
	if wait <= 0 {
		wait = DefaultRetryWait
	}

	maxRetries := options.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}

	backoffFactor := options.BackoffFactor
	if backoffFactor <= 0 {
		backoffFactor = DefaultBackoffFactor
	}

	for i := 0; i <= maxRetries; i++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return errors.Join(ErrContextCancelled, lastErr)
			}
			return ErrContextCancelled
		default:
			// Context not cancelled, proceed
		}

		// Try the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if we should retry this error
		if !isRetryableError(err, options.RetryableErrors) {
			return err
		}

		// Return immediately on last attempt
		if i == maxRetries {
			break
		}

		// Wait before next attempt
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ErrContextCancelled, lastErr)
		case <-timer.C:
			// Timer expired, continue with next attempt
		}

		// Increase wait time for next attempt
		wait = time.Duration(float64(wait) * backoffFactor)
	}

	return errors.Join(ErrMaxRetriesExceeded, lastErr)
}

// isRetryableError checks if an error should be retried
func isRetryableError(err error, retryableErrors []error) bool {
	// If no specific errors are provided, retry all errors
	if len(retryableErrors) == 0 {
		return true
	}

	// Check if the error is in the retryable errors list
	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}

// Retry retries a function with default options
func Retry(maxRetries int, fn func() error) error {
	return RetryWithContext(context.Background(), maxRetries, fn)
}

// RetryWithOptions retries a function with the given options
func RetryWithOptions(options RetryOptions, fn func() error) error {
	return RetryWithOptionsAndContext(context.Background(), options, fn)
}
