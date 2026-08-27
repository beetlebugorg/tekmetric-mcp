// Package retry provides exponential backoff with jitter for retrying failed requests.
// It implements a retry mechanism that gradually increases wait times between attempts
// to handle temporary API failures (rate limits, server errors, etc).
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Retryer implements exponential backoff with jitter for retrying failed operations.
// It retries failed operations with increasing delays between attempts.
//
// The backoff formula is: min(((2^n) + random_milliseconds), max_backoff)
// where n is the attempt number, starting from 0.
//
// Example backoff sequence (without jitter):
//   - Attempt 1: 2 seconds
//   - Attempt 2: 4 seconds
//   - Attempt 3: 8 seconds
//   - Attempt 4: 16 seconds
//   - etc., up to maxBackoff
type Retryer struct {
	maxRetries int // Maximum number of retry attempts
	maxBackoff int // Maximum backoff duration in seconds
}

// New creates a new Retryer with the specified retry and backoff limits.
//
// Parameters:
//   - maxRetries: Maximum number of times to retry a failed operation
//   - maxBackoffSec: Maximum wait time between retries in seconds
//
// Returns:
//   - *Retryer: Configured retryer instance
func New(maxRetries, maxBackoffSec int) *Retryer {
	return &Retryer{
		maxRetries: maxRetries,
		maxBackoff: maxBackoffSec,
	}
}

// Temporary marks an error that another attempt may resolve.
// An error that does not implement this interface is permanent.
type Temporary interface {
	error
	Temporary() bool
}

// IsTemporary reports whether an error, or any error it wraps, is temporary.
func IsTemporary(err error) bool {
	var temp Temporary
	if errors.As(err, &temp) {
		return temp.Temporary()
	}
	return false
}

// Do executes a function with exponential backoff retry logic.
// It retries only a temporary error, and it waits longer before each attempt.
//
// The function stops retrying when:
//   - fn returns nil
//   - fn returns an error that is not temporary
//   - the attempts reach maxRetries
//   - ctx is cancelled
//
// Parameters:
//   - ctx: Context that cancels both the attempts and the waits
//   - fn: Function to execute and retry on failure
//
// Returns:
//   - error: The last error fn returned, the context error, or nil on success
func (rl *Retryer) Do(ctx context.Context, fn func() error) error {
	var err error

	// Try the operation up to maxRetries + 1 times (initial attempt + retries)
	for attempt := 0; attempt <= rl.maxRetries; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if err != nil {
				return err
			}
			return ctxErr
		}

		err = fn()
		if err == nil {
			return nil
		}

		if !IsTemporary(err) {
			return err
		}

		// The last attempt does not wait.
		if attempt == rl.maxRetries {
			break
		}

		if waitErr := wait(ctx, rl.calculateBackoff(attempt)); waitErr != nil {
			return err
		}
	}

	return err
}

// wait sleeps for a duration, or returns early when the context is cancelled.
func wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// calculateBackoff calculates the backoff duration using exponential backoff with jitter.
// Jitter (randomness) is added to prevent multiple clients from retrying simultaneously,
// which could cause a "thundering herd" problem.
//
// Formula: min(((2^(n+1)) + random_milliseconds), max_backoff)
//
// Parameters:
//   - attempt: Current attempt number (0-indexed)
//
// Returns:
//   - time.Duration: Time to wait before next retry
func (rl *Retryer) calculateBackoff(attempt int) time.Duration {
	// Calculate exponential component: 2^(n+1) seconds
	exponential := math.Pow(2, float64(attempt+1))

	// Add random jitter between 0 and 1000 milliseconds
	// This prevents synchronized retries across multiple clients
	jitterMs := rand.Intn(1001)
	jitter := float64(jitterMs) / 1000.0

	// Total backoff in seconds
	backoffSec := exponential + jitter

	// Cap at maximum backoff to prevent excessive wait times
	if backoffSec > float64(rl.maxBackoff) {
		backoffSec = float64(rl.maxBackoff)
	}

	return time.Duration(backoffSec * float64(time.Second))
}
